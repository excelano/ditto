package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdNew(args []string) error {
	// The name is a positional, so a flag must be rejected before it is taken
	// for one: otherwise a mistyped flag becomes the project directory.
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return unknownArg("new", a)
		}
	}
	if len(args) != 1 {
		return usagef("usage: ditto new <name>")
	}
	name := args[0]
	if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("%s already exists", name)
	}
	if err := scaffold(name, filepath.Base(name)); err != nil {
		return err
	}
	fmt.Printf("Created %s/\n", name)
	reportScaffold()
	return nil
}

// cmdInit scaffolds a project around files that are already here, for the
// common case of a folder of sources that predates the decision to build it
// with ditto. It is `new` without the new directory: nothing existing is
// overwritten, and src/ keeps whatever it already holds.
func cmdInit(args []string) error {
	if len(args) != 0 {
		return unknownArg("init", args[0])
	}
	if _, err := os.Stat(manifestName); err == nil {
		return fmt.Errorf("%s already exists; this is already a ditto project", manifestName)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// Nothing here is a manifest yet, so a root found above us belongs to
	// another project that this one would shadow — worth saying, not refusing.
	parent, parentErr := findProjectRoot()

	if err := scaffold(".", filepath.Base(cwd)); err != nil {
		return err
	}
	fmt.Printf("Initialised %s in %s\n", manifestName, cwd)
	reportScaffold()
	if parentErr == nil {
		fmt.Fprintf(os.Stderr, "\nditto: note: %s is also a ditto project; commands run here will use this one\n", parent)
	}
	return nil
}

// scaffold writes the project skeleton into dir, which may already exist and
// hold files. Every write is additive: an existing manifest is never clobbered
// (callers check first), an existing .gitignore is appended to rather than
// replaced, and .gitkeep is only for an empty src/ that would otherwise not
// survive a commit.
func scaffold(dir, name string) error {
	for _, d := range []string{filepath.Join(dir, srcDir), filepath.Join(dir, distDir)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), []byte(starterManifest(name)), 0o644); err != nil {
		return err
	}
	if err := ensureGitignore(filepath.Join(dir, ".gitignore")); err != nil {
		return err
	}
	return ensureGitkeep(filepath.Join(dir, srcDir))
}

// ensureGitignore makes sure the build's outputs and its recorded state are
// ignored without discarding rules the directory already had. Both are products
// of a build rather than things anyone wrote, and both are reproduced by
// rerunning it.
func ensureGitignore(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	ignored := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		ignored[strings.TrimSpace(line)] = true
	}
	add := ""
	for _, dir := range []string{distDir, stateDir} {
		// A rule already covering the directory in any of its spellings is left
		// exactly as the author wrote it.
		if ignored["/"+dir+"/"] || ignored["/"+dir] || ignored[dir+"/"] || ignored[dir] {
			continue
		}
		add += "/" + dir + "/\n"
	}
	if add == "" {
		return nil
	}
	body := string(existing)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return os.WriteFile(path, []byte(body+add), 0o644)
}

// ensureGitkeep adds a placeholder only to an empty src/, so adopting a
// directory that already has sources does not litter it.
func ensureGitkeep(src string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.WriteFile(filepath.Join(src, ".gitkeep"), nil, 0o644)
}

func reportScaffold() {
	fmt.Printf("  %s/   your text sources (versioned)\n", srcDir)
	fmt.Printf("  %s/  built deliverables (gitignored)\n", distDir)
	fmt.Printf("  %s\n", manifestName)
	fmt.Printf("\nAdd files to %s/, then: ditto scan --write && ditto build\n", srcDir)
}

func starterManifest(name string) string {
	return fmt.Sprintf(`[project]
name = %s

# Optional. Override where 'ditto build' writes and 'ditto publish' reads.
# Defaults to dist/ under the project; ~ is expanded. src/ is always fixed.
# dist = "~/%s-stage"

# Optional. Where 'ditto publish' mirrors dist/.
# A SharePoint library URL or a local path. Delete this section to skip publishing.
# [publish]
# root = "https://contoso.sharepoint.com/sites/Engagement/Shared Documents/Deliverables"

# Each target builds one file from src/ into dist/. The output extension picks
# the converter: .docx and .pptx (from .md), .xlsx (from .csv), .html (from
# .md/.csv/.ics via cleave). Run 'ditto scan --write' to fill these in from the
# files already in src/.
#
# [[target]]
# input = "report.md"                  # relative to src/
# output = "Final Report.docx"         # relative to dist/
# reference = "brand/excelano.docx"    # optional styling template
# view = "slides"                      # optional (cleave/html only)
# converter = "converters/custom.sh"   # optional override
`, tomlString(name), name)
}
