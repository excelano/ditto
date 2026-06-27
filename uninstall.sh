#!/bin/bash
# uninstall.sh - Remove ditto from ~/bin.
#
# Usage: ./uninstall.sh

set -euo pipefail

target="$HOME/bin/ditto"
if [ -e "$target" ]; then
    rm -f "$target"
    echo "Removed: $target"
else
    echo "Not installed: $target (nothing to remove)"
fi

# The ~/bin PATH entry in .bashrc is shared by the other nursery tools and is
# left alone on purpose. ditto only orchestrates other tools, so it does not
# touch them either: pandoc, office-convert, cleave, and xfiles stay installed.
