package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	manifestName = "Ditto.toml"
	// The manifest was called Manifest.toml through v0.1.0. Kept only to give
	// a project written against that name a pointed error instead of a bare
	// "no Ditto.toml"; it is never read.
	legacyManifestName = "Manifest.toml"
	srcDir             = "src"
	distDir            = "dist"
)

// Manifest is the authoritative description of a ditto project: nothing is
// built that is not listed here.
type Manifest struct {
	Project Project  `toml:"project"`
	Publish *Publish `toml:"publish"` // optional; nil when the section is absent
	Targets []Target `toml:"target"`
}

type Project struct {
	Name                 string `toml:"name"`
	Dist                 string `toml:"dist"`                   // optional; overrides the dist/ output dir (~ expanded). src/ is always fixed.
	DefaultReferenceDocx string `toml:"default_reference_docx"` // fallback reference for .docx targets
	DefaultReferencePptx string `toml:"default_reference_pptx"` // fallback reference for .pptx targets
}

// Publish names where `ditto publish` mirrors dist/. Root is a SharePoint
// library URL or a local path.
type Publish struct {
	Root string `toml:"root"`
}

// Target builds one file from src/ into one file under dist/. The output
// extension selects the converter unless Converter overrides it.
type Target struct {
	Input     string   `toml:"input"`     // relative to src/
	Inputs    []string `toml:"inputs"`    // optional; extra sources, relative to src/ (custom converter only)
	Output    string   `toml:"output"`    // relative to dist/
	Reference string   `toml:"reference"` // optional styling template
	View      string   `toml:"view"`      // optional (cleave/html only)
	Converter string   `toml:"converter"` // optional override
	Pipeline  []string `toml:"pipeline"`  // optional scripts to run, in order, before the converter
}

// resolvedInputs returns every source path the target consumes (input first if
// set, then inputs), joined under src/. Order is stable: the first element is
// the primary input passed to the converter as its positional argument.
func (t Target) resolvedInputs() []string {
	var ins []string
	if t.Input != "" {
		ins = append(ins, t.Input)
	}
	ins = append(ins, t.Inputs...)
	return ins
}

// resolveDist returns where build writes and publish reads: the project's dist
// override if set (~ expanded), else the default dist/ under the project root.
// src/ is deliberately not overridable, so a dist that resolves inside src/ is
// rejected — building there would write outputs into the curated source tree.
func (m *Manifest) resolveDist() (string, error) {
	dist := distDir
	if strings.TrimSpace(m.Project.Dist) != "" {
		dist = expandHome(m.Project.Dist)
	}
	absSrc, errSrc := filepath.Abs(srcDir)
	absDist, errDist := filepath.Abs(dist)
	if errSrc == nil && errDist == nil {
		if absDist == absSrc || strings.HasPrefix(absDist, absSrc+string(os.PathSeparator)) {
			return "", fmt.Errorf("dist %q resolves inside %s/; build would write into the source tree", dist, srcDir)
		}
	}
	return dist, nil
}

// findProjectRoot walks up from the current directory looking for the manifest,
// the way cargo finds Cargo.toml, so the verbs work from anywhere inside a
// project instead of only at its root. It returns the directory holding the
// manifest.
func findProjectRoot() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", err
	}
	legacy := ""
	for dir := start; ; {
		if _, err := os.Stat(filepath.Join(dir, manifestName)); err == nil {
			return dir, nil
		}
		// Remember the nearest project written against the pre-v0.2.0 name so
		// the failure can point at it rather than report a missing manifest.
		if legacy == "" {
			if _, err := os.Stat(filepath.Join(dir, legacyManifestName)); err == nil {
				legacy = dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if legacy != "" {
		return "", fmt.Errorf("no %s in %s or any parent, but %s has a %s: the manifest was renamed after v0.1.0. Rename it (mv %s %s) — the contents are unchanged",
			manifestName, start, legacy, legacyManifestName, legacyManifestName, manifestName)
	}
	return "", fmt.Errorf("no %s in %s or any parent", manifestName, start)
}

// loadManifest finds the project root and reads the manifest there. It moves
// the process into that root, so every relative path that follows — src/,
// dist/, converter and pipeline scripts — resolves against the project rather
// than against wherever the user happened to be standing. Every verb that
// reads a manifest goes through here, so none of them has to think about it.
func loadManifest() (*Manifest, error) {
	root, err := findProjectRoot()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if root != cwd {
		if err := os.Chdir(root); err != nil {
			return nil, fmt.Errorf("entering project root %s: %w", root, err)
		}
		// The output below names paths like dist/ relative to the root, which
		// is not where the user is standing — so say where that is.
		fmt.Fprintf(os.Stderr, "ditto: project root %s\n", root)
	}
	var m Manifest
	md, err := toml.DecodeFile(manifestName, &m)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", manifestName, err)
	}
	for _, k := range md.Undecoded() {
		fmt.Fprintf(os.Stderr, "ditto: warning: unknown key %q in %s\n", k.String(), manifestName)
	}
	return &m, nil
}
