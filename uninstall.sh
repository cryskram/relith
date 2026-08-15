#!/bin/sh
# Uninstall Relith: remove binaries, and optionally data and config.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cryskram/relith/main/uninstall.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/cryskram/relith/main/uninstall.sh | sh -s -- --purge
#
# What it does:
#   1. Runs `relith uninstall` to remove MCP entries from AI agent configs
#      (opencode, cursor, claude-code), if the binary is on PATH.
#   2. Removes the relith, relithd, and relithmcp binaries.
#   3. Removes the data dir (indexes/database) and config dir only when run
#      interactively and you confirm, or when --purge is passed.
#
# Environment overrides:
#   RELITH_INSTALL_DIR  install directory (default: $HOME/.local/bin)
#
set -e

INSTALL_DIR="${RELITH_INSTALL_DIR:-$HOME/.local/bin}"

PURGE=0
case "$1" in
	--purge) PURGE=1 ;;
	"") ;;
	*) echo "error: unknown argument: $1" >&2; exit 1 ;;
esac

EXT=""
case "$(uname -s)" in
	MINGW* | MSYS* | CYGWIN*) EXT=".exe" ;;
esac

if command -v relith >/dev/null 2>&1; then
	echo "==> Removing MCP configuration from AI agents..."
	relith uninstall || echo "    (relith uninstall reported an error; continuing)"
else
	echo "==> relith not on PATH; skipping agent config cleanup"
fi

found=0
for bin in relith relithd relithmcp; do
	path="$INSTALL_DIR/$bin$EXT"
	if [ -f "$path" ]; then
		rm -f "$path"
		echo "    removed $path"
		found=1
	fi
done
if [ "$found" = 0 ]; then
	echo "==> No relith binaries found in $INSTALL_DIR"
fi

DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/relith"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/relith"

want_data=0
if [ "$PURGE" = 1 ]; then
	want_data=1
elif [ -t 0 ] && { [ -d "$DATA_DIR" ] || [ -d "$CONFIG_DIR" ]; }; then
	printf "==> Remove data (%s) and config (%s) too? [y/N] " "$DATA_DIR" "$CONFIG_DIR"
	read -r answer
	case "$answer" in
		y | Y | yes | YES) want_data=1 ;;
	esac
fi

if [ "$want_data" = 1 ]; then
	if [ -d "$DATA_DIR" ]; then
		rm -rf "$DATA_DIR"
		echo "    removed data: $DATA_DIR"
	fi
	if [ -d "$CONFIG_DIR" ]; then
		rm -rf "$CONFIG_DIR"
		echo "    removed config: $CONFIG_DIR"
	fi
else
	echo "==> Kept data ($DATA_DIR) and config ($CONFIG_DIR)"
	echo "    Remove them manually, or re-run with --purge."
fi

echo "==> Relith uninstalled."
