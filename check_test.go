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

// SharePoint is case-preserving but case-insensitive, so outputs that differ
// only in case are one item there. The folder form is what a manifest mixing
// hand-named "D3/…" targets with scanned "d3/…" ones produces.
func TestCaseCollisions(t *testing.T) {
	cases := []struct {
		name    string
		targets []Target
		want    []collision
	}{
		{
			name:    "distinct outputs",
			targets: []Target{{Output: "D3/Report.docx"}, {Output: "D4/Register.xlsx"}},
		},
		{
			name:    "same folder in two spellings",
			targets: []Target{{Output: "D3/Report.docx"}, {Output: "d3/checklist.docx"}},
			want:    []collision{{kind: "folder", spellings: `"D3" and "d3"`}},
		},
		{
			name:    "same file in two spellings",
			targets: []Target{{Output: "Report.docx"}, {Output: "report.DOCX"}},
			want:    []collision{{kind: "file", spellings: `"Report.docx" and "report.DOCX"`}},
		},
		{
			name:    "collision deep in the tree",
			targets: []Target{{Output: "D3/Exhibits/a.docx"}, {Output: "D3/exhibits/b.docx"}},
			want:    []collision{{kind: "folder", spellings: `"D3/Exhibits" and "D3/exhibits"`}},
		},
		{
			// One spelling used many times is how a correct manifest looks.
			name:    "repeated folder in one spelling",
			targets: []Target{{Output: "D3/a.docx"}, {Output: "D3/b.docx"}, {Output: "D3/c.docx"}},
		},
		{
			name:    "empty outputs are left to the per-target checks",
			targets: []Target{{Output: ""}, {Output: ""}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := caseCollisions(tc.targets)
			if len(got) != len(tc.want) {
				t.Fatalf("collisions = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("collision %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// Every converter ditto dispatches to on its own is one a user never chose and
// so cannot be expected to know the provenance of: nothing about the name
// md2docx says "office-convert". The hint is what makes the report actionable,
// and the risk is that a new built-in is added to converterCmd without one.
// Driving the real dispatch rather than restating the names guards that class.
func TestEveryBuiltInConverterHasAnInstallHint(t *testing.T) {
	pairs := []struct{ in, out string }{
		{"report.md", "Report.docx"},
		{"deck.md", "Deck.pptx"},
		{"data.csv", "Data.xlsx"},
		{"notes.md", "Notes.html"},
		{"cal.ics", "Cal.html"},
	}
	for _, p := range pairs {
		cmd, err := converterCmd(Target{Input: p.in, Output: p.out}, []string{p.in}, p.out)
		if err != nil {
			t.Fatalf("no built-in for %s -> %s: %v", p.in, p.out, err)
		}
		if installHint(cmd.Args[0]) == "" {
			t.Errorf("built-in converter %q has no install hint", cmd.Args[0])
		}
	}
	// xsync is not reached through converterCmd, but publish needs it and check
	// reports it the same way.
	if installHint("xsync") == "" {
		t.Error("xsync has no install hint")
	}
	if installHint("./my-converter.sh") != "" {
		t.Error("a custom converter should get no hint; ditto cannot know where it came from")
	}
}
