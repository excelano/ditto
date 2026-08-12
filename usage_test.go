package main

import (
	"errors"
	"testing"
)

// isUsage is the classification main makes to pick exit 2 over exit 1.
func isUsage(err error) bool {
	var ue *usageError
	return errors.As(err, &ue)
}

func TestArgumentErrorsAreUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "unknown argument", err: unknownArg("build", "--nope")},
		{name: "new without a name", err: cmdNew(nil)},
		{name: "new with a stray flag", err: cmdNew([]string{"proj", "--nope"})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatal("expected an error")
			}
			if !isUsage(tc.err) {
				t.Errorf("%v should exit 2, not 1", tc.err)
			}
		})
	}
}

// The counterpart: a project that is wrong is not a command that is wrong, and
// the two must not collapse back onto one exit code.
func TestProjectErrorsAreNotUsageErrors(t *testing.T) {
	err := errors.New("input not found: src/missing.md")
	if isUsage(err) {
		t.Error("a plain error should exit 1")
	}
}

func TestNearest(t *testing.T) {
	cases := []struct {
		word       string
		candidates []string
		want       string
	}{
		{word: "buidl", candidates: commands, want: "build"},
		{word: "publsh", candidates: commands, want: "publish"},
		{word: "chek", candidates: commands, want: "check"},
		{word: "--verison", candidates: globalFlags, want: "--version"},
		{word: "--instal-skill", candidates: globalFlags, want: "--install-skill"},
		// Far enough away that a suggestion would be noise, not help.
		{word: "nonsenseword", candidates: commands, want: ""},
		{word: "--json", candidates: globalFlags, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.word, func(t *testing.T) {
			if got := nearest(tc.word, tc.candidates); got != tc.want {
				t.Errorf("nearest(%q) = %q, want %q", tc.word, got, tc.want)
			}
		})
	}
}

// Every command the usage block advertises must be one main will dispatch on,
// or a did-you-mean would point at a command that does not exist.
func TestEveryNamedCommandHasHelp(t *testing.T) {
	for _, c := range commands {
		if _, ok := commandUsage[c]; !ok {
			t.Errorf("command %q has no help text", c)
		}
	}
}
