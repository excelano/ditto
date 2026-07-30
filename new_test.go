package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Both the outputs and the build's recorded state are products of a build, so
// neither belongs in the repository.
const scaffoldedIgnores = "/dist/\n/.ditto/\n"

// init adopts directories that already exist, so the scaffold must add to what
// is there rather than replace it.
func TestEnsureGitignoreCreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitignore")
	if err := ensureGitignore(path); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != scaffoldedIgnores {
		t.Errorf("gitignore = %q, want %q", got, scaffoldedIgnores)
	}
}

func TestEnsureGitignoreAppendsToExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitignore")
	if err := os.WriteFile(path, []byte("node_modules/\n*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureGitignore(path); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if !strings.HasPrefix(got, "node_modules/\n*.log\n") {
		t.Errorf("existing rules must survive, got %q", got)
	}
	if !strings.Contains(got, "/dist/") {
		t.Errorf("dist/ must be ignored, got %q", got)
	}
	if !strings.Contains(got, "/"+stateDir+"/") {
		t.Errorf("%s must be ignored, got %q", stateDir, got)
	}
}

// A file with no trailing newline must not have the new rule glued onto its
// last line.
func TestEnsureGitignoreAddsMissingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitignore")
	if err := os.WriteFile(path, []byte("*.log"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureGitignore(path); err != nil {
		t.Fatal(err)
	}
	want := "*.log\n" + scaffoldedIgnores
	if got := read(t, path); got != want {
		t.Errorf("gitignore = %q, want %q", got, want)
	}
}

// A rule the author already wrote is left in their spelling; only what is
// genuinely missing gets appended.
func TestEnsureGitignoreLeavesEquivalentRuleAlone(t *testing.T) {
	for _, existing := range []string{"/dist/\n", "dist/\n", "dist\n", "/dist\n"} {
		path := filepath.Join(t.TempDir(), ".gitignore")
		if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := ensureGitignore(path); err != nil {
			t.Fatal(err)
		}
		want := existing + "/" + stateDir + "/\n"
		if got := read(t, path); got != want {
			t.Errorf("%q was already ignoring dist, rewritten to %q, want %q", existing, got, want)
		}
	}
}

// Running init twice, or on a project scaffolded by an earlier ditto, must not
// keep appending rules that are already there.
func TestEnsureGitignoreIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitignore")
	for i := 0; i < 3; i++ {
		if err := ensureGitignore(path); err != nil {
			t.Fatal(err)
		}
	}
	if got := read(t, path); got != scaffoldedIgnores {
		t.Errorf("gitignore = %q after three passes, want %q", got, scaffoldedIgnores)
	}
}

func TestEnsureGitkeepSkipsPopulatedSrc(t *testing.T) {
	src := filepath.Join(t.TempDir(), srcDir)
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "report.md"), []byte("# R\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureGitkeep(src); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, ".gitkeep")); err == nil {
		t.Error("a src/ that already has sources does not need a .gitkeep")
	}
}

func TestEnsureGitkeepFillsEmptySrc(t *testing.T) {
	src := filepath.Join(t.TempDir(), srcDir)
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensureGitkeep(src); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, ".gitkeep")); err != nil {
		t.Error("an empty src/ needs a .gitkeep to survive a commit")
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
