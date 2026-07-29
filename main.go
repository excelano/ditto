package main

import (
	"fmt"
	"os"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `ditto - build text sources into Microsoft Office and HTML deliverables.

The text in src/ is the master you author and version. ditto presses it into
the Office and HTML deliverables under dist/, then optionally mirrors dist/ to a
publish root. The manifest is authoritative: if a file is not listed, it is not
built.

Usage:
  ditto new <name>      Scaffold a new project (src/, dist/, Ditto.toml).
  ditto init            Scaffold a project around the files already here.
  ditto check           Validate the manifest without building anything.
  ditto build           Build every target whose output is out of date.
  ditto build <prefix>  Build only targets whose output falls under <prefix>/.
  ditto build --force   Rebuild every target, up to date or not.
  ditto scan            Report files in src/ that no target covers.
  ditto scan --write    Append a target for each uncovered file, using defaults.
  ditto clean           Remove the built deliverables in dist/.
  ditto clean -n        List what clean would remove, without removing it.
  ditto publish         Mirror dist/ to the publish root (SharePoint or local).
  ditto publish <prefix>  Mirror only dist/<prefix>/ to <root>/<prefix>/.
  ditto publish -n      Preview the publish without writing anything.
  ditto publish --delete  Also remove root files no longer in dist/.

Flags:
  -h, --help       Show this help.
  -V, --version    Show the version.
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	var err error
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Print(usage)
	case "-V", "--version", "version":
		fmt.Printf("ditto %s\n", version)
	case "new":
		err = cmdNew(args[1:])
	case "init":
		err = cmdInit(args[1:])
	case "check":
		err = cmdCheck(args[1:])
	case "build":
		err = cmdBuild(args[1:])
	case "scan":
		err = cmdScan(args[1:])
	case "clean":
		err = cmdClean(args[1:])
	case "publish":
		err = cmdPublish(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "ditto: unknown command %q\n\n%s", args[0], usage)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ditto: "+err.Error())
		os.Exit(1)
	}
}
