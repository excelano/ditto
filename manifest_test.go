package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newProject lays out a project root holding a manifest, plus the nested
// directory the test will run from.
func newProject(t *testing.T, manifest, nested string) (root, deep string) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, manifest), []byte("[project]\nname = \"demo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep = filepath.Join(root, nested)
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, deep
}

func TestFindProjectRootFromNestedDir(t *testing.T) {
	root, deep := newProject(t, manifestName, "src/chapters")
	t.Chdir(deep)

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	// t.TempDir can hand back a symlinked path (/tmp -> /private/tmp and the
	// like), so compare what the filesystem resolves both sides to.
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	if gotResolved != want {
		t.Errorf("root = %q, want %q", gotResolved, want)
	}
}

func TestFindProjectRootAtRootItself(t *testing.T) {
	root, _ := newProject(t, manifestName, "src")
	t.Chdir(root)

	if _, err := findProjectRoot(); err != nil {
		t.Fatalf("findProjectRoot at the root: %v", err)
	}
}

func TestFindProjectRootMissing(t *testing.T) {
	t.Chdir(t.TempDir())

	_, err := findProjectRoot()
	if err == nil {
		t.Fatal("expected an error with no manifest anywhere above")
	}
	if !strings.Contains(err.Error(), "or any parent") {
		t.Errorf("error should say the walk went up: %v", err)
	}
}

// A project written against v0.1.0 gets the rename hint, not a bare "missing
// manifest" — including when the stale manifest is in a parent directory.
func TestFindProjectRootLegacyName(t *testing.T) {
	_, deep := newProject(t, legacyManifestName, "src/chapters")
	t.Chdir(deep)

	_, err := findProjectRoot()
	if err == nil {
		t.Fatal("expected an error for a project still on the old manifest name")
	}
	if !strings.Contains(err.Error(), "mv "+legacyManifestName+" "+manifestName) {
		t.Errorf("error should carry the rename hint: %v", err)
	}
}

// The walk must not confuse a stale manifest in a parent with the real one:
// the nearest Ditto.toml wins even when an ancestor holds the old name.
func TestFindProjectRootPrefersManifestOverLegacyAbove(t *testing.T) {
	outer, inner := newProject(t, legacyManifestName, "nested")
	if err := os.WriteFile(filepath.Join(inner, manifestName), []byte("[project]\nname = \"inner\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(inner)

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	if filepath.Base(got) != "nested" {
		t.Errorf("root = %q, want the inner project, not %q", got, outer)
	}
}
