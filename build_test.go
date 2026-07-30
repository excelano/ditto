package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// freshProject lays out a project where the output is newer than every input
// and its manifest entry is the one on record, which is the state a build has
// just left behind.
func freshProject(t *testing.T) (target Target, fps fingerprints) {
	t.Helper()
	t.Chdir(t.TempDir())

	mustWrite(t, manifestName, "[project]\nname = \"demo\"\n")
	mustWrite(t, filepath.Join(srcDir, "report.md"), "# Report\n")
	mustWrite(t, filepath.Join(distDir, "Report.docx"), "built")

	old := time.Now().Add(-time.Hour)
	touch(t, manifestName, old)
	touch(t, filepath.Join(srcDir, "report.md"), old)
	touch(t, filepath.Join(distDir, "Report.docx"), time.Now())

	target = Target{Input: "report.md", Output: "Report.docx"}
	fps = fingerprints{}
	fps.record(target)
	return target, fps
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func touch(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func TestIsFreshWhenOutputIsNewest(t *testing.T) {
	target, fps := freshProject(t)
	if !isFresh(target, distDir, fps) {
		t.Error("a target whose output postdates its input should be skipped")
	}
}

func TestIsFreshStaleInput(t *testing.T) {
	target, fps := freshProject(t)
	touch(t, filepath.Join(srcDir, "report.md"), time.Now().Add(time.Minute))
	if isFresh(target, distDir, fps) {
		t.Error("editing the source must rebuild")
	}
}

// Editing a target's view, reference, or converter changes the result without
// touching any source file, so the entry itself is compared.
func TestIsFreshStaleTargetEntry(t *testing.T) {
	target, fps := freshProject(t)
	target.View = "slides"
	if isFresh(target, distDir, fps) {
		t.Error("editing the target's manifest entry must rebuild")
	}
}

// The reason the entry is fingerprinted rather than the manifest's mtime taken
// as the answer: an edit elsewhere in a manifest of fifty targets must not
// reconvert the forty-nine it did not touch.
func TestIsFreshSurvivesUnrelatedManifestEdits(t *testing.T) {
	target, fps := freshProject(t)
	touch(t, manifestName, time.Now().Add(time.Minute))
	if !isFresh(target, distDir, fps) {
		t.Error("touching the manifest without changing this target must leave it up to date")
	}
}

// State that was never recorded, or that has been thrown away, may only cost a
// rebuild — it must never be read as confirmation that the output is current.
func TestIsFreshWithoutRecordedState(t *testing.T) {
	target, _ := freshProject(t)
	if isFresh(target, distDir, fingerprints{}) {
		t.Error("an unrecorded target must rebuild rather than be assumed fresh")
	}
}

func TestIsFreshMissingOutput(t *testing.T) {
	target, fps := freshProject(t)
	if err := os.Remove(filepath.Join(distDir, "Report.docx")); err != nil {
		t.Fatal(err)
	}
	if isFresh(target, distDir, fps) {
		t.Error("a missing output must rebuild")
	}
}

func TestIsFreshStaleReference(t *testing.T) {
	target, _ := freshProject(t)
	mustWrite(t, "brand/house.docx", "template")
	touch(t, "brand/house.docx", time.Now().Add(time.Minute))
	target.Reference = "brand/house.docx"
	fps := fingerprints{}
	fps.record(target)
	if isFresh(target, distDir, fps) {
		t.Error("a restyled reference must rebuild the documents that use it")
	}
}

// A pipeline refreshes its inputs from somewhere ditto cannot see, so mtimes
// cannot prove the output is current.
func TestIsFreshNeverForPipelineTargets(t *testing.T) {
	target, _ := freshProject(t)
	target.Pipeline = []string{"extract.sh"}
	fps := fingerprints{}
	fps.record(target)
	if isFresh(target, distDir, fps) {
		t.Error("a target with a pipeline must always rebuild")
	}
}

// A missing input is left for buildTarget to report, so it must not be
// mistaken for a fresh target and skipped silently.
func TestIsFreshMissingInput(t *testing.T) {
	target, fps := freshProject(t)
	if err := os.Remove(filepath.Join(srcDir, "report.md")); err != nil {
		t.Fatal(err)
	}
	if isFresh(target, distDir, fps) {
		t.Error("a missing input must fall through to the build for a real error")
	}
}

// A dry run must reach the same rebuild decision as a real build while running
// neither the converter nor the pipeline. This project's converter does not
// exist and its pipeline writes a file, so either one running is visible.
func staleProjectWithSideEffects(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())

	mustWrite(t, manifestName, `[project]
name = "demo"

[[target]]
input = "report.md"
output = "Report.docx"
converter = "./no-such-converter"
pipeline = ["./touch-evidence.sh"]
`)
	mustWrite(t, filepath.Join(srcDir, "report.md"), "# Report\n")
	mustWrite(t, "touch-evidence.sh", "#!/bin/sh\ntouch pipeline-ran\n")
	if err := os.Chmod("touch-evidence.sh", 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDryRunConvertsNothing(t *testing.T) {
	staleProjectWithSideEffects(t)

	if err := cmdBuild([]string{"--dry-run"}); err != nil {
		t.Fatalf("a dry run must not fail on a converter it never calls: %v", err)
	}
	if _, err := os.Stat(filepath.Join(distDir, "Report.docx")); err == nil {
		t.Error("a dry run wrote an output")
	}
	if _, err := os.Stat("pipeline-ran"); err == nil {
		t.Error("a dry run ran the pipeline, which writes into the project")
	}
}

func TestBuildDryRunShortFlag(t *testing.T) {
	staleProjectWithSideEffects(t)
	if err := cmdBuild([]string{"-n"}); err != nil {
		t.Fatalf("-n must mean the same as --dry-run: %v", err)
	}
}

// Without the flag the same project fails, which is what proves the dry run
// above was skipping the work rather than the project having none to do.
func TestBuildWithoutDryRunRunsTheConverter(t *testing.T) {
	staleProjectWithSideEffects(t)
	if err := cmdBuild(nil); err == nil {
		t.Error("a real build must call the missing converter and fail")
	}
	if _, err := os.Stat("pipeline-ran"); err != nil {
		t.Error("a real build must run the pipeline")
	}
}

// twoTargets is a project of two independent deliverables, the smallest shape
// in which "rebuild only what changed" means anything.
const twoTargets = `[project]
name = "demo"

[[target]]
input = "a.md"
output = "A.txt"
converter = "convert.sh"
%s
[[target]]
input = "b.md"
output = "B.txt"
converter = "convert.sh"
`

// countingProject gives the project a converter that records every invocation,
// so the tests below can ask what the build actually ran rather than what it
// reported.
func countingProject(t *testing.T, manifest string) {
	t.Helper()
	t.Chdir(t.TempDir())
	mustWrite(t, manifestName, manifest)
	mustWrite(t, filepath.Join(srcDir, "a.md"), "# A\n")
	mustWrite(t, filepath.Join(srcDir, "b.md"), "# B\n")
	mustWrite(t, "convert.sh", "#!/bin/sh\nprintf '%s\\n' \"$2\" >> converted.log\ncp \"$1\" \"$2\"\n")
	if err := os.Chmod("convert.sh", 0o755); err != nil {
		t.Fatal(err)
	}
}

// conversions returns the outputs the converter has been called for, in order.
func conversions(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile("converted.log")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Fields(string(body))
}

func TestBuildSkipsWhatItAlreadyBuilt(t *testing.T) {
	countingProject(t, fmt.Sprintf(twoTargets, ""))

	if err := cmdBuild(nil); err != nil {
		t.Fatal(err)
	}
	if got := conversions(t); len(got) != 2 {
		t.Fatalf("first build converted %v, want both targets", got)
	}
	if err := cmdBuild(nil); err != nil {
		t.Fatal(err)
	}
	if got := conversions(t); len(got) != 2 {
		t.Errorf("second build converted %v; nothing changed, so it should have converted nothing", got)
	}
}

// The whole point of the change: editing one target's entry must reconvert that
// target and leave every other one alone, where before it reconverted the lot.
func TestBuildAfterManifestEditRebuildsOnlyTheEditedTarget(t *testing.T) {
	countingProject(t, fmt.Sprintf(twoTargets, ""))
	if err := cmdBuild(nil); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, manifestName, fmt.Sprintf(twoTargets, "view = \"slides\"\n"))
	if err := cmdBuild(nil); err != nil {
		t.Fatal(err)
	}

	got := conversions(t)
	if len(got) != 3 {
		t.Fatalf("converted %v, want one rebuild on top of the first two", got)
	}
	if want := filepath.Join(distDir, "A.txt"); got[2] != want {
		t.Errorf("rebuilt %s, want the edited target %s", got[2], want)
	}
}

// --force is the escape hatch for everything freshness cannot see, so it must
// ignore the recorded state as well as the timestamps.
func TestBuildForceIgnoresRecordedState(t *testing.T) {
	countingProject(t, fmt.Sprintf(twoTargets, ""))
	if err := cmdBuild(nil); err != nil {
		t.Fatal(err)
	}
	if err := cmdBuild([]string{"--force"}); err != nil {
		t.Fatal(err)
	}
	if got := conversions(t); len(got) != 4 {
		t.Errorf("converted %v, want both targets converted twice", got)
	}
}

// A filtered build visits only part of the manifest, so it must merge into the
// recorded state rather than replace it — otherwise `ditto build D3` would
// forget every other deliverable and the next full build would redo them all.
func TestFilteredBuildKeepsTheOtherTargetsState(t *testing.T) {
	countingProject(t, fmt.Sprintf(twoTargets, ""))
	if err := cmdBuild(nil); err != nil {
		t.Fatal(err)
	}
	if err := cmdBuild([]string{"A.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdBuild(nil); err != nil {
		t.Fatal(err)
	}
	if got := conversions(t); len(got) != 2 {
		t.Errorf("converted %v; a filtered build must not discard what it did not look at", got)
	}
}

// A dry run reports a decision without acting on it, so it must not leave state
// claiming the targets were built.
func TestDryRunRecordsNothing(t *testing.T) {
	countingProject(t, fmt.Sprintf(twoTargets, ""))
	if err := cmdBuild([]string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fingerprintPath()); err == nil {
		t.Error("a dry run recorded build state for outputs it never wrote")
	}
	if err := cmdBuild(nil); err != nil {
		t.Fatal(err)
	}
	if got := conversions(t); len(got) != 2 {
		t.Errorf("converted %v; the real build after a dry run must still do the work", got)
	}
}

// A target that failed has no output worth remembering: the next build must
// retry it rather than treat the failure as done.
func TestFailedTargetIsNotRecorded(t *testing.T) {
	countingProject(t, fmt.Sprintf(twoTargets, ""))
	mustWrite(t, "convert.sh", "#!/bin/sh\nprintf '%s\\n' \"$2\" >> converted.log\nexit 1\n")
	if err := os.Chmod("convert.sh", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cmdBuild(nil); err == nil {
		t.Fatal("a converter that exits non-zero must fail the build")
	}
	if err := cmdBuild(nil); err == nil {
		t.Fatal("the retry must fail too")
	}
	if got := conversions(t); len(got) != 4 {
		t.Errorf("converted %v, want both targets attempted on both builds", got)
	}
}
