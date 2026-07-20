<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="images/logo.png">
    <img src="images/logo.png" alt="Relith" width="400">
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

Relith is a **local-first context engine** that indexes your codebases and exposes them through a unified interface for AI assistants. Instead of every AI tool building its own isolated context, Relith acts as a **shared intelligence layer** - one index, any AI.

## Features

- **Full-text search** - SQLite FTS5 with BM25 ranking, prefix matching, boolean operators
- **Multi-repo support** - Index unlimited repos, search across them all at once
- **MCP-native** - Works with Cursor, Claude Code, OpenCode, and any MCP client
- **REST API** - HTTP server for scripts, CI pipelines, and programmatic access
- **File watcher** - Auto-reindexes changed files via fsnotify
- **Local-first** - Your code never leaves your machine, zero cloud dependencies
- **Single binary** - Go binary, no npm/pip/uv, no Docker, no runtime

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

# Launch daemon for MCP
./bin/relithd
```

## CLI

| Command | Description |
|---------|-------------|
| `relith repo add <path>` | Register a repository for indexing |
| `relith repo list` | List all indexed repositories |
| `relith index [path]` | Index a repo (or all pending) |
| `relith search <query>` | Full-text search across all indexed code |
| `relith status` | Show indexing status with file/chunk counts |
| `relith version` | Print version |

## MCP Server

Relith exposes an **MCP server** that AI assistants connect to directly. Supported clients: **Cursor**, **Claude Code**, **OpenCode**, and any MCP-compatible tool.

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_code` | Full-text search across indexed code | `query` (req), `repo_name`, `language`, `max_results` |
| `get_file_content` | Retrieve file content by repo + path | `repo_name` (req), `path` (req) |
| `list_repositories` | List all tracked repos | - |
| `get_repo_summary` | Language breakdown, file/chunk counts | `repo_name` (req) |

### Setup

**Cursor** - Settings → MCP Servers → Add new:

```
Name: relith
Type: command
Command: D:\relith\bin\relithmcp.exe
```

**Claude Code** - add to `~/.config/claude/mcp.json`:

```json
{
  "mcpServers": {
    "relith": {
      "command": "relithmcp"
    }
  }
}
```

## REST API

The daemon (`relithd`) provides an HTTP API for programmatic access:

```bash
curl -s http://127.0.0.1:9876/v1/health
curl -s -X POST http://127.0.0.1:9876/v1/repos \
  -H "Content-Type: application/json" \
  -d '{"path":"/path/to/repo","name":"my-repo"}'
curl -s -X POST http://127.0.0.1:9876/v1/repos/1/index
curl -s "http://127.0.0.1:9876/v1/search?q=sqlite"
```

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/health` | Health check |
| `GET` | `/v1/repos` | List repositories |
| `POST` | `/v1/repos` | Create repository |
| `GET` | `/v1/repos/{id}` | Get repository by ID |
| `DELETE` | `/v1/repos/{id}` | Delete repository |
| `POST` | `/v1/repos/{id}/index` | Trigger indexing |
| `GET` | `/v1/search?q=` | Full-text search |

## Configuration

Config file: `~/.config/relith/relith.yaml` or `%LOCALAPPDATA%\Relith\relith.yaml`.  
Environment variables with `RELITH_` prefix override file values.

```yaml
core:
  data_dir: ~/.local/share/relith
indexer:
  concurrency: 4
  max_file_size: 10485760
log:
  level: info
  format: console
  output: stderr
```

## Architecture

Three binaries sharing the same SQLite database:

| Binary | Role | Interface |
|--------|------|-----------|
| `relith` | CLI client | Terminal commands |
| `relithd` | Daemon | REST API (Unix socket / TCP) |
| `relithmcp` | MCP server | stdio JSON-RPC (AI assistants) |

**Stack**: Go + SQLite (FTS5, WAL mode, porter unicode61) + sqlc-generated queries.

## License

MIT - see [LICENSE](LICENSE) for details.
