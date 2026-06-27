package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

const (
	manifestName = "Manifest.toml"
	srcDir       = "src"
	distDir      = "dist"
)

// Manifest is the authoritative description of a ditto project: nothing is
// built that is not listed here.
type Manifest struct {
	Project Project  `toml:"project"`
	Publish *Publish `toml:"publish"` // optional; nil when the section is absent
	Targets []Target `toml:"target"`
}

type Project struct {
	Name string `toml:"name"`
}

// Publish names where `ditto publish` mirrors dist/. Root is a SharePoint
// library URL or a local path.
type Publish struct {
	Root string `toml:"root"`
}

// Target builds one file from src/ into one file under dist/. The output
// extension selects the converter unless Converter overrides it.
type Target struct {
	Input     string `toml:"input"`     // relative to src/
	Output    string `toml:"output"`    // relative to dist/
	Reference string `toml:"reference"` // optional styling template
	View      string `toml:"view"`      // optional (cleave/html only)
	Converter string `toml:"converter"` // optional override
}

func loadManifest() (*Manifest, error) {
	if _, err := os.Stat(manifestName); err != nil {
		return nil, fmt.Errorf("no %s in the current directory", manifestName)
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
