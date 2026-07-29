package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// init adopts directories that already exist, so the scaffold must add to what
// is there rather than replace it.
func TestEnsureGitignoreCreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitignore")
	if err := ensureGitignore(path); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "/dist/\n" {
		t.Errorf("gitignore = %q, want %q", got, "/dist/\n")
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
	if got := read(t, path); got != "*.log\n/dist/\n" {
		t.Errorf("gitignore = %q, want %q", got, "*.log\n/dist/\n")
	}
}

func TestEnsureGitignoreLeavesEquivalentRuleAlone(t *testing.T) {
	for _, existing := range []string{"/dist/\n", "dist/\n", "dist\n", "/dist\n"} {
		path := filepath.Join(t.TempDir(), ".gitignore")
		if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := ensureGitignore(path); err != nil {
			t.Fatal(err)
		}
		if got := read(t, path); got != existing {
			t.Errorf("%q was already ignoring dist, rewritten to %q", existing, got)
		}
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
