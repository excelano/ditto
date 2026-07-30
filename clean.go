package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// cmdClean removes the built deliverables. Nothing else does: build only ever
// writes, so an output renamed in the manifest leaves the old file behind in
// dist/, and `publish` without --delete then ships both the current deliverable
// and the stale one. Cleaning and rebuilding leaves exactly what the manifest
// describes.
//
// It takes the recorded fingerprints with it. They describe outputs that are
// about to stop existing, and clearing them is the way to force the build to
// distrust its own state — the reason the count reported below covers only
// dist/ is that the state directory is ditto's bookkeeping and not anything the
// user put there.
func cmdClean(args []string) error {
	dryRun := false
	for _, a := range args {
		switch a {
		case "-n", "--dry-run":
			dryRun = true
		default:
			return unknownArg("clean", a)
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
	absDist, err := filepath.Abs(dist)
	if err != nil {
		return err
	}
	if err := refuseUnsafeClean(absDist); err != nil {
		return err
	}

	if _, err := os.Stat(absDist); err != nil {
		// State can outlive the outputs it describes — a hand-deleted dist/,
		// or a dist moved in the manifest — so clear it even here.
		if !dryRun {
			if err := os.RemoveAll(stateDir); err != nil {
				return err
			}
		}
		fmt.Printf("Nothing to clean: no %s/\n", dist)
		return nil
	}
	files, err := countFiles(absDist)
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Printf("Would remove %s/ (%d file(s)).\n", dist, files)
		return nil
	}
	if err := os.RemoveAll(absDist); err != nil {
		return err
	}
	if err := os.RemoveAll(stateDir); err != nil {
		return err
	}
	fmt.Printf("Removed %s/ (%d file(s)).\n", dist, files)
	return nil
}

// refuseUnsafeClean rejects a dist directory that clean must not delete. dist
// is configurable and ~-expanded, so a typo in the manifest can aim a recursive
// remove at something that matters. resolveDist already refuses a dist inside
// src/; these are the cases where the damage would be outside the project.
func refuseUnsafeClean(absDist string) error {
	if absDist == string(os.PathSeparator) {
		return fmt.Errorf("dist resolves to the filesystem root; refusing to clean it")
	}
	if home, err := os.UserHomeDir(); err == nil {
		if absHome, err := filepath.Abs(home); err == nil && absHome == absDist {
			return fmt.Errorf("dist resolves to the home directory; refusing to clean it")
		}
	}
	// loadManifest has already moved us to the project root.
	if root, err := os.Getwd(); err == nil {
		if root == absDist || strings.HasPrefix(root, absDist+string(os.PathSeparator)) {
			return fmt.Errorf("dist %s contains the project itself; refusing to clean it", absDist)
		}
	}
	return nil
}

// countFiles reports how many regular files a tree holds, so clean can say what
// it removed rather than only that it removed something.
func countFiles(dir string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	return n, err
}
