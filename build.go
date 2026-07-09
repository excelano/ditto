package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func cmdBuild(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("build takes no arguments")
	}
	m, err := loadManifest()
	if err != nil {
		return err
	}
	if len(m.Targets) == 0 {
		fmt.Printf("No targets in the manifest. Add files to %s/ and run: ditto scan --write\n", srcDir)
		return nil
	}

	built, failed := 0, 0
	for _, t := range m.Targets {
		if err := buildTarget(t); err != nil {
			fmt.Fprintf(os.Stderr, "  FAILED %s: %v\n", t.Output, err)
			failed++
			continue
		}
		built++
	}
	fmt.Printf("Built %d of %d target(s) into %s/\n", built, len(m.Targets), distDir)
	if failed > 0 {
		return fmt.Errorf("%d target(s) failed", failed)
	}
	return nil
}

func buildTarget(t Target) error {
	rel := t.resolvedInputs()
	if len(rel) == 0 || t.Output == "" {
		return fmt.Errorf("target is missing input or output")
	}
	if err := runPipeline(t); err != nil {
		return err
	}
	ins := make([]string, len(rel))
	for i, p := range rel {
		ins[i] = filepath.Join(srcDir, p)
		if _, err := os.Stat(ins[i]); err != nil {
			return fmt.Errorf("input not found: %s", ins[i])
		}
	}
	out := filepath.Join(distDir, t.Output)
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

// runPipeline runs a target's upstream scripts in order before the converter,
// each with no arguments, failing the target on the first non-zero exit. These
// are the computation stages that produce the target's inputs (an extract, an
// enrich), so they run before the inputs are checked for existence.
func runPipeline(t Target) error {
	for _, script := range t.Pipeline {
		cmd := exec.Command(script)
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
		cmd := exec.Command(t.Converter, in, out)
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

	if len(ins) > 1 {
		return nil, fmt.Errorf("built-in converters take a single input; set converter = \"...\" to consume multiple inputs")
	}

	inExt, outExt := ext(in), ext(out)
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
