# Relith

> One context. Every AI.

Relith is a local-first context engine that indexes your codebases and exposes them through a unified interface for AI assistants.

Instead of every AI tool building its own isolated context, Relith acts as a shared intelligence layer that enables assistants to search, understand, and reason over the same knowledge base.

## Install

Download the latest binary from [releases](https://github.com/cryskram/relith/releases), or build from source:

```bash
# Build from source
git clone https://github.com/cryskram/relith.git
cd relith
make build-all
# Binaries in bin/relith (CLI) and bin/relithd (daemon)
```

## Quick Start

```bash
# Build the binaries
make build-all

# Add a repository
./bin/relith repo add /path/to/your/project

# Index it
./bin/relith index

# Search
./bin/relith search "your query"

# Check status
./bin/relith status
```

## Commands

| Command | Description |
|---------|-------------|
| `relith repo add <path>` | Register a repository for indexing |
| `relith repo list` | List all indexed repositories |
| `relith index [path]` | Index a repo (or all pending) |
| `relith search <query>` | Full-text search across all indexed code |
| `relith status` | Show indexing status with file/chunk counts |
| `relith version` | Print version |

**`relith repo add <path>`** — Validates the path, detects git remote origin URL, and creates a database entry.

**`relith index [path]`** — Walks the repository directory, computes SHA-256 hashes for change detection, detects programming languages, chunks files into overlapping segments (50 lines, 10 overlap), and writes to SQLite FTS5. On subsequent runs, only changed files are re-indexed.

**`relith search <query>`** — Full-text search using SQLite FTS5 with BM25 ranking. Supports simple terms, quoted phrases, prefix (`term*`), and boolean operators (`AND`, `OR`, `NOT`). Results with path matches rank higher when path boosting is enabled.

## Daemon

Start the HTTP API server:

```bash
relithd
```

By default it listens on a Unix socket. Use TCP instead by setting the socket to empty:

```bash
RELITH_DAEMON_SOCKET="" relithd
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET`    | `/v1/health`          | Health check |
| `GET`    | `/v1/repos`           | List repositories |
| `POST`   | `/v1/repos`           | Create repository |
| `GET`    | `/v1/repos/{id}`      | Get repository by ID |
| `DELETE` | `/v1/repos/{id}`      | Delete repository |
| `POST`   | `/v1/repos/{id}/index` | Trigger indexing |
| `GET`    | `/v1/search?q=`       | Full-text search |

### API Examples

```bash
# Unix socket (Linux)
curl -s --unix-socket ~/.local/share/relith/relith.sock http://local/v1/health

# TCP (Windows)
curl -s http://127.0.0.1:9876/v1/health

# Create a repository
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"path":"/path/to/repo","name":"my-repo"}' \
  http://127.0.0.1:9876/v1/repos

# Index
curl -s -X POST http://127.0.0.1:9876/v1/repos/1/index

# Search
curl -s "http://127.0.0.1:9876/v1/search?q=sqlite"
```

## Configuration

Config file: `~/.config/relith/relith.yaml` (Linux) or `%LOCALAPPDATA%/Relith/relith.yaml` (Windows). Environment variables with `RELITH_` prefix override file values.

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

## Data

SQLite database at `~/.local/share/relith/relith.db` (Linux) or `%LOCALAPPDATA%/Relith/relith.db` (Windows).

```sql
-- Inspect indexed files
SELECT id, path, language, length(hash) FROM documents;

-- View chunks
SELECT d.path, c.chunk_index, length(c.content) AS size
FROM chunks c JOIN documents d ON d.id = c.doc_id
WHERE d.path LIKE '%.go';
```

## Architecture

Two binaries:
- **`relith`** — CLI client (opens DB directly for simple operations)
- **`relithd`** — Daemon server (Unix socket + TCP HTTP API)

Components:
- `internal/api` — REST API with health, repo CRUD, indexing trigger, search
- `internal/indexer` — File walker, language detection, chunking, hash-based change detection
- `internal/search` — FTS5 query builder, BM25 ranking, path boosting
- `internal/watcher` — fsnotify-based file watcher with debounced re-indexing
- `internal/db` — SQLite with WAL mode, FTS5, goose migrations, sqlc-generated queries

Storage: SQLite with FTS5 full-text search, WAL mode, porter unicode61 tokenizer.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.

## License

MIT
