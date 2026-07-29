<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="images/logo.png">
    <img src="images/logo.png" alt="Relith" width="300">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/cryskram/relith/releases"><img src="https://img.shields.io/github/v/release/cryskram/relith?style=for-the-badge&logo=github&color=e94560" alt="Release"></a>
  <a href="https://github.com/cryskram/relith/stargazers"><img src="https://img.shields.io/github/stars/cryskram/relith?style=for-the-badge&logo=github&color=3178C6" alt="Stars"></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=for-the-badge&logo=go" alt="Go"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=for-the-badge" alt="License"></a>
  <a href="#"><img src="https://img.shields.io/badge/platform-linux%20%7C%20windows%20%7C%20macOS-969696?style=for-the-badge" alt="Platform"></a>
</p>

<br>

Relith is a **local-first context engine** for AI-assisted coding. It indexes your codebases and exposes them through a unified MCP interface — one index, any AI.

Instead of every AI tool building its own isolated context, Relith acts as a **shared intelligence layer** that Cursor, Claude Code, OpenCode, and any MCP client can query for code search, symbol lookup, reference tracking, and dependency graph traversal.

## Features

- **MCP-native** — 17 tools for AI assistants: search, symbols, references, definitions, callers/callees, file outline, dependency trace, graph queries, architecture overview
- **Cross-file reasoning** — Combine FTS search + symbol matches + references + graph neighbors into one context bundle via `trace_context`
- **Knowledge graph** — Typed dependency graph with import edges (Go/JS/TS/Python/Rust) and reference co-occurrence edges (all languages)
- **Graph visualization** — Interactive D3.js force-directed graph in the browser (`/v1/graph`)
- **Symbol extraction** — Functions, types, structs, methods, interfaces, enums, traits, macros per language (17 languages)
- **Reference extraction** — Function calls, imports, usages across all code files
- **Multi-repo** — Index unlimited repos, search across them all at once
- **File watcher** — Auto-reindexes changed files via fsnotify
- **REST API** — HTTP server for scripts, CI pipelines, and programmatic access
- **Terminal UI** — Bubble Tea progress bars, spinners, and server dashboard for interactive commands
- **Single binary** — Go binary, no npm/pip/uv, no Docker, no runtime
- **Local-first** — Your code never leaves your machine, zero cloud dependencies

### Performance

Linux kernel benchmark (94,989 files, 1.7M chunks):

| Phase | Time |
|-------|------|
| Walk + index | 14m 40s |
| Graph build | 1m 8s |
| **Total** | **15m 48s** |

## Quick Start

```bash
# Build
git clone https://github.com/cryskram/relith.git
cd relith
make build-all

# Index your codebase
./bin/relith repo add /path/to/your/project
./bin/relith index

# Search
./bin/relith search "your query"

# Auto-install MCP for your AI agent
./bin/relith install

# Launch daemon for REST API + graph UI
./bin/relithd
```

> **macOS**: If Gatekeeper blocks the downloaded binary, run `xattr -d com.apple.quarantine /path/to/binary` then right-click → Open.

## CLI

| Command | Description |
|---------|-------------|
| `relith repo add <path>` | Register a repository for indexing |
| `relith repo list` | List all indexed repositories |
| `relith repo remove <id-or-name>` | Remove a repository and all its data |
| `relith index [path]` | Index a repo (or all pending) |
| `relith search <query>` | Full-text search across all indexed code |
| `relith status` | Show indexing status with file/chunk counts |
| `relith serve` | Start the daemon (REST API + graph UI + file watcher) |
| `relith install` | Auto-detect and configure MCP for OpenCode, Cursor, Claude Code |
| `relith uninstall` | Remove relith MCP configuration from agents |
| `relith db vacuum` | Reclaim unused database space |
| `relith version` | Print version |

Interactive commands (`index`, `remove`, `serve`) show a Bubble Tea TUI automatically when run in a terminal.

## MCP Server

Relith exposes an MCP server over stdio. Connect any MCP-compatible AI assistant (Cursor, Claude Code, OpenCode) — 17 tools covering search, symbol intelligence, graph traversal, and file operations.

### Setup

```bash
# Auto-detect and configure for OpenCode, Cursor, Claude Code
relith install

# Or target a specific agent
relith install --agent=cursor
relith install --agent=opencode
relith install --agent=code   # Claude Code
```

After running `relith install`, restart your agent. The tools are available automatically.

### Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `search_code` | Full-text search across all indexed repos | `query` (req), `repo_name`, `language`, `max_results` |
| `get_file_content` | Retrieve file content by repo + path | `repo_name` (req), `path` (req) |
| `list_repositories` | List all tracked repos | — |
| `get_repo_summary` | Language breakdown, file/chunk counts, last indexed | `repo_name` (req) |
| `find_symbol` | Search symbols by name prefix | `name` (req), `kind`, `repo_name` |
| `find_references` | Find all call sites for a symbol across all repos | `name` (req), `repo_name` |
| `trace_context` | Combined FTS + symbols + references + graph neighbors | `query` (req), `repo_name`, `max_results` |
| `get_file_outline` | File metadata + chunks + symbols + references | `repo_name` (req), `path` (req) |
| `get_symbol_definition` | Exact symbol definition with repo/file/kind/snippet | `name` (req), `repo_name`, `kind` |
| `find_callees` | Functions called inside a symbol's definition file | `name` (req), `repo_name` |
| `find_callers` | Find exact call sites for a symbol across repos | `name` (req), `repo_name` |
| `get_related_files` | Graph-neighbor files for a given repo/path | `repo_name` (req), `path` (req) |
| `list_hub_files` | Most connected files (degree centrality) in a repo | `repo_name`, `max_results` |
| `query_graph` | Query the knowledge graph: neighbors, hotspots, dependency paths | `mode` (req), `repo_name` (req), `path` |
| `get_architecture` | High-level architecture: languages, directories, entry points, hotspots | `repo_name` (req) |
| `trace_dependency` | Trace import/reference chains (recursive) | `repo_name` (req), `path` (req), `direction`, `depth` |
| `get_file_tree` | Browse directory tree — show immediate children at a path | `repo_name` (req), `path` |

### Manual Setup

**Cursor** — Settings → MCP Servers → Add new:
```
Name: relith
Type: command
Command: /path/to/relithmcp
```

**Claude Code** — add to `~/.config/claude/mcp.json`:
```json
{"mcpServers": {"relith": {"command": "/path/to/relithmcp"}}}
```

> **macOS**: If Gatekeeper blocks the binary, run `xattr -d com.apple.quarantine /path/to/relithmcp` first.

## REST API

The daemon (`relithd`) provides an HTTP API over Unix socket (Linux) or TCP (Windows):

```bash
curl -s http://127.0.0.1:9876/v1/health
curl -s http://127.0.0.1:9876/v1/stats
curl -s "http://127.0.0.1:9876/v1/search?q=sqlite"
curl -s "http://127.0.0.1:9876/v1/graph?repo=my-repo"
curl -s "http://127.0.0.1:9876/v1/graph"   # Interactive D3.js graph UI (browser)
```

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Dashboard web UI |
| GET | `/v1/health` | Health check |
| GET | `/v1/repos` | List repositories |
| POST | `/v1/repos` | Create repository |
| GET | `/v1/repos/{id}` | Get repository |
| DELETE | `/v1/repos/{id}` | Delete repository |
| POST | `/v1/repos/{id}/index` | Trigger indexing |
| GET | `/v1/search?q=` | Full-text search |
| GET | `/v1/content?repo=&path=` | File content |
| GET | `/v1/reason?q=&repo=` | Cross-file reasoning bundle |
| GET | `/v1/stats` | Aggregate stats with savings % |
| GET | `/v1/graph?repo=` | Graph edges data (JSON) |
| GET | `/v1/graph` | Interactive graph visualization (HTML) |

## Configuration

Config file: `~/.config/relith/relith.yaml` or `%LOCALAPPDATA%\Relith\relith.yaml`.

Environment variables with `RELITH_` prefix override file values.

```yaml
core:
  data_dir: ~/.local/share/relith

daemon:
  socket: ""                    # empty = TCP; set to use Unix socket
  tcp_host: 127.0.0.1
  tcp_port: 9876

mcp:
  enabled: true
  transport: stdio
  tcp_port: 9877

indexer:
  concurrency: 4                # parallel file workers
  max_file_size: 10485760       # 10 MB limit

watcher:
  enabled: true
  debounce: 1s

search:
  max_results: 100
  path_boosting: true
```

## Architecture

Three binaries sharing the same SQLite database:

| Binary | Role | Interface |
|--------|------|-----------|
| `relith` | CLI client | Terminal commands (Bubble Tea TUI for interactive) |
| `relithd` | Daemon | REST API + graph UI + file watcher |
| `relithmcp` | MCP server | stdio JSON-RPC for AI assistants |

**Stack**: Go + SQLite (FTS5, WAL, sqlc-generated queries, compound indexes).

**Data flow**: Walk → chunk → extract symbols + refs → batch INSERT → build graph edges (import + reference co-occurrence) → store in `graph_edges` table.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.

## License

MIT — see [LICENSE](LICENSE) for details.
