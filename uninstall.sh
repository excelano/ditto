#!/bin/sh
# ditto uninstaller — finds and removes the ditto binary installed by
# install.sh (the curl-piped release installer). POSIX sh, no bash extensions.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/excelano/ditto/main/uninstall.sh | sh
#
# This removes only the binary. ditto keeps no config or state of its own, and
# it leaves the converters it orchestrates (pandoc, office-convert, cleave,
# xsync) installed. If you installed via apt, remove it with apt instead:
#   sudo apt remove ditto
#
# Environment variables:
#   DITTO_UNINSTALL_YES=1  Skip the confirmation prompt (assume yes).

set -eu

say() { printf '%s\n' "$*" >&2; }
err() { say "error: $*"; exit 1; }

# read_yes reads a y/N answer from the controlling terminal, not stdin, because
# this script is typically invoked as `curl ... | sh` where stdin is the script.
read_yes() {
	prompt="$1"
	if [ "${DITTO_UNINSTALL_YES:-0}" = "1" ]; then
		return 0
	fi
	if [ ! -t 0 ] && [ ! -e /dev/tty ]; then
		err "no terminal available for confirmation; re-run with DITTO_UNINSTALL_YES=1 to skip the prompt"
	fi
	printf '%s [y/N]: ' "$prompt" >&2
	if [ -e /dev/tty ]; then
		read ans </dev/tty
	else
		read ans
	fi
	case "$ans" in
		y|Y|yes|YES) return 0 ;;
		*) return 1 ;;
	esac
}

if ! command -v ditto >/dev/null 2>&1; then
	say "ditto is not on PATH; nothing to uninstall."
	say "If you installed to a custom location, remove it manually:"
	say "    rm /path/to/ditto"
	exit 0
fi

TARGET=$(command -v ditto)
say "Found ditto at $TARGET"

# An apt-managed binary lives under /usr/bin and should be removed with apt, not
# rm, so dpkg's database stays consistent.
case "$TARGET" in
	/usr/bin/ditto)
		err "$TARGET looks apt-managed; remove it with:  sudo apt remove ditto" ;;
esac

if [ ! -w "$TARGET" ] && [ ! -w "$(dirname "$TARGET")" ]; then
	err "$TARGET is not writable; re-run with sudo to remove it"
fi

if ! read_yes "Remove $TARGET?"; then
	say "Aborted."
	exit 1
fi

rm -f "$TARGET" || err "could not remove $TARGET"
say "Removed $TARGET"

# Invalidate the shell's command hash so a follow-up `command -v` does not
# report the just-deleted path as still present.
hash -r 2>/dev/null || true

# A second copy can exist (e.g. one in /usr/local/bin and one in ~/.local/bin).
# PATH lookup only finds the first; warn so the user knows the others remain.
LEFTOVER=$(command -v ditto 2>/dev/null || true)
if [ -n "$LEFTOVER" ]; then
	say ""
	say "Note: another ditto binary is still on PATH at $LEFTOVER"
	say "Re-run this uninstaller to remove it, or remove it manually."
fi

say ""
say "Done."
