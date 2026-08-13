package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// captureStdout runs fn with stdout redirected, so a test can assert on what
// the report actually said rather than on the values behind it. The reports are
// a few hundred bytes, well under the pipe buffer.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	w.Close()
	os.Stdout = old
	body, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(body), runErr
}

// statusProject lays out a project with one source, one declared target, and a
// built output that postdates it: a set with nothing outstanding.
func statusProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	mustWrite(t, manifestName, `[project]
name = "demo"

[[target]]
input = "report.md"
output = "Report.docx"
`)
	mustWrite(t, filepath.Join(srcDir, "report.md"), "# Report\n")
	mustWrite(t, filepath.Join(distDir, "Report.docx"), "built")
	touch(t, filepath.Join(srcDir, "report.md"), time.Now().Add(-time.Hour))
	touch(t, filepath.Join(distDir, "Report.docx"), time.Now())
	// The build that produced that output would have recorded it.
	fps := fingerprints{}
	fps.record(Target{Input: "report.md", Output: "Report.docx"})
	if err := fps.save(); err != nil {
		t.Fatal(err)
	}
	// A PATH holding the converter this project needs, so the converters line
	// does not depend on what happens to be installed on the machine running
	// the tests.
	bin := filepath.Join(dir, "bin")
	mustWrite(t, filepath.Join(bin, "md2docx"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(bin, "md2docx"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return dir
}

func mustStatus(t *testing.T) string {
	t.Helper()
	out, err := captureStdout(t, func() error { return cmdStatus(nil) })
	if err != nil {
		t.Fatalf("status returned an error: %v", err)
	}
	return out
}

// The report is one screen, and every line of it has to be there: a status
// missing a line silently answers less than it was asked.
func TestStatusReportsEveryLine(t *testing.T) {
	statusProject(t)
	out := mustStatus(t)
	for _, label := range []string{"project", "manifest", "build", "sources", "dist", "publish", "converters"} {
		if !strings.Contains(out, label) {
			t.Errorf("report has no %q line:\n%s", label, out)
		}
	}
	for _, want := range []string{"demo", "1 target", "1 target up to date", "md2docx: all present"} {
		if !strings.Contains(out, want) {
			t.Errorf("report does not say %q:\n%s", want, out)
		}
	}
}

// The whole promise of the command: it describes the project without touching
// it. Nothing it reads may be created as a side effect of reading it.
func TestStatusWritesNothing(t *testing.T) {
	dir := statusProject(t)
	before := treeSnapshot(t, dir)
	mustStatus(t)
	if after := treeSnapshot(t, dir); after != before {
		t.Errorf("status changed the project.\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// A build's worth of work outstanding is the number status exists to show.
func TestStatusCountsOutOfDateTargets(t *testing.T) {
	statusProject(t)
	touch(t, filepath.Join(srcDir, "report.md"), time.Now().Add(time.Minute))
	if out := mustStatus(t); !strings.Contains(out, "1 target out of date") {
		t.Errorf("an edited source should read as out of date:\n%s", out)
	}
	// And the line points at the command that resolves it.
	if out := mustStatus(t); !strings.Contains(out, "ditto build") {
		t.Errorf("the build line should name the command to run:\n%s", out)
	}
}

// Both directions between src/ and the manifest: a file nobody declared is
// scan's question, a declared input that is not there is check's, and status
// counts each without diagnosing either.
func TestStatusReportsSourcesBothWays(t *testing.T) {
	statusProject(t)
	mustWrite(t, filepath.Join(srcDir, "appendix.md"), "# Appendix\n")
	out := mustStatus(t)
	if !strings.Contains(out, "1 file in src/ that no target covers") {
		t.Errorf("an undeclared file should be counted:\n%s", out)
	}
	if !strings.Contains(out, "ditto scan") {
		t.Errorf("the sources line should name scan:\n%s", out)
	}

	if err := os.Remove(filepath.Join(srcDir, "report.md")); err != nil {
		t.Fatal(err)
	}
	out = mustStatus(t)
	if !strings.Contains(out, "1 input declared but not found") {
		t.Errorf("a declared input that is gone should be counted:\n%s", out)
	}
	if !strings.Contains(out, "ditto check") {
		t.Errorf("a missing input should be left to check to diagnose:\n%s", out)
	}
}

// A pipeline produces its target's inputs, so their absence is expected rather
// than a fault — the same rule check follows.
func TestStatusDoesNotCountPipelineInputsAsMissing(t *testing.T) {
	statusProject(t)
	mustWrite(t, manifestName, `[project]
name = "demo"

[[target]]
input = "generated.csv"
output = "Data.xlsx"
pipeline = ["./extract.sh"]
`)
	if out := mustStatus(t); strings.Contains(out, "declared but not found") {
		t.Errorf("a pipeline's inputs are produced, not missing:\n%s", out)
	}
}

// A renamed target leaves its old output behind, and publish without --delete
// would ship it beside the real deliverable.
func TestStatusCountsOrphansInDist(t *testing.T) {
	statusProject(t)
	mustWrite(t, filepath.Join(distDir, "Old Name.docx"), "stale")
	out := mustStatus(t)
	if !strings.Contains(out, "2 files, 1 produced by no target") {
		t.Errorf("the stale output should be counted:\n%s", out)
	}
	if !strings.Contains(out, "ditto clean") {
		t.Errorf("the dist line should name clean:\n%s", out)
	}
}

func TestStatusUnbuiltDist(t *testing.T) {
	statusProject(t)
	if err := os.RemoveAll(distDir); err != nil {
		t.Fatal(err)
	}
	if out := mustStatus(t); !strings.Contains(out, "not built yet") {
		t.Errorf("an absent dist/ should say so:\n%s", out)
	}
}

// The publish line names the destination and says nothing about what is
// already there, because finding that out costs a network call.
func TestStatusPublishLine(t *testing.T) {
	statusProject(t)
	if out := mustStatus(t); !strings.Contains(out, "none; dist/ is itself the deliverable") {
		t.Errorf("a project with no publish root should say so:\n%s", out)
	}

	mustWrite(t, manifestName, `[project]
name = "demo"

[publish]
root = "https://example.sharepoint.com/Shared Documents/Phase 2"

[[target]]
input = "report.md"
output = "Report.docx"
`)
	if out := mustStatus(t); !strings.Contains(out, "https://example.sharepoint.com/Shared Documents/Phase 2") {
		t.Errorf("the publish root should be named:\n%s", out)
	}
}

// "Can this machine build this project" is the question that separates a wrong
// project from a new laptop, and the answer has to carry where the tool comes
// from: nothing about the name md2docx says office-convert.
func TestStatusReportsMissingConverterWithHint(t *testing.T) {
	statusProject(t)
	t.Setenv("PATH", t.TempDir()) // an empty PATH: nothing is installed
	out := mustStatus(t)
	if !strings.Contains(out, "md2docx: not on PATH") {
		t.Errorf("a missing converter should be named:\n%s", out)
	}
	if !strings.Contains(out, "sudo apt install office-convert") {
		t.Errorf("a missing built-in should say where it comes from:\n%s", out)
	}
}

// A custom converter is shown as the manifest spells it, not as the absolute
// path resolveScript turns it into.
func TestStatusNamesCustomConverterAsWritten(t *testing.T) {
	dir := statusProject(t)
	mustWrite(t, manifestName, `[project]
name = "demo"

[[target]]
input = "report.md"
output = "Report.docx"
converter = "./tools/render.sh"
`)
	mustWrite(t, "tools/render.sh", "#!/bin/sh\n")
	if err := os.Chmod("tools/render.sh", 0o755); err != nil {
		t.Fatal(err)
	}
	out := mustStatus(t)
	if !strings.Contains(out, "./tools/render.sh: all present") {
		t.Errorf("a custom converter should be named as written:\n%s", out)
	}
	// The project line legitimately carries the root; the converters line must
	// not, which is where resolveScript's absolute path would show up.
	if strings.Contains(lineWithLabel(out, "converters"), dir) {
		t.Errorf("the resolved absolute path leaked into the converters line:\n%s", out)
	}
}

// Outside a project, "there is no project here" is the true answer to the
// question status was asked, so it is reported rather than raised.
func TestStatusOutsideAProjectSucceeds(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := captureStdout(t, func() error { return cmdStatus(nil) })
	if err != nil {
		t.Fatalf("status outside a project should not fail: %v", err)
	}
	if !strings.Contains(out, "No ditto project here") {
		t.Errorf("status should say there is no project:\n%s", out)
	}
	for _, verb := range []string{"ditto new", "ditto init"} {
		if !strings.Contains(out, verb) {
			t.Errorf("status should point at %q:\n%s", verb, out)
		}
	}
}

// A project written against the pre-v0.2.0 manifest name is broken, not
// absent, and its error carries the rename that fixes it.
func TestStatusRaisesLegacyManifest(t *testing.T) {
	t.Chdir(t.TempDir())
	mustWrite(t, legacyManifestName, "[project]\nname = \"old\"\n")
	_, err := captureStdout(t, func() error { return cmdStatus(nil) })
	if err == nil {
		t.Fatal("a project on the old manifest name should be raised, not reported as absent")
	}
	if !strings.Contains(err.Error(), manifestName) {
		t.Errorf("the error should name the rename: %v", err)
	}
}

// A manifest that will not parse leaves nothing to report; that is a fault in
// the project rather than a state of it.
func TestStatusRaisesUnparsableManifest(t *testing.T) {
	t.Chdir(t.TempDir())
	mustWrite(t, manifestName, "[project\nname = \n")
	if _, err := captureStdout(t, func() error { return cmdStatus(nil) }); err == nil {
		t.Error("an unparsable manifest should be an error, not a status line")
	}
}

// A dist that resolves inside src/ is refused before anything is built, so
// status cannot say what is out of date. It says that rather than guessing, and
// carries the reason on the dist line alone rather than twice on one screen.
func TestStatusMisconfiguredDist(t *testing.T) {
	statusProject(t)
	mustWrite(t, manifestName, `[project]
name = "demo"
dist = "src/out"

[[target]]
input = "report.md"
output = "Report.docx"
`)
	out := mustStatus(t)
	if !strings.Contains(lineWithLabel(out, "dist"), "misconfigured") {
		t.Errorf("the dist line should carry the fault:\n%s", out)
	}
	if !strings.Contains(lineWithLabel(out, "build"), "cannot tell") {
		t.Errorf("the build line should decline to guess:\n%s", out)
	}
	if strings.Contains(lineWithLabel(out, "build"), "resolves inside") {
		t.Errorf("the reason belongs on one line, not two:\n%s", out)
	}
}

func TestStatusTakesNoArguments(t *testing.T) {
	statusProject(t)
	if err := cmdStatus([]string{"D3"}); err == nil {
		t.Error("status takes no filter argument; an unexpected one should be rejected")
	}
}

func TestCountPluralises(t *testing.T) {
	cases := []struct {
		n    int
		noun string
		want string
	}{
		{0, "target", "0 targets"},
		{1, "target", "1 target"},
		{2, "file", "2 files"},
	}
	for _, c := range cases {
		if got := count(c.n, c.noun); got != c.want {
			t.Errorf("count(%d, %q) = %q, want %q", c.n, c.noun, got, c.want)
		}
	}
}

func TestShortenHomeIsInverseOfExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	for _, p := range []string{"~", "~/clients/acme"} {
		if got := shortenHome(expandHome(p)); got != p {
			t.Errorf("shortenHome(expandHome(%q)) = %q, want %q", p, got, p)
		}
	}
	if got := shortenHome("/etc/ditto"); got != "/etc/ditto" {
		t.Errorf("a path outside %s should be untouched, got %q", home, got)
	}
}

// treeSnapshot renders every path under dir with its size, so a test can assert
// that a command left the project exactly as it found it.
func treeSnapshot(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		b.WriteString(rel)
		if !info.IsDir() {
			b.WriteString(" " + strconv.FormatInt(info.Size(), 10))
		}
		b.WriteString("\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// lineWithLabel returns the report line starting with the given label, so an
// assertion can be aimed at one line rather than at the whole screen.
func lineWithLabel(report, label string) string {
	for _, line := range strings.Split(report, "\n") {
		if strings.HasPrefix(line, label) {
			return line
		}
	}
	return ""
}
