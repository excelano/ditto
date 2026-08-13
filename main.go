package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
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
  ditto status          Report where the project stands, changing nothing.
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

Exit codes:
  0  success, including a build with nothing out of date
  1  the project is wrong -- a missing source, a failed converter, a
     manifest problem 'check' reports
  2  the command is wrong -- unknown command or flag, missing argument
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "ditto: no command given; run 'ditto --help' for the list.")
		os.Exit(2)
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
	case "status":
		err = cmdStatus(args[1:])
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
		// A flag and a command fail for different reasons and deserve
		// different sentences: "unknown command \"--verison\"" told the caller
		// to look for a command it never meant to type.
		noun, candidates := "command", commands
		if strings.HasPrefix(args[0], "-") {
			noun, candidates = "flag", globalFlags
		}
		fmt.Fprintf(os.Stderr, "ditto: unknown %s %q\n", noun, args[0])
		if near := nearest(args[0], candidates); near != "" {
			fmt.Fprintf(os.Stderr, "Did you mean %s?\n", near)
		}
		fmt.Fprintln(os.Stderr, "Run 'ditto --help' for the list.")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ditto: "+err.Error())
		var ue *usageError
		if errors.As(err, &ue) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
