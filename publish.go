package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func cmdPublish(args []string) error {
	dryRun, del := false, false
	var prefix string
	for _, a := range args {
		switch a {
		case "-n", "--dry-run":
			dryRun = true
		case "--delete":
			del = true
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf("publish: unknown argument %q", a)
			}
			if prefix != "" {
				return fmt.Errorf("publish takes at most one filter argument")
			}
			prefix = a
		}
	}

	m, err := loadManifest()
	if err != nil {
		return err
	}
	if m.Publish == nil || strings.TrimSpace(m.Publish.Root) == "" {
		return fmt.Errorf("no [publish] root in %s; nothing to publish to", manifestName)
	}

	// A prefix narrows both ends: only dist/<prefix> is mirrored, and only to
	// the matching <root>/<prefix> subfolder.
	distRoot := distDir
	if prefix != "" {
		distRoot = filepath.Join(distDir, filepath.FromSlash(prefix))
	}
	if _, err := os.Stat(distRoot); err != nil {
		hint := "ditto build"
		if prefix != "" {
			hint += " " + prefix
		}
		return fmt.Errorf("no %s/ to publish; run '%s' first", distRoot, hint)
	}

	root := m.Publish.Root
	if isRemote(root) {
		dst := root
		if prefix != "" {
			dst = strings.TrimRight(root, "/") + "/" + filepath.ToSlash(prefix)
		}
		fmt.Printf("Publishing %s/ -> %s\n", distRoot, dst)
		return publishRemote(distRoot, dst, dryRun, del)
	}
	dest := expandHome(root)
	if prefix != "" {
		dest = filepath.Join(dest, filepath.FromSlash(prefix))
	}
	fmt.Printf("Publishing %s/ -> %s\n", distRoot, dest)
	return publishLocal(distRoot, dest, dryRun, del)
}

func isRemote(root string) bool {
	return strings.HasPrefix(root, "https://") || strings.HasPrefix(root, "http://")
}

// publishRemote mirrors a local directory to a SharePoint library with xsync.
// xsync owns the Graph plumbing, incremental upload, and the metadata-identity
// quirks, so ditto just hands it the source directory and the destination URL.
func publishRemote(src, dst string, dryRun, del bool) error {
	if _, err := exec.LookPath("xsync"); err != nil {
		return fmt.Errorf("xsync not found on PATH; install xfiles to publish to SharePoint")
	}
	// xsync parses with the flag package, which stops at the first positional,
	// so any flags must precede the src/dst arguments.
	var xargs []string
	if dryRun {
		xargs = append(xargs, "--dry-run")
	}
	if del {
		xargs = append(xargs, "--delete")
	}
	xargs = append(xargs, src, dst)
	cmd := exec.Command("xsync", xargs...)
	cmd.Stdin = os.Stdin // xsync prompts to confirm deletes on a TTY
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// publishLocal mirrors src into a local folder. A plain overwrite copy is
// enough for a local root; --delete prunes files the destination holds that src
// no longer does.
func publishLocal(src, dest string, dryRun, del bool) error {
	copied := 0
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			if dryRun {
				return nil
			}
			return os.MkdirAll(target, 0o755)
		}
		copied++
		if dryRun {
			fmt.Printf("  copy %s\n", rel)
			return nil
		}
		if err := copyFile(path, target); err != nil {
			return err
		}
		fmt.Printf("  %s\n", rel)
		return nil
	})
	if err != nil {
		return err
	}

	removed := 0
	if del {
		removed, err = deleteExtraneous(src, dest, dryRun)
		if err != nil {
			return err
		}
	}

	verb := "copied"
	if dryRun {
		verb = "to copy"
	}
	fmt.Printf("Done: %d %s, %d deleted.\n", copied, verb, removed)
	return nil
}

// deleteExtraneous removes files under dest that have no counterpart in src.
func deleteExtraneous(src, dest string, dryRun bool) (int, error) {
	if _, err := os.Stat(dest); err != nil {
		return 0, nil // nothing published yet, nothing to prune
	}
	removed := 0
	err := filepath.WalkDir(dest, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		if _, statErr := os.Stat(filepath.Join(src, rel)); statErr == nil {
			return nil // still present in src
		}
		removed++
		if dryRun {
			fmt.Printf("  delete %s\n", rel)
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		fmt.Printf("  deleted %s\n", rel)
		return nil
	})
	return removed, err
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}
