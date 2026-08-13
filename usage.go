package main

import (
	"fmt"
	"strings"
)

// usageError marks a failure that is the caller's fault at the argument level
// — a missing argument, an unknown flag — rather than something wrong with the
// project. It exits 2 so a program driving ditto can tell "you typed it wrong"
// (fix the command) from "the build failed" (fix the sources), which the single
// exit 1 used to conflate.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usagef(format string, a ...any) error {
	return &usageError{msg: fmt.Sprintf(format, a...)}
}

// commands are the verbs main dispatches on, in the order the usage block lists
// them. Kept here so a did-you-mean has something to search.
var commands = []string{"new", "init", "status", "check", "build", "scan", "clean", "publish"}

// globalFlags are the flags main answers itself, before any command runs.
var globalFlags = []string{"--help", "--version", "--install-skill", "--uninstall-skill"}

// nearest names the candidate closest to word, or "" if none is within two
// edits. Two catches the realistic typo — a transposition (`buidl`), a dropped
// letter (`publsh`) — without inventing a suggestion for a word that was never
// meant to be a command.
func nearest(word string, candidates []string) string {
	word = strings.TrimLeft(word, "-")
	best, bestDist := "", 3
	for _, c := range candidates {
		if d := editDistance(word, strings.TrimLeft(c, "-")); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

// editDistance is Levenshtein over bytes. Commands and flag names are ASCII, so
// byte distance and rune distance are the same thing here.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, min(curr[j-1]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}
