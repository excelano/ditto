package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A renamed target leaves its previous output in dist/, where publish would
// ship it alongside the current one. check has to see it.
func TestOrphanOutputs(t *testing.T) {
	dir := t.TempDir()
	dist := filepath.Join(dir, distDir)
	for _, p := range []string{"Report.docx", "chapters/One.docx", "Old Name.docx", ".hidden"} {
		full := filepath.Join(dist, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	targets := []Target{
		{Input: "report.md", Output: "Report.docx"},
		{Input: "one.md", Output: "chapters/One.docx"},
	}
	got, err := orphanOutputs(targets, dist)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "Old Name.docx" {
		t.Errorf("orphans = %v, want [Old Name.docx] (hidden files are not deliverables)", got)
	}
}

func TestOrphanOutputsNoDist(t *testing.T) {
	if _, err := orphanOutputs(nil, filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("expected an error when dist/ does not exist, so check stays quiet")
	}
}
