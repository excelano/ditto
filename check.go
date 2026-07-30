package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// cmdCheck validates a manifest without converting anything. Everything it
// reports is something build would otherwise hit partway through, after some
// targets had already written files — a missing source, a reference that moved,
// a converter that is not installed on this machine. It is the cheap pass to
// run after hand-editing the manifest, and before handing a project to someone
// whose machine may not have the same tools.
func cmdCheck(args []string) error {
	if len(args) != 0 {
		return unknownArg("check", args[0])
	}
	m, err := loadManifest()
	if err != nil {
		return err
	}

	var problems, notes []string
	problem := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	note := func(format string, a ...any) { notes = append(notes, fmt.Sprintf(format, a...)) }

	if strings.TrimSpace(m.Project.Name) == "" {
		note("[project] has no name")
	}
	dist, err := m.resolveDist()
	if err != nil {
		problem("%v", err)
		dist = distDir // keep checking the targets against the default
	}
	if len(m.Targets) == 0 {
		note("no targets; run 'ditto scan --write' to add one per file in %s/", srcDir)
	}
	if err := checkDuplicateOutputs(m.Targets); err != nil {
		problem("%v", err)
	}
	for _, d := range []string{m.Project.DefaultReferenceDocx, m.Project.DefaultReferencePptx} {
		if d != "" {
			if _, err := os.Stat(d); err != nil {
				problem("default reference not found: %s", d)
			}
		}
	}

	tools := map[string][]string{} // external command -> outputs needing it
	for _, t := range m.Targets {
		t.Reference = effectiveReference(m.Project, t)
		checkTarget(t, dist, tools, problem)
	}
	for _, cmd := range sortedKeys(tools) {
		if _, err := exec.LookPath(cmd); err != nil {
			problem("%s is not on PATH, needed by: %s%s", cmd, strings.Join(tools[cmd], ", "), installHint(cmd))
		}
	}
	if orphans, err := orphanOutputs(m.Targets, dist); err == nil {
		for _, o := range orphans {
			note("%s is in %s/ but no target produces it; 'ditto clean' clears it", o, dist)
		}
	}

	// A case-only difference is two paths locally and one on SharePoint. It is
	// a real fault when there is a remote root to publish to and merely untidy
	// otherwise, so it fails the check only in the first case.
	report := note
	if m.Publish != nil && isRemote(m.Publish.Root) {
		report = problem
	}
	for _, c := range caseCollisions(m.Targets) {
		report("%s, which a SharePoint library treats as one %s", c.spellings, c.kind)
	}

	for _, n := range notes {
		fmt.Printf("  note   %s\n", n)
	}
	for _, p := range problems {
		fmt.Fprintf(os.Stderr, "  error  %s\n", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s: %d problem(s) in %d target(s)", manifestName, len(problems), len(m.Targets))
	}
	fmt.Printf("%s: %d target(s), no problems.\n", manifestName, len(m.Targets))
	return nil
}

// checkTarget validates one target and records the external command it needs,
// so a converter missing from this machine is reported once rather than per
// target that wants it.
func checkTarget(t Target, dist string, tools map[string][]string, problem func(string, ...any)) {
	label := t.Output
	if label == "" {
		label = primaryInput(t)
	}
	ins := t.resolvedInputs()
	if len(ins) == 0 {
		problem("%s: no input", label)
		return
	}
	if t.Output == "" {
		problem("%s: no output", primaryInput(t))
		return
	}

	paths := make([]string, len(ins))
	for i, in := range ins {
		paths[i] = filepath.Join(srcDir, in)
		// A pipeline generates its target's inputs, so they are allowed to be
		// absent until it has run.
		if len(t.Pipeline) == 0 {
			if _, err := os.Stat(paths[i]); err != nil {
				problem("%s: input not found: %s", label, paths[i])
			}
		}
	}
	if t.Reference != "" {
		if _, err := os.Stat(t.Reference); err != nil {
			problem("%s: reference not found: %s", label, t.Reference)
		}
	}
	for _, script := range t.Pipeline {
		if _, err := exec.LookPath(resolveScript(script)); err != nil {
			problem("%s: pipeline script not runnable: %s", label, script)
		}
	}

	// Resolving the command is the same dispatch build uses, so an unbuildable
	// (input ext -> output ext) pair surfaces here with the identical message.
	cmd, err := converterCmd(t, paths, filepath.Join(dist, t.Output))
	if err != nil {
		problem("%s: %v", label, err)
		return
	}
	if t.Converter != "" {
		if _, err := exec.LookPath(cmd.Args[0]); err != nil {
			problem("%s: converter not runnable: %s", label, t.Converter)
		}
		return
	}
	tools[cmd.Args[0]] = append(tools[cmd.Args[0]], label)
}

// orphanOutputs lists files in dist/ that no target produces: the residue of a
// renamed or deleted target, which publish would otherwise ship alongside the
// real deliverables.
func orphanOutputs(targets []Target, dist string) ([]string, error) {
	if _, err := os.Stat(dist); err != nil {
		return nil, err
	}
	produced := map[string]bool{}
	for _, t := range targets {
		if t.Output != "" {
			produced[filepath.ToSlash(filepath.Clean(t.Output))] = true
		}
	}
	var orphans []string
	err := filepath.WalkDir(dist, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(dist, file)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !produced[rel] {
			orphans = append(orphans, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(orphans)
	return orphans, nil
}

// collision is one set of output paths that a case-insensitive destination
// cannot tell apart.
type collision struct {
	kind      string // "file" or "folder"
	spellings string // the differing spellings, quoted and joined
}

// caseCollisions finds outputs that differ only by case. A local filesystem
// keeps them apart; SharePoint and OneDrive are case-preserving but
// case-insensitive, so they are the same item there. Two files that collide
// overwrite each other on publish with nothing reported, and two folders merge
// their contents into whichever spelling the library already has.
//
// The folder case is the one that shows up in practice: `ditto scan --write`
// derives outputs from the src/ tree, so a manifest that mixes hand-named
// "D3/…" targets with scanned "d3/…" ones asks for two folders that the library
// sees as one.
func caseCollisions(targets []Target) []collision {
	files := map[string]map[string]bool{}   // lower-cased path -> spellings seen
	folders := map[string]map[string]bool{} // lower-cased dir  -> spellings seen

	add := func(index map[string]map[string]bool, p string) {
		key := strings.ToLower(p)
		if index[key] == nil {
			index[key] = map[string]bool{}
		}
		index[key][p] = true
	}
	for _, t := range targets {
		if t.Output == "" {
			continue
		}
		out := filepath.ToSlash(filepath.Clean(t.Output))
		add(files, out)
		for dir := path.Dir(out); dir != "." && dir != "/"; dir = path.Dir(dir) {
			add(folders, dir)
		}
	}

	var found []collision
	for _, c := range []struct {
		kind  string
		index map[string]map[string]bool
	}{{"file", files}, {"folder", folders}} {
		for _, key := range sortedKeys(c.index) {
			if len(c.index[key]) < 2 {
				continue
			}
			found = append(found, collision{kind: c.kind, spellings: quotedList(c.index[key])})
		}
	}
	return found
}

// quotedList renders the differing spellings in a stable order, so the same
// manifest always produces the same message.
func quotedList(set map[string]bool) string {
	items := make([]string, 0, len(set))
	for s := range set {
		items = append(items, `"`+s+`"`)
	}
	sort.Strings(items)
	return strings.Join(items, " and ")
}

// installHint names where a missing built-in converter comes from, so the
// report answers "now what" rather than only "not found". ditto orchestrates
// tools it does not ship, which makes "not on PATH" the expected first
// experience on a new machine rather than an exotic failure — and knowing that
// md2docx lives in a package called office-convert is not something the name
// tells you. A custom converter from the manifest gets no hint, correctly:
// ditto has no idea where someone else's script comes from.
func installHint(cmd string) string {
	switch cmd {
	case "md2docx", "md2pptx", "csv2xlsx":
		return " (sudo apt install office-convert)"
	case "xsync":
		return " (sudo apt install xfiles)"
	case "cleave":
		return " (sudo apt install cleave)"
	}
	return ""
}

// sortedKeys keeps every list check prints in a stable order, so the same
// manifest always produces the same report.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
