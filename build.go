package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func cmdBuild(args []string) error {
	force, dryRun := false, false
	var prefix string
	for _, a := range args {
		switch a {
		case "-f", "--force":
			force = true
		case "-n", "--dry-run":
			dryRun = true
		default:
			if strings.HasPrefix(a, "-") {
				return unknownArg("build", a)
			}
			if prefix != "" {
				return fmt.Errorf("build takes at most one filter argument")
			}
			prefix = a
		}
	}
	m, err := loadManifest()
	if err != nil {
		return err
	}
	dist, err := m.resolveDist()
	if err != nil {
		return err
	}
	if len(m.Targets) == 0 {
		fmt.Printf("No targets in the manifest. Add files to %s/ and run: ditto scan --write\n", srcDir)
		return nil
	}

	var targets []Target
	for _, t := range m.Targets {
		if underPrefix(t.Output, prefix) {
			t.Reference = effectiveReference(m.Project, t)
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		return fmt.Errorf("no targets with output under %q", prefix)
	}
	if err := checkDuplicateOutputs(targets); err != nil {
		return err
	}

	fps := loadFingerprints()
	built, failed, fresh, recorded := 0, 0, 0, false
	for _, t := range targets {
		if !force && isFresh(t, dist, fps) {
			fresh++
			continue
		}
		// A dry run reports the same decision the real build just made, but
		// runs neither the pipeline nor the converter: the pipeline is the
		// stage that writes into src/, so previewing must stop short of it.
		if dryRun {
			fmt.Printf("  %s -> %s\n", strings.Join(srcInputs(t), ", "), filepath.Join(dist, t.Output))
			built++
			continue
		}
		if err := buildTarget(t, dist); err != nil {
			fmt.Fprintf(os.Stderr, "  FAILED %s: %v\n", t.Output, err)
			failed++
			continue
		}
		// Only a target that actually converted is recorded, so a failure is
		// retried on the next build rather than remembered as done. A dry run
		// records nothing at all: it built nothing.
		fps.record(t)
		recorded = true
		built++
	}
	if recorded {
		if err := fps.save(); err != nil {
			fmt.Fprintf(os.Stderr, "ditto: warning: could not record build state (%v); the targets just built may rebuild next time\n", err)
		}
	}
	verb := "Built"
	if dryRun {
		verb = "Would build"
	}
	fmt.Printf("%s %d of %d target(s) into %s/", verb, built, len(targets), dist)
	if fresh > 0 {
		fmt.Printf(" (%d up to date)", fresh)
	}
	fmt.Println()
	if failed > 0 {
		return fmt.Errorf("%d target(s) failed", failed)
	}
	return nil
}

// srcInputs turns a target's manifest-relative inputs into paths relative to
// the project root, which is both what the converter is handed and what the
// build prints.
func srcInputs(t Target) []string {
	rel := t.resolvedInputs()
	ins := make([]string, len(rel))
	for i, p := range rel {
		ins[i] = filepath.Join(srcDir, p)
	}
	return ins
}

// isFresh reports whether a target's output is already newer than everything
// that feeds it and was built from the manifest entry that still describes it,
// so the build can skip converting it again. Rebuilding a set of deliverables
// is a pandoc call per target, which is the slow part of an edit-build-look
// loop where usually one source changed.
//
// Two checks, and both must pass. The timestamps cover the inputs, the styling
// reference, and the converter and pipeline scripts when they are files in the
// project. The fingerprint covers the manifest entry itself, because editing a
// target's view, reference, or converter changes the result without touching
// any file the timestamps watch. Splitting the second question out of the first
// is what lets an edit to one target leave the other forty-nine alone; comparing
// against the manifest's own mtime, as this once did, could only say that
// something in the file had changed and therefore rebuilt everything.
//
// The timestamps stay authoritative and the fingerprint only ever withholds
// freshness, never grants it, so losing the recorded state costs a rebuild
// rather than a stale deliverable. Anything missing or unreadable counts as
// stale too, which lets buildTarget produce the real error rather than this
// returning one.
//
// Two things are deliberately not tracked. A target with a pipeline always
// rebuilds: the pipeline exists to generate or refresh inputs from somewhere
// ditto cannot see, so its inputs' mtimes say nothing about whether the result
// is current. And converters resolved from $PATH are not stat'd, so upgrading
// pandoc or office-convert calls for `ditto build --force`.
func isFresh(t Target, dist string, fps fingerprints) bool {
	if len(t.Pipeline) > 0 {
		return false
	}
	out, err := os.Stat(filepath.Join(dist, t.Output))
	if err != nil {
		return false
	}
	var deps []string
	for _, in := range t.resolvedInputs() {
		deps = append(deps, filepath.Join(srcDir, in))
	}
	if t.Reference != "" {
		deps = append(deps, t.Reference)
	}
	if t.Converter != "" {
		// Only a converter that is a file in the project can be compared; a
		// bare $PATH command has no path to stat here.
		if abs := resolveScript(t.Converter); filepath.IsAbs(abs) {
			deps = append(deps, abs)
		}
	}
	for _, dep := range deps {
		info, err := os.Stat(dep)
		if err != nil || info.ModTime().After(out.ModTime()) {
			return false
		}
	}
	return fps.matches(t)
}

// checkDuplicateOutputs fails the build if two targets write the same output,
// which would otherwise silently clobber: build order alone would decide the
// winner and one deliverable would be lost.
func checkDuplicateOutputs(targets []Target) error {
	seen := map[string]string{} // output -> first target's primary input
	for _, t := range targets {
		key := filepath.ToSlash(t.Output)
		if prev, ok := seen[key]; ok {
			return fmt.Errorf("two targets write the same output %s (inputs %s and %s); each output must be unique", t.Output, prev, primaryInput(t))
		}
		seen[key] = primaryInput(t)
	}
	return nil
}

// primaryInput labels a target by its first input, for diagnostics.
func primaryInput(t Target) string {
	if ins := t.resolvedInputs(); len(ins) > 0 {
		return ins[0]
	}
	return "(no input)"
}

// underPrefix reports whether an output path falls under a filter prefix. An
// empty prefix matches everything; otherwise the prefix must land on a path
// segment boundary, so "D3" matches "D3/report.docx" but not "D30/report.docx".
func underPrefix(output, prefix string) bool {
	if prefix == "" {
		return true
	}
	o := filepath.ToSlash(output)
	p := strings.TrimSuffix(filepath.ToSlash(prefix), "/")
	return o == p || strings.HasPrefix(o, p+"/")
}

func buildTarget(t Target, dist string) error {
	if len(t.resolvedInputs()) == 0 || t.Output == "" {
		return fmt.Errorf("target is missing input or output")
	}
	if err := runPipeline(t); err != nil {
		return err
	}
	ins := srcInputs(t)
	for _, in := range ins {
		if _, err := os.Stat(in); err != nil {
			return fmt.Errorf("input not found: %s", in)
		}
	}
	out := filepath.Join(dist, t.Output)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	cmd, err := converterCmd(t, ins, out)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd.Args[0], err)
	}
	fmt.Printf("  %s -> %s\n", strings.Join(ins, ", "), out)
	return nil
}

// effectiveReference resolves the styling template a target actually uses. A
// reference set on the target always wins; otherwise a .docx or .pptx output
// falls back to the project-level default for that type, so a house style need
// not be repeated on every target. Other output types have no default.
func effectiveReference(p Project, t Target) string {
	if t.Reference != "" {
		return t.Reference
	}
	switch ext(t.Output) {
	case "docx":
		return p.DefaultReferenceDocx
	case "pptx":
		return p.DefaultReferencePptx
	}
	return ""
}

// resolveScript locates a converter or pipeline script the way the project
// expects: a path is relative to the project root, not $PATH. Go's exec only
// does a $PATH lookup for a bare name and never checks the current directory,
// so a bare "enrich.py" would not be found even when it sits in the project. If
// the path exists relative to the root, return it absolute so exec runs that
// file; otherwise leave it for a genuine $PATH lookup.
func resolveScript(path string) string {
	if !filepath.IsAbs(path) {
		if _, err := os.Stat(path); err == nil {
			if abs, err := filepath.Abs(path); err == nil {
				return abs
			}
		}
	}
	return path
}

// runPipeline runs a target's upstream scripts in order before the converter,
// each with no arguments, failing the target on the first non-zero exit. These
// are the computation stages that produce the target's inputs (an extract, an
// enrich), so they run before the inputs are checked for existence.
func runPipeline(t Target) error {
	for _, script := range t.Pipeline {
		cmd := exec.Command(resolveScript(script))
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pipeline %s: %w", script, err)
		}
	}
	return nil
}

// ext returns the lower-case extension without the dot.
func ext(p string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(p), "."))
}

// absInputs turns the src/-relative input paths into absolute paths for the
// INPUTS env var, so a custom converter can find them regardless of its cwd.
func absInputs(ins []string) ([]string, error) {
	abs := make([]string, len(ins))
	for i, p := range ins {
		a, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		abs[i] = a
	}
	return abs, nil
}

// converterCmd resolves the command that turns ins into out. ins[0] is the
// primary input, passed to the converter as its positional argument; the full
// list reaches a custom converter through the INPUTS env var (newline-joined,
// absolute paths). A custom converter overrides the built-ins entirely;
// otherwise the (input ext, output ext) pair selects a built-in. The output
// extension is what picks the format.
func converterCmd(t Target, ins []string, out string) (*exec.Cmd, error) {
	in := ins[0]
	if t.Converter != "" {
		cmd := exec.Command(resolveScript(t.Converter), in, out)
		cmd.Env = os.Environ()
		if t.Reference != "" {
			cmd.Env = append(cmd.Env, "REFERENCE_DOC="+t.Reference)
		}
		if t.View != "" {
			cmd.Env = append(cmd.Env, "VIEW="+t.View)
		}
		abs, err := absInputs(ins)
		if err != nil {
			return nil, err
		}
		cmd.Env = append(cmd.Env, "INPUTS="+strings.Join(abs, "\n"))
		return cmd, nil
	}

	inExt, outExt := ext(in), ext(out)

	// The .xlsx built-in is the only one that accepts multiple inputs: N CSVs
	// become one workbook with a sheet per CSV (named by filename stem), which
	// csv2xlsx writes when handed several inputs and an -o output.
	if len(ins) > 1 {
		if outExt == "xlsx" && allExt(ins, "csv") {
			return exec.Command("csv2xlsx", append(append([]string{}, ins...), "-o", out)...), nil
		}
		return nil, fmt.Errorf("only the .xlsx built-in takes multiple inputs, and only as CSVs; set converter = \"...\" otherwise")
	}

	switch outExt {
	case "docx":
		if isMarkdown(inExt) {
			return pandocCmd("md2docx", t.Reference, in, out), nil
		}
	case "pptx":
		if isMarkdown(inExt) {
			return pandocCmd("md2pptx", t.Reference, in, out), nil
		}
	case "xlsx":
		if inExt == "csv" {
			return exec.Command("csv2xlsx", in, out), nil
		}
	case "html":
		switch inExt {
		case "md", "markdown", "csv", "tsv", "ics", "ical":
			return cleaveCmd(t, in, out), nil
		}
	}
	return nil, fmt.Errorf("no built-in converter for .%s -> .%s; set converter = \"...\" on this target", inExt, outExt)
}

func isMarkdown(e string) bool { return e == "md" || e == "markdown" }

// allExt reports whether every path has the given extension.
func allExt(paths []string, want string) bool {
	for _, p := range paths {
		if ext(p) != want {
			return false
		}
	}
	return true
}

// pandocCmd builds an office-convert invocation. Those scripts read the style
// template from REFERENCE_DOC, so reference is passed through the environment.
func pandocCmd(tool, reference, in, out string) *exec.Cmd {
	cmd := exec.Command(tool, in, out)
	cmd.Env = os.Environ()
	if reference != "" {
		cmd.Env = append(cmd.Env, "REFERENCE_DOC="+reference)
	}
	return cmd
}

// cleaveCmd builds a cleave invocation. cleave takes view and brand as flags
// rather than env, so reference maps to --brand (a brand CSS) and view to
// --view.
func cleaveCmd(t Target, in, out string) *exec.Cmd {
	cargs := []string{in, out}
	if t.View != "" {
		cargs = append(cargs, "--view", t.View)
	}
	if t.Reference != "" {
		cargs = append(cargs, "--brand", t.Reference)
	}
	return exec.Command("cleave", cargs...)
}
