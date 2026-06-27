package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// defaultOutput maps a source extension to the output extension scan assigns
// when adding a target. Only the unambiguous cases live here; everything else
// (md->pptx, md->html, json->...) is left for a human to declare.
var defaultOutput = map[string]string{
	"md":       "docx",
	"markdown": "docx",
	"csv":      "xlsx",
}

func cmdScan(args []string) error {
	write := false
	for _, a := range args {
		switch a {
		case "--write":
			write = true
		default:
			return fmt.Errorf("scan: unknown argument %q", a)
		}
	}

	m, err := loadManifest()
	if err != nil {
		return err
	}

	covered := map[string]bool{}
	for _, t := range m.Targets {
		covered[filepath.ToSlash(t.Input)] = true
	}

	missing, err := uncoveredInputs(covered)
	if err != nil {
		return err
	}

	if len(missing) == 0 {
		fmt.Printf("All files in %s/ are covered by the manifest.\n", srcDir)
		return nil
	}

	if !write {
		fmt.Printf("Uncovered files in %s/ (%d):\n", srcDir, len(missing))
		for _, f := range missing {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println("\nRun 'ditto scan --write' to add targets with default outputs.")
		return nil
	}

	return appendTargets(missing)
}

// uncoveredInputs walks src/ and returns the slash-separated paths (relative to
// src/) that no target's input already covers.
func uncoveredInputs(covered map[string]bool) ([]string, error) {
	if _, err := os.Stat(srcDir); err != nil {
		return nil, fmt.Errorf("no %s/ directory", srcDir)
	}
	var missing []string
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !covered[rel] {
			missing = append(missing, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(missing)
	return missing, nil
}

// appendTargets appends a [[target]] block per uncovered file to the manifest.
// It appends raw text rather than re-marshalling the whole manifest so existing
// hand-edits, ordering, and comments survive.
func appendTargets(missing []string) error {
	var b strings.Builder
	var skipped []string
	added := 0
	for _, in := range missing {
		outExt, ok := defaultOutput[ext(in)]
		if !ok {
			skipped = append(skipped, in)
			continue
		}
		out := strings.TrimSuffix(in, filepath.Ext(in)) + "." + outExt
		fmt.Fprintf(&b, "\n[[target]]\ninput = %s\noutput = %s\n", tomlString(in), tomlString(out))
		added++
	}

	if added > 0 {
		f, err := os.OpenFile(manifestName, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.WriteString(b.String()); err != nil {
			return err
		}
	}

	fmt.Printf("Added %d target(s) to %s.\n", added, manifestName)
	for _, s := range skipped {
		fmt.Printf("  skipped %s (no default output for .%s; add a target by hand)\n", s, ext(s))
	}
	return nil
}

// tomlString renders s as a TOML basic string.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
