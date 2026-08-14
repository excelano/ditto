package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	Project  Project            `toml:"project"`
	Publish  *Publish           `toml:"publish"` // optional; nil when the section is absent
	Profiles map[string]Profile `toml:"profile"` // optional; keyed by profile name
	Targets  []Target           `toml:"target"`
}

// Profile overrides styling for a whole build rather than per target, so a
// deliverable set can be worked for days against a plain template and shipped
// once against the branded one.
//
// A set field wins over both the project default and a target's own value.
// That is the point rather than an accident: a draft build has to reach the
// targets that name their reference explicitly, and those are exactly the ones
// a project default would miss.
//
// Deliberately narrow. `converter` is not overridable, because changing it
// changes what the deliverable is rather than how it looks, and a profile that
// can do that is a second manifest. Nor can a profile exclude targets: the
// prefix filter already builds one slice, and two ways to choose targets that
// compose differently is a worse answer than one.
type Profile struct {
	ReferenceDocx string `toml:"reference_docx"` // styling reference for .docx targets
	ReferencePptx string `toml:"reference_pptx"` // styling reference for .pptx targets
	View          string `toml:"view"`           // view for targets that already have one
}

// apply returns t with this profile's overrides in place. It is called after
// the project-level reference default has been resolved, so it overrides the
// effective value whatever its source.
//
// View only replaces a view a target already has; it never introduces one. A
// target with no view either does not take one, or takes its converter's
// default, and a custom converter is handed VIEW in its environment — so
// introducing a view here would change what those converters do rather than
// how the output looks.
func (p Profile) apply(t Target) Target {
	switch ext(t.Output) {
	case "docx":
		if p.ReferenceDocx != "" {
			t.Reference = p.ReferenceDocx
		}
	case "pptx":
		if p.ReferencePptx != "" {
			t.Reference = p.ReferencePptx
		}
	}
	if p.View != "" && t.View != "" {
		t.View = p.View
	}
	return t
}

// lookupProfile resolves a named profile, or fails naming what is available.
// An unknown profile is a typo far more often than a missing section, and
// silently building unprofiled would ship the branded template while the
// terminal said "draft".
func (m *Manifest) lookupProfile(name string) (Profile, error) {
	if name == "" {
		return Profile{}, nil
	}
	if p, ok := m.Profiles[name]; ok {
		return p, nil
	}
	if len(m.Profiles) == 0 {
		return Profile{}, fmt.Errorf("no profile %q: %s defines none", name, manifestName)
	}
	names := make([]string, 0, len(m.Profiles))
	for n := range m.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return Profile{}, fmt.Errorf("no profile %q; %s defines: %s", name, manifestName, strings.Join(names, ", "))
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

// notAProjectError says there is no project here or above here. It is a
// distinct type because `status` treats it as a state to report rather than a
// failure to raise, while every other verb — each of which was about to do
// something that needs a project — treats it like any other error.
type notAProjectError struct{ start string }

func (e *notAProjectError) Error() string {
	return fmt.Sprintf("no %s in %s or any parent", manifestName, e.start)
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
	return "", &notAProjectError{start: start}
}

// loadManifest finds the project root and reads the manifest there. It moves
// the process into that root, so every relative path that follows — src/,
// dist/, converter and pipeline scripts — resolves against the project rather
// than against wherever the user happened to be standing. Every verb that
// reads a manifest goes through here, so none of them has to think about it.
//
// Moving the process is invisible to someone standing in a subdirectory, and
// the output that follows names paths like dist/ relative to the root, so the
// move is announced on stderr.
func loadManifest() (*Manifest, error) {
	return loadManifestAt(true)
}

// loadManifestQuiet loads without announcing the move to the project root. It
// exists for `status`, whose first line is the project root: announcing it too
// would print the same path twice, once as a warning about something the
// command was asked to report.
func loadManifestQuiet() (*Manifest, error) {
	return loadManifestAt(false)
}

func loadManifestAt(announce bool) (*Manifest, error) {
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
		if announce {
			fmt.Fprintf(os.Stderr, "ditto: project root %s\n", root)
		}
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
