package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// freshProject lays out a project where the output is newer than every input,
// which is the state a build has just left behind.
func freshProject(t *testing.T) (dir string, target Target) {
	t.Helper()
	dir = t.TempDir()
	t.Chdir(dir)

	mustWrite(t, manifestName, "[project]\nname = \"demo\"\n")
	mustWrite(t, filepath.Join(srcDir, "report.md"), "# Report\n")
	mustWrite(t, filepath.Join(distDir, "Report.docx"), "built")

	old := time.Now().Add(-time.Hour)
	touch(t, manifestName, old)
	touch(t, filepath.Join(srcDir, "report.md"), old)
	touch(t, filepath.Join(distDir, "Report.docx"), time.Now())

	return dir, Target{Input: "report.md", Output: "Report.docx"}
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
	_, target := freshProject(t)
	if !isFresh(target, distDir) {
		t.Error("a target whose output postdates its input should be skipped")
	}
}

func TestIsFreshStaleInput(t *testing.T) {
	_, target := freshProject(t)
	touch(t, filepath.Join(srcDir, "report.md"), time.Now().Add(time.Minute))
	if isFresh(target, distDir) {
		t.Error("editing the source must rebuild")
	}
}

// Editing a target's view, reference, or converter changes the result without
// touching any source file, so the manifest counts as an input.
func TestIsFreshStaleManifest(t *testing.T) {
	_, target := freshProject(t)
	touch(t, manifestName, time.Now().Add(time.Minute))
	if isFresh(target, distDir) {
		t.Error("editing the manifest must rebuild")
	}
}

func TestIsFreshMissingOutput(t *testing.T) {
	_, target := freshProject(t)
	if err := os.Remove(filepath.Join(distDir, "Report.docx")); err != nil {
		t.Fatal(err)
	}
	if isFresh(target, distDir) {
		t.Error("a missing output must rebuild")
	}
}

func TestIsFreshStaleReference(t *testing.T) {
	_, target := freshProject(t)
	mustWrite(t, "brand/house.docx", "template")
	touch(t, "brand/house.docx", time.Now().Add(time.Minute))
	target.Reference = "brand/house.docx"
	if isFresh(target, distDir) {
		t.Error("a restyled reference must rebuild the documents that use it")
	}
}

// A pipeline refreshes its inputs from somewhere ditto cannot see, so mtimes
// cannot prove the output is current.
func TestIsFreshNeverForPipelineTargets(t *testing.T) {
	_, target := freshProject(t)
	target.Pipeline = []string{"extract.sh"}
	if isFresh(target, distDir) {
		t.Error("a target with a pipeline must always rebuild")
	}
}

// A missing input is left for buildTarget to report, so it must not be
// mistaken for a fresh target and skipped silently.
func TestIsFreshMissingInput(t *testing.T) {
	_, target := freshProject(t)
	if err := os.Remove(filepath.Join(srcDir, "report.md")); err != nil {
		t.Fatal(err)
	}
	if isFresh(target, distDir) {
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
