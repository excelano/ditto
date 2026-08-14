package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Workspace is a root that owns no deliverables of its own and runs a verb
// across several member projects with one command — a client engagement whose
// phases are authored separately and shipped together.
//
// The root is deliberately a *pure* workspace: a manifest may declare
// [workspace] or [[target]], never both. Cargo allows a root package alongside
// its workspace, and the cost is that every command needs a rule about which
// one it means. An engagement folder has no deliverables of its own, so the
// rule would earn nothing here but the ambiguity.
//
// Members are not given shared [project] defaults, which is the one open
// question from the issue answered with "not yet". Inheritance would mean
// either that every member load walks up looking for a workspace, or that the
// same member builds differently depending on which directory you are standing
// in. The second is a trap; the first is a real change to how every verb finds
// a project, and it buys one repeated line per member. Worth doing when the
// repetition actually hurts, and cheap to add then.
type Workspace struct {
	Members []string `toml:"members"`
}

// workspaceAt reads the manifest at root and returns its workspace, or nil if
// that manifest describes an ordinary project.
//
// It parses the file a second time rather than threading a manifest through
// the dispatch: the verbs each load their own, the file is small, and the
// alternative is every command carrying an argument it does not use. A parse
// failure returns nil so the verb reports it — this is not the place to
// diagnose a broken manifest.
func workspaceAt(root string) *Workspace {
	var m Manifest
	if _, err := toml.DecodeFile(filepath.Join(root, manifestName), &m); err != nil {
		return nil
	}
	return m.Workspace
}

// validate rejects the shapes that cannot mean anything.
func (w *Workspace) validate(m *Manifest) error {
	if len(m.Targets) > 0 {
		return fmt.Errorf("%s declares both [workspace] and [[target]]: a workspace root builds its members, so move those targets into a member", manifestName)
	}
	if len(w.Members) == 0 {
		return fmt.Errorf("[workspace] lists no members; add members = [\"phase-1\", ...]")
	}
	seen := map[string]bool{}
	for _, member := range w.Members {
		if seen[member] {
			return fmt.Errorf("member %q is listed twice", member)
		}
		seen[member] = true
	}
	return nil
}

// selectMembers resolves which members a run covers, and returns the arguments
// with the selector removed.
//
// The selector has to be the first argument. That is what keeps it apart from
// a flag's value — `ditto build --profile draft` at a workspace root would
// otherwise have to guess whether "draft" names a member — and it costs
// nothing, since nobody writes the filter after the flags anyway.
func (w *Workspace) selectMembers(args []string) ([]string, []string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return w.Members, args, nil
	}
	name := args[0]
	for _, member := range w.Members {
		if member == name {
			return []string{member}, args[1:], nil
		}
	}
	return nil, nil, usagef("no member %q; [workspace] lists: %s", name, strings.Join(w.Members, ", "))
}

// runAcrossMembers runs one verb in each member, from the workspace root.
//
// Every member runs even after one fails, and the count is reported at the
// end. Stopping at the first failure would report a broken phase-1 and say
// nothing about phase-2 and closeout, which is the opposite of what running a
// verb across the whole engagement was for.
func runAcrossMembers(name string, w *Workspace, root string, args []string, run func([]string) error) error {
	members, rest, err := w.selectMembers(args)
	if err != nil {
		return err
	}
	failed := 0
	for i, member := range members {
		dir := filepath.Join(root, member)
		if _, err := os.Stat(filepath.Join(dir, manifestName)); err != nil {
			fmt.Fprintf(os.Stderr, "ditto: member %s has no %s\n", member, manifestName)
			failed++
			continue
		}
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("entering member %s: %w", member, err)
		}
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("== %s ==\n", member)
		if err := run(rest); err != nil {
			fmt.Fprintf(os.Stderr, "ditto: %s: %v\n", member, err)
			failed++
		}
		if err := os.Chdir(root); err != nil {
			return fmt.Errorf("returning to workspace root: %w", err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%s failed in %d of %d member(s)", name, failed, len(members))
	}
	return nil
}

// workspaceAware wraps a verb so it fans out when it is run at a workspace
// root and behaves exactly as before anywhere else — including inside a
// member, where findProjectRoot reaches that member's own manifest first.
//
// That is also the answer to filtering: `ditto build phase-2` builds one
// member from the root, and `cd phase-2 && ditto build D3` filters inside it,
// so the member selector and the prefix filter never have to share a position.
func workspaceAware(name string, args []string, run func([]string) error) error {
	root, err := findProjectRoot()
	if err != nil {
		return run(args) // no project here; let the verb say so in its own words
	}
	w := workspaceAt(root)
	if w == nil {
		return run(args)
	}
	var m Manifest
	if _, err := toml.DecodeFile(filepath.Join(root, manifestName), &m); err == nil {
		if err := w.validate(&m); err != nil {
			return err
		}
	}
	if err := os.Chdir(root); err != nil {
		return fmt.Errorf("entering workspace root %s: %w", root, err)
	}
	return runAcrossMembers(name, w, root, args, run)
}
