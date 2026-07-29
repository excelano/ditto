package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
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
		return fmt.Errorf("usage: ditto check")
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
			problem("%s is not on PATH, needed by: %s", cmd, strings.Join(tools[cmd], ", "))
		}
	}
	if orphans, err := orphanOutputs(m.Targets, dist); err == nil {
		for _, o := range orphans {
			note("%s is in %s/ but no target produces it; 'ditto clean' clears it", o, dist)
		}
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
	err := filepath.WalkDir(dist, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(dist, path)
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

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
