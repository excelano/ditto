#!/bin/bash
# install.sh - Build ditto and install it into ~/bin.
#
# ditto orchestrates other tools; it does not convert anything itself. For full
# coverage install office-convert (md2docx/csv2xlsx/md2pptx), cleave (html), and
# xfiles (xsync, for publish). The install warns about whatever is missing.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

VERSION="$(git -C "$SCRIPT_DIR" describe --tags --always --dirty 2>/dev/null || echo dev)"

mkdir -p "$HOME/bin"
go build -ldflags "-X main.version=$VERSION" -o "$HOME/bin/ditto" .
echo "Installed: ~/bin/ditto ($VERSION)"

if [[ ":$PATH:" != *":$HOME/bin:"* ]]; then
    echo 'export PATH="$HOME/bin:$PATH"' >> "$HOME/.bashrc"
    echo "Added ~/bin to PATH in .bashrc (restart shell or: source ~/.bashrc)"
fi

command -v pandoc  >/dev/null || echo "Warning: pandoc not found (needed for docx/pptx)."
command -v md2docx >/dev/null || echo "Warning: office-convert not installed (md2docx/csv2xlsx/md2pptx)."
command -v cleave  >/dev/null || echo "Warning: cleave not installed (needed for html targets)."
command -v xsync   >/dev/null || echo "Warning: xsync not installed (needed for publish)."
