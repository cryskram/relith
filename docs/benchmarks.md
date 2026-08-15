# Relith Benchmarks

Relith indexes a codebase once, then serves every MCP client from the same
SQLite (FTS5) index. This document describes how to reproduce indexing
measurements and reports the numbers seen on the maintainer's machine.

## Published numbers

The [README](../README.md) performance table is measured against the **Linux
kernel source tree**:

| Phase | Time |
|-------|------|
| Walk + index | 14m 40s |
| Graph build | 1m 8s |
| **Total** | **15m 48s** |

- Corpus: Linux kernel **6.x**, **94,989** files, **~1.7M** chunks
- Graph build was optimized 21× (24m 12s → 1m 8s) by pre-computing edges into a
  `graph_edges` table instead of running the reference JOIN per query - see
  [ARCHITECTURE.md](../ARCHITECTURE.md) for details.

Numbers vary by machine, disk type, and concurrency. Use the script below to
get measurements for your own hardware.

## Reproducing

The benchmark is scripted in [`bench/index-kernel.sh`](../bench/index-kernel.sh):

```bash
# from the repo root (builds relith, downloads kernel 6.13, indexes, times it)
./bench/index-kernel.sh

# or a different kernel version:
RELITH_KERNEL_VERSION=6.12 ./bench/index-kernel.sh
```

What the script does:

1. Builds `bin/relith` if missing (`make build-all`).
2. Downloads `linux-<version>.tar.xz` from `cdn.kernel.org` into
   `$RELITH_BENCH_WORKDIR` (default `/tmp/relith-bench`) if not already present.
3. Extracts the source.
4. Uses a fresh, isolated data directory so the run is clean
   (`RELITH_CORE_DATA_DIR`).
5. Runs `relith repo add` then `relith index <path>` in non-TTY mode.
6. Reports elapsed wall-clock time and the indexer's file/chunk counts.

### Environment knobs

| Variable | Default | Meaning |
|----------|---------|---------|
| `RELITH_KERNEL_VERSION` | `6.13` | Kernel version to benchmark |
| `RELITH_KERNEL_URL` | auto | Override the tarball URL |
| `RELITH_BENCH_WORKDIR` | `/tmp/relith-bench` | Scratch dir for tarball + data |
| `RELITH_BIN` | `<repo>/bin/relith` | Path to the relith binary |

### Interpreting output

The final lines look like:

```
==> Index complete in 948 seconds
```

followed by `relith status`, which reports file and chunk counts. Compare
**files indexed** and **elapsed seconds**; the chunk count depends on the
indexer's chunking configuration.

## Notes

- `relith index` reports total elapsed for the whole pipeline (walk → chunk →
  symbol/reference extraction → graph build). The walk/index vs graph-build
  split above comes from internal phase timing.
- Indexing is I/O-bound: NVMe vs HDD and cold vs warm caches change results
  significantly. Report your hardware when sharing numbers.
