package main

import "testing"

func TestShellQuote(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Left bare: the common case has to stay readable, or the echoed line is
		// noisier than the two paths it replaced.
		{"plain word", "dist", "dist"},
		{"flag", "--dry-run", "--dry-run"},
		{"absolute path", "/home/anderix/dist/D3", "/home/anderix/dist/D3"},
		{"url without spaces", "https://excelano.sharepoint.com/apt", "https://excelano.sharepoint.com/apt"},
		{"percent-encoded segment", "Shared%20Documents", "Shared%20Documents"},

		// Quoted: each of these would change meaning or split if pasted bare.
		{"library url with space", "https://x/Shared Documents/y", "'https://x/Shared Documents/y'"},
		{"share link query", "https://x/:f:/r/sites/A?csf=1&web=1", "'https://x/:f:/r/sites/A?csf=1&web=1'"},
		{"dollar sign", "/tmp/$HOME/out", "'/tmp/$HOME/out'"},
		{"backtick", "/tmp/`whoami`", "'/tmp/`whoami`'"},
		// An empty argument must survive as a visible '' rather than vanishing
		// and silently shifting every argument after it.
		{"empty", "", "''"},
		// POSIX has no escape inside single quotes; you close, emit an escaped
		// quote, and reopen.
		{"embedded single quote", "it's", `'it'\''s'`},
	}
	for _, c := range cases {
		if got := shellQuote(c.in); got != c.want {
			t.Errorf("%s: shellQuote(%q) = %s, want %s", c.name, c.in, got, c.want)
		}
	}
}

func TestShellCommand(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		args []string
		want string
	}{
		{"no arguments", "xsync", nil, "xsync"},
		{
			"flags then operands",
			"xsync",
			[]string{"--dry-run", "dist", "https://x/Shared Documents/y"},
			"xsync --dry-run dist 'https://x/Shared Documents/y'",
		},
		{
			"delete run",
			"xsync",
			[]string{"--dry-run", "--delete", "/abs/dist/D3", "https://x/lib/D3"},
			"xsync --dry-run --delete /abs/dist/D3 https://x/lib/D3",
		},
	}
	for _, c := range cases {
		if got := shellCommand(c.cmd, c.args); got != c.want {
			t.Errorf("%s: shellCommand = %q, want %q", c.name, got, c.want)
		}
	}
}
