package main

import (
	"os"
	"path/filepath"
	"testing"
)

// dist is ~-expanded and hand-edited, so these are the typos that would aim a
// recursive remove at something outside the project.
func TestRefuseUnsafeClean(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	cwd, err := os.Getwd() // t.TempDir may hand back a symlinked path
	if err != nil {
		t.Fatal(err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory to test against")
	}
	absHome, err := filepath.Abs(home)
	if err != nil {
		t.Fatal(err)
	}

	refused := map[string]string{
		"filesystem root":         string(os.PathSeparator),
		"home directory":          absHome,
		"the project itself":      cwd,
		"a parent of the project": filepath.Dir(cwd),
	}
	for name, dist := range refused {
		if err := refuseUnsafeClean(dist); err == nil {
			t.Errorf("clean should refuse %s (%s)", name, dist)
		}
	}

	if err := refuseUnsafeClean(filepath.Join(cwd, distDir)); err != nil {
		t.Errorf("clean should allow the project's own dist/: %v", err)
	}
}

func TestCountFiles(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"a.docx", "sub/b.docx", "sub/deep/c.xlsx"} {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	n, err := countFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("countFiles = %d, want 3 (directories are not files)", n)
	}
}
