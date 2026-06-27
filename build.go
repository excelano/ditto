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
			fmt.Fprintf(os.Stderr, "  FAILED %s: %v\n", t.Input, err)
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
	if t.Input == "" || t.Output == "" {
		return fmt.Errorf("target is missing input or output")
	}
	in := filepath.Join(srcDir, t.Input)
	out := filepath.Join(distDir, t.Output)
	if _, err := os.Stat(in); err != nil {
		return fmt.Errorf("input not found: %s", in)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	cmd, err := converterCmd(t, in, out)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd.Args[0], err)
	}
	fmt.Printf("  %s -> %s\n", in, out)
	return nil
}

// ext returns the lower-case extension without the dot.
func ext(p string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(p), "."))
}

// converterCmd resolves the command that turns in into out. A custom converter
// overrides the built-ins entirely; otherwise the (input ext, output ext) pair
// selects a built-in. The output extension is what picks the format.
func converterCmd(t Target, in, out string) (*exec.Cmd, error) {
	if t.Converter != "" {
		cmd := exec.Command(t.Converter, in, out)
		cmd.Env = os.Environ()
		if t.Reference != "" {
			cmd.Env = append(cmd.Env, "REFERENCE_DOC="+t.Reference)
		}
		if t.View != "" {
			cmd.Env = append(cmd.Env, "VIEW="+t.View)
		}
		return cmd, nil
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
