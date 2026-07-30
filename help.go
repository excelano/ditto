package main

import "fmt"

// Per-command help. main dispatches on these before the command runs, so every
// command answers -h and --help identically and none can forget to: the
// alternative is each command parsing its own help flag, which is how `ditto
// new --help` once created a directory called "--help".
var commandUsage = map[string]string{
	"new":     newUsage,
	"init":    initUsage,
	"check":   checkUsage,
	"build":   buildUsage,
	"scan":    scanUsage,
	"clean":   cleanUsage,
	"publish": publishUsage,
}

const helpFlags = `  -h, --help       Show this help.
`

const newUsage = `ditto new - scaffold a new project.

Usage:
  ditto new <name>

Creates <name>/ holding src/ for the text you author, dist/ for the built
deliverables, and a starter ` + manifestName + `. Use 'ditto init' instead to
scaffold around files that are already here.

Flags:
` + helpFlags

const initUsage = `ditto init - scaffold a project around the files already here.

Usage:
  ditto init

'new' without the new directory, for a folder of sources that predates the
decision to build it with ditto. Every write is additive: src/ keeps whatever
it already holds and an existing .gitignore is appended to, not replaced.

Flags:
` + helpFlags

const checkUsage = `ditto check - validate the manifest without building anything.

Usage:
  ditto check

Reports what build would otherwise hit partway through, after some targets had
already written files: a missing source, a reference that moved, a converter
that is not installed on this machine. The cheap pass to run after hand-editing
the manifest, and before handing a project to someone whose machine may not
have the same tools.

Flags:
` + helpFlags

const buildUsage = `ditto build - build every target whose output is out of date.

Usage:
  ditto build [<prefix>] [flags]

A target rebuilds when its output is older than its inputs, its styling
reference, or its converter script, and when its own entry in the manifest has
changed since it was last built. Editing or adding one target therefore leaves
the others alone. Give a <prefix> to build only targets whose output falls
under that directory.

Converters resolved from $PATH are not tracked, so upgrading pandoc or
office-convert calls for --force once.

Flags:
  -f, --force      Rebuild every target, up to date or not.
  -n, --dry-run    List what would be rebuilt, without converting anything.
` + helpFlags

const scanUsage = `ditto scan - report files in src/ that no target covers.

Usage:
  ditto scan [flags]

With no flag scan only reports. --write appends a [[target]] for each uncovered
file using the unambiguous defaults (.md -> .docx, .csv -> .xlsx); anything
else is left for you to declare, and every appended output is worth reading
before building.

Flags:
      --write      Append a target for each uncovered file, using defaults.
` + helpFlags

const cleanUsage = `ditto clean - remove the built deliverables in dist/.

Usage:
  ditto clean [flags]

Nothing else removes them: build only ever writes, so an output renamed in the
manifest leaves the old file behind, and publish without --delete then ships
both the current deliverable and the stale one. Cleaning and rebuilding leaves
exactly what the manifest describes.

It also removes .ditto/, where build records what it has already made, so the
next build reconverts the whole set from source.

Flags:
  -n, --dry-run    List what clean would remove, without removing it.
` + helpFlags

const publishUsage = `ditto publish - mirror dist/ to the publish root.

Usage:
  ditto publish [<prefix>] [flags]

Sends the built deliverables to the [publish] root in the manifest, which is
either a SharePoint library URL (mirrored with xsync) or a local path. Give a
<prefix> to mirror only dist/<prefix>/ to <root>/<prefix>/.

Flags:
  -n, --dry-run    Preview the publish without writing anything.
      --delete     Also remove root files no longer in dist/.
` + helpFlags

// wantsHelp reports whether the arguments ask for help. It scans all of them so
// that a half-typed command still explains itself: `ditto publish D3 --help`
// asks a question rather than starting a mirror.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

// unknownArg is the one voice every command uses to reject an argument, and it
// points at the help that lists the real ones.
func unknownArg(cmd, arg string) error {
	return fmt.Errorf("%s: unknown argument %q; try 'ditto %s --help'", cmd, arg, cmd)
}
