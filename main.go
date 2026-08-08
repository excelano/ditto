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
  ditto scan            Report files in src/ that no target covers.
  ditto clean           Remove the built deliverables in dist/.
  ditto publish         Mirror dist/ to the publish root (SharePoint or local).

Run 'ditto <command> --help' for the arguments and flags of one command.

Flags:
  -h, --help       Show this help.
  -V, --version    Show the version.

Claude Code:
  --install-skill      Install the ditto skill into ~/.claude/skills/ditto.
  --uninstall-skill    Remove it again.
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	// Help is answered here rather than inside each command, so that asking a
	// command to explain itself never reaches its argument parsing.
	if u, ok := commandUsage[args[0]]; ok && wantsHelp(args[1:]) {
		fmt.Print(u)
		return
	}

	var err error
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Print(usage)
	case "-V", "--version", "version":
		fmt.Printf("ditto %s\n", resolveVersion())
	// Terminal actions: they touch the user's skills directory and nothing
	// else, so no manifest is read on the way through.
	case "--install-skill":
		os.Exit(installSkill(resolveVersion()))
	case "--uninstall-skill":
		os.Exit(uninstallSkill())
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
