#!/usr/bin/env bash
# Reproducible Relith indexing benchmark.
#
# Downloads the Linux kernel source tarball (or uses a local copy), indexes it
# with Relith, and reports timing. Used by docs/benchmarks.md.
#
# Configuration (all optional):
#   RELITH_KERNEL_VERSION  kernel version to benchmark (default: 6.13)
#   RELITH_KERNEL_URL      override the tarball URL entirely
#   RELITH_BENCH_WORKDIR   scratch dir for downloads + data (default: /tmp/relith-bench)
#   RELITH_BIN             path to the relith binary (default: <repo>/bin/relith)
#
# Usage:
#   ./bench/index-kernel.sh            # from the repo root
#
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

VERSION="${RELITH_KERNEL_VERSION:-6.13}"
WORKDIR="${RELITH_BENCH_WORKDIR:-${TMPDIR:-/tmp}/relith-bench}"
RELITH_BIN="${RELITH_BIN:-${REPO_ROOT}/bin/relith}"

if command -v curl >/dev/null 2>&1; then
  DOWNLOADER="curl -fSL"
else
  DOWNLOADER="wget -O"
fi

mkdir -p "$WORKDIR"
WORKDIR="$(cd "$WORKDIR" && pwd)"

MAJOR="${VERSION%%.*}"
case "$MAJOR" in
  5|6|7|8) KERNEL_DIR="${MAJOR}.x" ;;
  *) echo "error: cannot infer kernel download directory for version ${VERSION}" >&2; exit 1 ;;
esac

TARBALL="linux-${VERSION}.tar.xz"
URL="${RELITH_KERNEL_URL:-https://cdn.kernel.org/pub/linux/kernel/${KERNEL_DIR}/${TARBALL}}"
KERNEL_PATH="${WORKDIR}/linux-${VERSION}"
DATA_DIR="${WORKDIR}/data-${VERSION}"

echo "==> Relith benchmark: Linux kernel ${VERSION}"
echo "    kernel : ${KERNEL_PATH}"
echo "    data   : ${DATA_DIR}"
echo "    binary : ${RELITH_BIN}"

if [ ! -x "$RELITH_BIN" ]; then
  echo "==> relith binary not found, building..."
  make -C "$REPO_ROOT" build-all
fi

if [ ! -d "$KERNEL_PATH" ]; then
  if [ ! -f "$WORKDIR/$TARBALL" ]; then
    echo "==> Downloading ${URL}"
    if command -v curl >/dev/null 2>&1; then
      curl -fSL -o "$WORKDIR/$TARBALL" "$URL"
    else
      wget -q -O "$WORKDIR/$TARBALL" "$URL"
    fi
  fi
  echo "==> Extracting ${TARBALL}"
  tar -C "$WORKDIR" -xJf "$WORKDIR/$TARBALL"
fi

rm -rf "$DATA_DIR"
export RELITH_CORE_DATA_DIR="$DATA_DIR"

echo "==> Adding repository"
"$RELITH_BIN" repo add "$KERNEL_PATH"

echo "==> Indexing (this is the measured phase)"
START_SECS=$(date +%s)
"$RELITH_BIN" index "$KERNEL_PATH"
END_SECS=$(date +%s)
ELAPSED=$((END_SECS - START_SECS))

echo ""
echo "==> Index complete in ${ELAPSED} seconds"
"$RELITH_BIN" status
