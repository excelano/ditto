#!/bin/sh
# ditto installer — fetches the latest release binary for the host platform and
# drops it into /usr/local/bin (or ~/.local/bin if /usr/local/bin is not
# writable). POSIX sh, no bash extensions.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/excelano/ditto/main/install.sh | sh
#
# On Debian/Ubuntu, prefer the apt route (auto-updates with the system):
#   curl -fsSL https://excelano.com/apt/setup.sh | sudo sh
#   sudo apt install ditto
#
# Environment variables:
#   DITTO_INSTALL_DIR   Override install directory (e.g. /opt/bin or $HOME/bin)
#   DITTO_VERSION       Install a specific release tag (e.g. v0.1.0) instead of latest

set -eu

REPO="excelano/ditto"

say() { printf '%s\n' "$*" >&2; }
err() { say "error: $*"; exit 1; }

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		err "this installer needs '$1' on PATH; please install it and re-run"
	fi
}

need_cmd curl
need_cmd tar
need_cmd uname

detect_platform() {
	OS=$(uname -s | tr '[:upper:]' '[:lower:]')
	ARCH=$(uname -m)
	# ditto ships linux binaries only. macOS already has its own /usr/bin/ditto,
	# so a same-named binary would collide; build from source there instead and
	# choose a name that does not clash.
	case "$OS" in
		linux) ;;
		darwin) err "ditto ships linux binaries only; on macOS build from source
       (and pick a name other than 'ditto' — the OS ships /usr/bin/ditto):
       git clone https://github.com/${REPO} && cd ditto && go build ." ;;
		*) err "unsupported OS: $OS (ditto ships linux binaries)";;
	esac
	case "$ARCH" in
		x86_64|amd64) ARCH=amd64 ;;
		aarch64|arm64) ARCH=arm64 ;;
		*) err "unsupported architecture: $ARCH";;
	esac
	PLATFORM="${OS}_${ARCH}"
}

resolve_version() {
	if [ -n "${DITTO_VERSION:-}" ]; then
		VERSION="$DITTO_VERSION"
		say "Installing ditto $VERSION (pinned via DITTO_VERSION)"
		return
	fi
	# Resolve the latest tag via the GitHub API. The web /releases/latest
	# redirect is edge-cached for several minutes after a new release, which
	# makes a re-run right after tagging silently install the previous version.
	# The API is real-time. Anonymous calls are rate-limited to 60/hour per IP,
	# which is fine for a human-run installer.
	VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
		| awk -F'"' '/"tag_name":/ { print $4; exit }')
	if [ -z "${VERSION:-}" ]; then
		err "could not resolve latest release tag from GitHub"
	fi
	say "Installing ditto $VERSION (latest)"
}

detect_existing() {
	EXISTING_PATH=""
	EXISTING_DIR=""
	if command -v ditto >/dev/null 2>&1; then
		EXISTING_PATH=$(command -v ditto)
		EXISTING_DIR=$(dirname "$EXISTING_PATH")
	fi
}

pick_install_dir() {
	if [ -n "${DITTO_INSTALL_DIR:-}" ]; then
		INSTALL_DIR="$DITTO_INSTALL_DIR"
	elif [ -n "$EXISTING_DIR" ]; then
		# An existing install wins over the default — upgrade in place rather
		# than scattering a second copy into a directory earlier on PATH.
		INSTALL_DIR="$EXISTING_DIR"
		say "Existing install at $EXISTING_PATH — upgrading in place"
	elif [ -w /usr/local/bin ] 2>/dev/null; then
		INSTALL_DIR=/usr/local/bin
	else
		# /usr/local/bin needs root; fall back to a user-writable spot.
		INSTALL_DIR="$HOME/.local/bin"
	fi
	mkdir -p "$INSTALL_DIR" || err "cannot create install dir $INSTALL_DIR"
	# Many users land here because they tried `sudo curl ... | sh`, which only
	# sudoes curl, not the sh that writes the binary. Give them the literal
	# correct command (sudo wraps sh, not curl).
	if [ ! -w "$INSTALL_DIR" ]; then
		err "$INSTALL_DIR is not writable; either set DITTO_INSTALL_DIR to a
       writable directory, or re-run as
       curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sudo sh"
	fi
	if [ -n "$EXISTING_DIR" ] && [ "$EXISTING_DIR" != "$INSTALL_DIR" ]; then
		say "Warning: ditto already installed at $EXISTING_PATH"
		say "         New copy will land at $INSTALL_DIR/ditto"
		say "         You will have two copies; PATH order decides which runs"
	fi
}

download_and_install() {
	VERSION_NUM=${VERSION#v}
	ARCHIVE="ditto_${VERSION_NUM}_${PLATFORM}.tar.gz"
	URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
	CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

	TMPDIR=$(mktemp -d)
	trap 'rm -rf "$TMPDIR"' EXIT INT TERM

	say "Downloading $ARCHIVE"
	if ! curl -fsSL -o "$TMPDIR/$ARCHIVE" "$URL"; then
		err "download failed: $URL"
	fi

	say "Verifying checksum"
	if ! curl -fsSL -o "$TMPDIR/checksums.txt" "$CHECKSUMS_URL"; then
		err "could not fetch checksums.txt from release"
	fi
	EXPECTED=$(awk -v a="$ARCHIVE" '$2==a {print $1}' "$TMPDIR/checksums.txt")
	if [ -z "$EXPECTED" ]; then
		err "checksums.txt has no entry for $ARCHIVE"
	fi
	if command -v sha256sum >/dev/null 2>&1; then
		ACTUAL=$(sha256sum "$TMPDIR/$ARCHIVE" | awk '{print $1}')
	elif command -v shasum >/dev/null 2>&1; then
		ACTUAL=$(shasum -a 256 "$TMPDIR/$ARCHIVE" | awk '{print $1}')
	else
		err "need sha256sum or shasum to verify the download"
	fi
	if [ "$EXPECTED" != "$ACTUAL" ]; then
		err "checksum mismatch: expected $EXPECTED, got $ACTUAL"
	fi

	say "Extracting to $INSTALL_DIR"
	tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR" ditto
	# install(1) handles permissions and atomicity better than mv on most systems.
	if command -v install >/dev/null 2>&1; then
		install -m 0755 "$TMPDIR/ditto" "$INSTALL_DIR/ditto"
	else
		mv "$TMPDIR/ditto" "$INSTALL_DIR/ditto"
		chmod 0755 "$INSTALL_DIR/ditto"
	fi
}

# ditto reimplements nothing — it conducts converters that are separate installs.
# Warn (non-fatally) about the ones that are missing so a fresh install does not
# silently fail on the first build.
check_converters() {
	missing=""
	command -v pandoc  >/dev/null 2>&1 || missing="$missing pandoc(docx/pptx)"
	command -v md2docx >/dev/null 2>&1 || missing="$missing office-convert(md2docx/md2pptx/csv2xlsx)"
	command -v cleave  >/dev/null 2>&1 || missing="$missing cleave(html)"
	command -v xsync   >/dev/null 2>&1 || missing="$missing xsync(publish)"
	if [ -n "$missing" ]; then
		say ""
		say "Note: these converters ditto calls are not on PATH:$missing"
		say "      Install whichever you need — ditto only orchestrates them."
	fi
}

post_install_message() {
	say ""
	say "ditto installed to $INSTALL_DIR/ditto"
	case ":$PATH:" in
		*":$INSTALL_DIR:"*) ;;
		*) say "Note: $INSTALL_DIR is not on your PATH. Add it to your shell rc:"
		   say "    export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
	esac
	say ""
	say "Try it:"
	say "    ditto --help"
}

detect_platform
detect_existing
resolve_version
pick_install_dir
download_and_install
check_converters
post_install_message
