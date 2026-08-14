package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// workspaceProject lays out a root with three members, each a real project
// whose converter records that it ran. The recorder is what lets these tests
// ask what the fan-out actually did rather than what it printed.
func workspaceProject(t *testing.T, members ...string) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)

	mustWrite(t, "convert.sh", "#!/bin/sh\nprintf '%s\\n' \"$2\" >> \"$DITTO_LOG\"\ncp \"$1\" \"$2\"\n")
	if err := os.Chmod("convert.sh", 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DITTO_LOG", filepath.Join(root, "built.log"))

	list := make([]string, len(members))
	for i, m := range members {
		list[i] = `"` + m + `"`
		mustWrite(t, filepath.Join(m, srcDir, "report.md"), "# "+m+"\n")
		mustWrite(t, filepath.Join(m, manifestName),
			"[project]\nname = \""+m+"\"\n\n[[target]]\ninput = \"report.md\"\noutput = \""+m+".txt\"\nconverter = \"../convert.sh\"\n")
	}
	mustWrite(t, manifestName, "[workspace]\nmembers = ["+strings.Join(list, ", ")+"]\n")
	return root
}

// builtOutputs returns the outputs the converter was called for, in order.
func builtOutputs(t *testing.T, root string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "built.log"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Fields(string(body))
}

func TestWorkspaceBuildsEveryMember(t *testing.T) {
	root := workspaceProject(t, "phase-1", "phase-2", "closeout")
	if err := workspaceAware("build", nil, cmdBuild); err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := builtOutputs(t, root); len(got) != 3 {
		t.Errorf("every member should build; converted %v", got)
	}
}

func TestWorkspaceBuildsOneNamedMember(t *testing.T) {
	root := workspaceProject(t, "phase-1", "phase-2", "closeout")
	if err := workspaceAware("build", []string{"phase-2"}, cmdBuild); err != nil {
		t.Fatalf("build: %v", err)
	}
	got := builtOutputs(t, root)
	if len(got) != 1 || !strings.HasSuffix(got[0], "phase-2.txt") {
		t.Errorf("naming a member should build only that member; converted %v", got)
	}
}

// The selector has to be the first argument, so a flag's value is never
// mistaken for a member name. Without that rule `--profile draft` at a
// workspace root would have to guess what "draft" is.
func TestWorkspaceFlagValueIsNotAMemberSelector(t *testing.T) {
	root := workspaceProject(t, "phase-1", "phase-2")
	if err := workspaceAware("build", []string{"--force"}, cmdBuild); err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := builtOutputs(t, root); len(got) != 2 {
		t.Errorf("a leading flag selects no member, so every member builds; converted %v", got)
	}
}

func TestWorkspaceUnknownMemberListsWhatExists(t *testing.T) {
	workspaceProject(t, "phase-1", "phase-2")
	err := workspaceAware("build", []string{"phase-9"}, cmdBuild)
	if err == nil {
		t.Fatal("an unknown member must fail rather than build everything")
	}
	if !strings.Contains(err.Error(), "phase-1, phase-2") {
		t.Errorf("the error should list the members; got %v", err)
	}
}

// A broken phase-1 must not hide phase-2 and closeout — reporting one failure
// and stopping is the opposite of what running a verb across an engagement is
// for.
func TestWorkspaceKeepsGoingAfterAMemberFails(t *testing.T) {
	root := workspaceProject(t, "phase-1", "phase-2", "closeout")
	if err := os.Remove(filepath.Join("phase-2", srcDir, "report.md")); err != nil {
		t.Fatal(err)
	}
	err := workspaceAware("build", nil, cmdBuild)
	if err == nil {
		t.Fatal("a failing member must fail the run")
	}
	if !strings.Contains(err.Error(), "1 of 3") {
		t.Errorf("the error should count members; got %v", err)
	}
	got := builtOutputs(t, root)
	if len(got) != 2 {
		t.Errorf("the members either side of the failure should still build; converted %v", got)
	}
}

func TestWorkspaceMemberWithoutAManifestIsReported(t *testing.T) {
	workspaceProject(t, "phase-1", "phase-2")
	if err := os.Remove(filepath.Join("phase-2", manifestName)); err != nil {
		t.Fatal(err)
	}
	err := workspaceAware("build", nil, cmdBuild)
	if err == nil {
		t.Fatal("a member with no manifest must fail the run")
	}
}

func TestWorkspaceRejectsTargetsAtTheRoot(t *testing.T) {
	workspaceProject(t, "phase-1")
	mustWrite(t, manifestName, "[workspace]\nmembers = [\"phase-1\"]\n\n[[target]]\ninput = \"a.md\"\noutput = \"A.txt\"\n")
	err := workspaceAware("build", nil, cmdBuild)
	if err == nil {
		t.Fatal("a root cannot be both a workspace and a project")
	}
	if !strings.Contains(err.Error(), "move those targets into a member") {
		t.Errorf("got %v", err)
	}
}

func TestWorkspaceRejectsAnEmptyMemberList(t *testing.T) {
	workspaceProject(t, "phase-1")
	mustWrite(t, manifestName, "[workspace]\nmembers = []\n")
	if err := workspaceAware("build", nil, cmdBuild); err == nil {
		t.Fatal("a workspace with no members must say so")
	}
}

func TestWorkspaceRejectsADuplicateMember(t *testing.T) {
	workspaceProject(t, "phase-1")
	mustWrite(t, manifestName, "[workspace]\nmembers = [\"phase-1\", \"phase-1\"]\n")
	err := workspaceAware("build", nil, cmdBuild)
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("a member listed twice should be rejected; got %v", err)
	}
}

// Standing inside a member operates on that member alone, because
// findProjectRoot reaches its manifest before the workspace root. That is also
// how prefix filtering stays available without competing with the member
// selector for the same argument position.
func TestInsideAMemberTheWorkspaceIsInvisible(t *testing.T) {
	root := workspaceProject(t, "phase-1", "phase-2")
	t.Chdir(filepath.Join(root, "phase-2"))
	if err := workspaceAware("build", nil, cmdBuild); err != nil {
		t.Fatalf("build: %v", err)
	}
	got := builtOutputs(t, root)
	if len(got) != 1 || !strings.HasSuffix(got[0], "phase-2.txt") {
		t.Errorf("a member build should not reach its siblings; converted %v", got)
	}
}

// An ordinary project must be untouched by any of this.
func TestPlainProjectIsUnaffected(t *testing.T) {
	countingProject(t, "[project]\nname = \"demo\"\n\n[[target]]\ninput = \"a.md\"\noutput = \"A.txt\"\nconverter = \"convert.sh\"\n")
	if err := workspaceAware("build", nil, cmdBuild); err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := conversions(t); len(got) != 1 {
		t.Errorf("a plain project should build exactly as before; converted %v", got)
	}
}
