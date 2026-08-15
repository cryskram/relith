#!/bin/sh
# Install the latest Relith release for the current OS/arch.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cryskram/relith/main/install.sh | sh
#
# Environment overrides:
#   RELITH_VERSION      specific version to install (default: latest)
#   RELITH_INSTALL_DIR  install directory (default: $HOME/.local/bin)
#   RELITH_REPO         repo to fetch from (default: cryskram/relith)
#
set -e

REPO="${RELITH_REPO:-cryskram/relith}"
VERSION="${RELITH_VERSION:-}"
INSTALL_DIR="${RELITH_INSTALL_DIR:-$HOME/.local/bin}"

detect_os() {
	case "$(uname -s)" in
		Linux) echo linux ;;
		Darwin) echo darwin ;;
		MINGW* | MSYS* | CYGWIN*) echo windows ;;
		*) echo "error: unsupported OS: $(uname -s)" >&2; exit 1 ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		x86_64 | amd64) echo amd64 ;;
		aarch64 | arm64) echo arm64 ;;
		*) echo "error: unsupported arch: $(uname -m)" >&2; exit 1 ;;
	esac
}

fetch_latest_version() {
	curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
		sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' |
		head -n1
}

command -v curl >/dev/null 2>&1 || {
	echo "error: curl is required" >&2
	exit 1
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ -z "$VERSION" ]; then
	VERSION="$(fetch_latest_version)"
fi
VERSION="${VERSION#v}"
if [ -z "$VERSION" ]; then
	echo "error: could not determine latest release for ${REPO}" >&2
	exit 1
fi

FILE="relith_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILE}"

EXT=""
if [ "$OS" = "windows" ]; then
	EXT=".exe"
fi

echo "==> Relith ${VERSION} (${OS}/${ARCH})"
echo "==> Downloading ${URL}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL -o "${TMP_DIR}/${FILE}" "${URL}"
tar -xzf "${TMP_DIR}/${FILE}" -C "${TMP_DIR}"

mkdir -p "$INSTALL_DIR"
install -m 0755 "${TMP_DIR}/relith${EXT}" "$INSTALL_DIR/relith${EXT}"
install -m 0755 "${TMP_DIR}/relithd${EXT}" "$INSTALL_DIR/relithd${EXT}"
install -m 0755 "${TMP_DIR}/relithmcp${EXT}" "$INSTALL_DIR/relithmcp${EXT}"

echo "==> Installed relith, relithd, relithmcp to ${INSTALL_DIR}"
echo "    Add it to your PATH: export PATH=\"\${PATH}:${INSTALL_DIR}\""
