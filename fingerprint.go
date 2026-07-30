package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	// stateDir holds what the build knows about itself and nothing a person
	// wrote. It sits at the project root rather than inside dist/ because
	// `publish` mirrors dist/ verbatim to a client's SharePoint library — dist/
	// is the payload, so a cache kept there would ship with the deliverables.
	stateDir        = ".ditto"
	fingerprintFile = "fingerprints.json"
)

// fingerprints maps a target's output path to a hash of the manifest entry that
// produced it, recorded when that output was last built.
//
// It exists so that editing the manifest rebuilds the targets that changed
// rather than all of them. Timestamps cannot answer that question: one mtime on
// Ditto.toml says the file was edited, not which of its fifty targets the edit
// touched, so the only safe reading of a bare mtime is that everything is
// stale. Adding one target then reconverts an engagement's worth of documents.
//
// This is the one piece of state ditto keeps, and it is deliberately weak: it
// can withhold freshness but never grant it. Every mtime comparison still has
// to pass on its own, so a missing, truncated, or hand-mangled file costs one
// unnecessary rebuild and never a stale deliverable. That is what makes it safe
// to throw away, which `ditto clean` does.
type fingerprints map[string]string

func fingerprintPath() string { return filepath.Join(stateDir, fingerprintFile) }

// loadFingerprints reads what the last build recorded. Any problem reading it
// yields an empty set, which marks every target stale — the conservative
// direction, and the reason no error is returned: there is nothing for a caller
// to do about it that rebuilding does not already do.
func loadFingerprints() fingerprints {
	body, err := os.ReadFile(fingerprintPath())
	if err != nil {
		return fingerprints{}
	}
	fps := fingerprints{}
	if err := json.Unmarshal(body, &fps); err != nil {
		return fingerprints{}
	}
	return fps
}

// matches reports whether a target's manifest entry is the one its output was
// built from. An entry that has never been recorded does not match, so the
// first build after upgrading from a version without this file rebuilds
// everything once.
func (f fingerprints) matches(t Target) bool {
	fp := targetFingerprint(t)
	if fp == "" {
		return false
	}
	recorded, ok := f[fingerprintKey(t)]
	return ok && recorded == fp
}

// record notes the entry a target was just built from. Targets skipped as fresh
// keep the entry they already have, and targets outside a prefix filter are
// never visited, so a filtered build must not write a set built from scratch.
func (f fingerprints) record(t Target) {
	if fp := targetFingerprint(t); fp != "" {
		f[fingerprintKey(t)] = fp
	}
}

// save writes the set through a temporary file and a rename, so an interrupted
// write leaves the previous state rather than a half-written file that the next
// build would discard.
func (f fingerprints) save() error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(stateDir, "fingerprints-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once the rename below succeeds
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), fingerprintPath())
}

// fingerprintKey identifies a target by the file it produces, which is the one
// field the manifest already guarantees is unique: checkDuplicateOutputs
// refuses a build where two targets write the same output.
func fingerprintKey(t Target) string { return filepath.ToSlash(t.Output) }

// targetFingerprint hashes everything about a target that can change what the
// build produces from unchanged sources.
//
// It hashes the encoding of the whole Target rather than a hand-listed set of
// fields on purpose. A list has to be revisited every time the manifest grows a
// field, and forgetting is silent in the worst direction: the new field would
// change what the converter does while the fingerprint stayed identical, and
// the target would never rebuild. Hashing the struct means a field is covered
// the moment it exists, and TestFingerprintCoversEveryTargetField holds that.
//
// The Reference here is the effective one — cmdBuild resolves the project-level
// default before the target reaches this point — so changing
// default_reference_docx restyles every .docx target that relies on it.
func targetFingerprint(t Target) string {
	body, err := json.Marshal(t)
	if err != nil {
		// A Target is strings and string slices, so this is unreachable; an
		// empty hash matches nothing, keeping the target stale if it is not.
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
