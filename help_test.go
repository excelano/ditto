package main

import (
	"os"
	"strings"
	"testing"
)

// Every command main dispatches on. A command added to main without an entry
// here and in commandUsage would answer --help by rejecting it as an unknown
// argument, which is the state this whole file exists to prevent.
var dispatched = []string{"new", "init", "status", "check", "build", "scan", "clean", "publish"}

func TestEveryCommandHasHelp(t *testing.T) {
	for _, cmd := range dispatched {
		usage, ok := commandUsage[cmd]
		if !ok {
			t.Errorf("%s has no help text", cmd)
			continue
		}
		if !strings.HasPrefix(usage, "ditto "+cmd+" - ") {
			t.Errorf("%s help should open with 'ditto %s - ', got %q", cmd, cmd, firstLine(usage))
		}
		if !strings.Contains(usage, "-h, --help") {
			t.Errorf("%s help does not list -h, --help", cmd)
		}
		if !strings.HasSuffix(usage, "\n") {
			t.Errorf("%s help should end with a newline", cmd)
		}
	}
	for cmd := range commandUsage {
		if !contains(dispatched, cmd) {
			t.Errorf("help text for %q, which main does not dispatch", cmd)
		}
	}
}

func TestWantsHelp(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"-h"}, true},
		{[]string{"--help"}, true},
		{[]string{"D3", "--help"}, true}, // a half-typed command still explains itself
		{[]string{"--force"}, false},
		{[]string{"help"}, false}, // a positional named help is a prefix, not a flag
	}
	for _, c := range cases {
		if got := wantsHelp(c.args); got != c.want {
			t.Errorf("wantsHelp(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestUnknownArgPointsAtHelp(t *testing.T) {
	err := unknownArg("build", "--dryrun")
	want := `build: unknown argument "--dryrun"; try 'ditto build --help'`
	if err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

// cmdNew takes its name as a positional, so a flag reaching the name is how
// `ditto new --help` once created a directory called "--help".
func TestNewRejectsFlagsRatherThanScaffoldingThem(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, flag := range []string{"-h", "--help", "--force"} {
		if err := cmdNew([]string{flag}); err == nil {
			t.Errorf("cmdNew(%q) should fail, not scaffold", flag)
		}
		if _, err := os.Stat(flag); err == nil {
			t.Errorf("cmdNew(%q) created a directory named %q", flag, flag)
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
