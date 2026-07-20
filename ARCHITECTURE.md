# Relith Architecture

> One context. Every AI.

1. [High-Level Architecture](#1-high-level-architecture)
2. [Folder Structure](#2-folder-structure)
3. [Package Responsibilities](#3-package-responsibilities)
4. [Data Flow](#4-data-flow)
5. [Component Interactions](#5-component-interactions)
6. [Database Schema](#6-database-schema)
7. [API Design](#7-api-design)
8. [MCP Tools](#8-mcp-tools)
9. [Indexing Workflow](#9-indexing-workflow)
10. [Search Architecture](#10-search-architecture)
11. [Daemon Lifecycle](#11-daemon-lifecycle)
12. [Configuration Structure](#12-configuration-structure)
13. [Logging Strategy](#13-logging-strategy)
14. [Version Roadmap](#14-version-roadmap)

## 1. High-Level Architecture

```
┌─────────────────────────────────────────────────────┐
│                   External World                      │
│  ┌────────┐  ┌──────────┐  ┌──────────┐  ┌───────┐  │
│  │ Cursor  │  │ OpenCode │  │ Claude   │  │ Copilot│  │
│  └────┬───┘  └────┬─────┘  └────┬─────┘  └───┬───┘  │
│       │           │             │             │       │
│       └───────────┼─────────────┼─────────────┘       │
│                   │             │                      │
│               (MCP Protocol)    │                      │
└───────────────────┼─────────────┼──────────────────────┘
                    │             │
         ┌──────────▼──┐   ┌─────▼────────┐
         │   MCP Server │   │  REST API    │
         │ (relithmcp)  │   │  (relithd)   │
         │   stdio      │   │  socket/TCP  │
         └──────┬───────┘   └──────┬───────┘
                │                  │
         ┌──────▼──────────────────▼───────┐
         │         Indexer + Search         │
         │       (internal/indexer,         │
         │        internal/search)          │
         │                │                 │
         │         ┌──────▼──────┐          │
         │         │    SQLite   │          │
         │         │  (FTS5+WAL) │          │
         │         └─────────────┘          │
         └──────────────────────────────────┘
                      ▲
                      │ Direct DB access
                      │
         ┌────────────┴───────────┐
         │  CLI (relith, cobra)    │
         └────────────────────────┘
```

### Key Decisions

- **Three separate binaries**: `relith` (CLI), `relithd` (daemon), `relithmcp` (MCP server). MCP requires pure JSON-RPC over stdio with no flag parsing, so it must be its own binary.
- **CLI opens DB directly**: No daemon HTTP hop for CLI operations. Simpler, faster, no dependency on a running daemon.
- **Unix socket for daemon API**: File-system permissions as security boundary, no port conflicts. Windows falls back to localhost TCP.
- **SQLite with FTS5**: Zero-dependency embedded database with full-text search. WAL mode allows concurrent readers (MCP + daemon + CLI can coexist).

## 2. Folder Structure

```
relith/
├── cmd/
│   ├── relith/                    # CLI client
│   │   └── main.go
│   ├── relithd/                   # Daemon (REST API server)
│   │   └── main.go
│   └── relithmcp/                 # MCP server for AI assistants
│       └── main.go
│
├── internal/
│   ├── api/                       # REST API layer
│   │   ├── handlers.go            # Route handlers (health, repos, search)
│   │   ├── response.go            # JSON response helpers
│   │   └── server.go              # HTTP server, routing, middleware
│   │
│   ├── mcp/                       # MCP protocol server
│   │   ├── mcp.go                 # Protocol types (JSON-RPC, capabilities)
│   │   ├── server.go              # JSON-RPC dispatcher, lifecycle
│   │   ├── tools.go               # Tool handlers (search_code, etc.)
│   │   └── resources.go           # Resource URI handlers
│   │
│   ├── indexer/                   # Core indexing engine
│   │   ├── indexer.go             # Orchestrator (IndexRepo, IndexFile, DeleteFile)
│   │   ├── walker.go              # Directory walk + binary/hidden file filter
│   │   ├── chunker.go             # Line-based chunking with overlap
│   │   └── language.go            # Extension-to-language mapping
│   │
│   ├── watcher/                   # Filesystem event watcher
│   │   ├── watcher.go             # fsnotify wrapper
│   │   └── debouncer.go           # Coalesce rapid events
│   │
│   ├── db/                        # Data access layer (sqlc-generated)
│   │   ├── db.go                  # Connection, WAL, PRAGMAs
│   │   ├── migrate.go             # Goose migration runner
│   │   ├── models.go              # Generated types (Repository, Document, Chunk)
│   │   ├── querier.go             # Generated interface
│   │   ├── repos.sql.go           # Repo CRUD
│   │   ├── documents.sql.go       # Document CRUD
│   │   └── chunks.sql.go          # Chunk CRUD + FTS5 sync
│   │
│   ├── search/                    # Search abstraction over FTS5
│   │   ├── search.go              # Searcher with FTS5 queries
│   │   └── query.go               # Query builder (prefix, phrase, operators)
│   │
│   ├── daemon/                    # Orchestrator
│   │   └── daemon.go              # Init DB, start API server, signal handling
│   │
│   ├── config/                    # Configuration
│   │   ├── config.go              # Viper setup, defaults, validation
│   │   └── paths.go               # Platform-specific data/config/socket paths
│   │
│   ├── cli/                       # CLI commands (cobra)
│   │   ├── root.go                # Root command
│   │   ├── repo_add.go            # repo add
│   │   ├── repo_list.go           # repo list
│   │   ├── index.go               # index
│   │   ├── search.go              # search
│   │   ├── status.go              # status
│   │   ├── util.go                # Shared DB open helper
│   │   └── version.go             # Version command + ldflags injection
│   │
│   ├── app/                       # Shared application struct
│   │   └── app.go
│   │
│   └── logger/                    # Structured logging
│       └── logger.go              # Zerolog setup (console/json)
│
├── sql/
│   ├── migrations/                # SQL migration files (embed.FS)
│   │   ├── 00001_initial.sql
│   │   ├── 00002_schema.sql
│   │   └── migrations.go          # go:embed
│   └── queries/                   # sqlc query definitions
│       ├── repos.sql
│       ├── documents.sql
│       └── chunks.sql
│
├── bin/                           # Build output (gitignored)
├── docs/                          # ADRs (empty, planned)
├── go.mod, go.sum
├── Makefile
├── .goreleaser.yaml
├── .golangci.yml
└── README.md
```

### Why this structure

- **`internal/`**: Go visibility enforcement -- these packages cannot be imported by external consumers.
- **`sql/` separate from `db/`**: Source of truth (SQL migrations + sqlc queries) vs generated Go code.
- **`cmd/`**: Thin entry points -- parse flags, load config, launch component. Zero business logic.
- **`bin/`**: Build output, gitignored.

## 3. Package Responsibilities

| Package            | Responsibility                                                       | Dependencies (internal)                              |
| ------------------ | -------------------------------------------------------------------- | ---------------------------------------------------- |
| `cmd/relith`       | Parse CLI flags (cobra), open DB, dispatch commands                  | `internal/cli`, `internal/config`, `internal/db`     |
| `cmd/relithd`      | Parse flags, load config, instantiate daemon, block on signal        | `internal/daemon`, `internal/config`                 |
| `cmd/relithmcp`    | Load config, open DB, start MCP server over stdio                    | `internal/mcp`, `internal/config`, `internal/db`     |
| `internal/api`     | HTTP routing, request validation, JSON marshaling                    | `internal/db`, `internal/search`                     |
| `internal/mcp`     | JSON-RPC over stdio, tool/resource registration, dispatch            | `internal/db`, `internal/search`                     |
| `internal/indexer` | Walk filesystems, detect languages, chunk content, hash-based diff   | `internal/db`                                        |
| `internal/watcher` | Wrap fsnotify, debounce, filter, call IndexFile/DeleteFile           | `internal/indexer`                                   |
| `internal/db`      | Connection lifecycle, migration runner, sqlc-generated methods       | None (sqlite driver only)                            |
| `internal/search`  | FTS5 query construction, BM25 ranking, result formatting             | `internal/db`                                        |
| `internal/daemon`  | Component wiring, graceful shutdown, signal handling                 | `internal/api`, `internal/config`                    |
| `internal/config`  | Load/merge config from file + env, validate, defaults                | viper                                                |
| `internal/cli`     | Cobra command definitions for repo CRUD, index, search, status       | `internal/db`, `internal/indexer`, `internal/search` |
| `internal/logger`  | Zerolog setup (console/json output, level, file)                     | `internal/config`                                    |
| `internal/app`     | Shared App struct (Config, Logger, DB)                               | `internal/config`                                    |

## 4. Data Flow

### A. Adding and Indexing a Repository

```
User: relith repo add /path/to/project

CLI ── open DB ──▶ INSERT INTO repositories
               ──▶ Indexer: WalkRepo → for each file:
                       - compute SHA-256 hash
                       - detect language
                       - chunk content (50 lines, 10 overlap)
                       - write document + chunks to DB
                       - FTS5 sync triggers populate chunks_fts
```

### B. Search Query

```
AI Tool ── MCP "search_code" ──▶ MCP Server
                                       │
                                       ▼
                                   search.go: buildMatchQuery("auth middleware")
                                       │
                                       ▼
                                   SELECT FROM chunks_fts JOIN chunks JOIN documents JOIN repositories
                                   WHERE chunks_fts MATCH ?
                                   ORDER BY rank (+ path boost)
                                       │
                                       ▼
                                   Return []Result to MCP client
```

### C. File Change Detection (Watcher)

```
Filesystem change (editor saves)
       │
       ▼
   fsnotify event
       │
       ▼
   Debouncer (coalesces within configurable window)
       │
       ▼
   Indexer.IndexFile(): compute hash, if changed, update document + chunks
   or
   Indexer.DeleteFile(): if file no longer exists, remove document + chunks
```

## 5. Component Interactions

Three interaction patterns:

### Pattern 1: CLI Direct DB

```
CLI → open DB → Queries/Indexer → DB → output
```

Used for: adding repos, listing repos, indexing, search, status.

### Pattern 2: Daemon HTTP API

```
Client (curl/app) → HTTP → API Handler → Queries/Searcher → JSON response
```

Used for: health checks, programmatic access, remote queries.

### Pattern 3: MCP Request

```
AI Tool (JSON-RPC over stdio) → MCP Server → Searcher/Queries → JSON-RPC response
```

Used for: AI assistant integration (Cursor, Claude Code, OpenCode).

## 6. Database Schema

```sql
-- Enable WAL mode for concurrent reads
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;

-- Tracked repositories
CREATE TABLE repositories (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    path            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    remote_url      TEXT,
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK(status IN ('pending','indexing','ready','failed')),
    last_indexed_at DATETIME,
    file_count      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- File metadata (one row per indexed file)
CREATE TABLE documents (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id         INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    path            TEXT NOT NULL,
    size            INTEGER NOT NULL,
    hash            TEXT NOT NULL,            -- SHA-256 hex
    mod_time        DATETIME NOT NULL,
    mime_type       TEXT,
    language        TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_id, path)
);
CREATE INDEX idx_documents_repo_id ON documents(repo_id);
CREATE INDEX idx_documents_language ON documents(language);

-- Content chunks (one file can have many chunks)
CREATE TABLE chunks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_id          INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index     INTEGER NOT NULL,
    content         TEXT NOT NULL,
    UNIQUE(doc_id, chunk_index)
);
CREATE INDEX idx_chunks_doc_id ON chunks(doc_id);

-- FTS5 virtual table with content-sync triggers
CREATE VIRTUAL TABLE chunks_fts USING fts5(
    content,
    doc_id UNINDEXED,
    content=chunks,
    content_rowid=id,
    tokenize='porter unicode61'
);

-- Triggers to keep FTS5 in sync
CREATE TRIGGER chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts(rowid, doc_id, content) VALUES (new.id, new.doc_id, new.content);
END;
CREATE TRIGGER chunks_ad AFTER DELETE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, doc_id, content) VALUES ('delete', old.id, old.doc_id, old.content);
END;
CREATE TRIGGER chunks_au AFTER UPDATE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, doc_id, content) VALUES ('delete', old.id, old.doc_id, old.content);
    INSERT INTO chunks_fts(rowid, doc_id, content) VALUES (new.id, new.doc_id, new.content);
END;

-- Internal key-value store
CREATE TABLE metadata (
    key     TEXT PRIMARY KEY,
    value   TEXT NOT NULL
);
```

### Schema Decisions

- **INTEGER primary keys (auto-increment)**: Simpler than UUIDs for an MVP. Sequential IDs cluster well in SQLite B-trees.
- **DATETIME (not TEXT)**: SQLite has no native datetime type, but TEXT ISO 8601 is used for portability. INTEGER timestamps are also valid.
- **Content-sync FTS5**: The `content=chunks` declaration tells FTS5 to sync automatically via triggers on the `chunks` table. No manual FTS insert/update/delete needed.
- **`documents` table (not `files`)**: Named `documents` to avoid confusion with filesystem files and to leave room for non-file documents in the future (e.g., documentation pages).

## 7. API Design

The daemon (`relithd`) exposes a REST API over Unix socket (Linux) or TCP (Windows).

### Conventions

- Base path: `/v1/`
- JSON request/response bodies
- `Content-Type: application/json`

### Endpoints

```
# Lifecycle
GET    /v1/health                     → {"status":"ok"}

# Repositories
GET    /v1/repos                      → [{...repos}]
POST   /v1/repos                      → {...repo}  (body: {"path": "...", "name": "..."})
GET    /v1/repos/{id}                  → {...repo}
DELETE /v1/repos/{id}                  → 204 No Content
POST   /v1/repos/{id}/index            → {"files_indexed": N, "files_skipped": N, "elapsed": "..."}

# Search
GET    /v1/search?q=<query>            → [{doc_id, path, language, repo_name, content, score}]
```

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

# Trigger indexing
curl -s -X POST http://127.0.0.1:9876/v1/repos/1/index

# Search
curl -s "http://127.0.0.1:9876/v1/search?q=sqlite"
```

### Not yet implemented (future)

- Filtered file listing (`GET /v1/repos/:id/files`)
- Commit history (`GET /v1/repos/:id/commits`)
- Search suggestions (`GET /v1/search/suggest`)
- SSE event stream (`GET /v1/events`)

## 8. MCP Tools

The MCP server (`relithmcp`) implements the [Model Context Protocol](https://modelcontextprotocol.io) specification over stdio.

### Tools

| Tool Name             | Description                                      | Parameters                                                                                  |
| --------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| `search_code`         | Full-text search across indexed repos            | `query` (required), `repo_name` (optional), `language` (optional), `max_results` (default 20) |
| `get_file_content`    | Retrieve a file's content by repo name + path    | `repo_name` (required), `path` (required)                                                     |
| `list_repositories`   | List all tracked repos with status and file count | -                                                                                           |
| `get_repo_summary`    | Language breakdown, file/chunk count, last indexed| `repo_name` (required)                                                                        |

### Transport

- **stdio** (default): AI assistant spawns `relithmcp` as subprocess. Simplest integration, no port management.
- **TCP** (planned): For persistent connections when running inside the daemon.

### Protocol

Uses JSON-RPC 2.0 with MCP protocol version `2024-11-05`. Session lifecycle:

1. Client sends `initialize` request
2. Server responds with capabilities (tools + resources)
3. Client sends `notifications/initialized`
4. Normal operation: `tools/list`, `tools/call`, `resources/list`, `resources/read`

### Resources

```
relith://repos                  → All repositories (JSON)
relith://repos/{id}              → Repository metadata
```

## 9. Indexing Workflow

### Initial Index (Full Pass)

1. Open repo via config or CLI
2. Set repo status to `indexing`
3. Walk directory tree:
   - Skip `.git`, `node_modules`, `vendor`, `__pycache__`, hidden files, binary extensions
   - Skip files > max_file_size (default 10MB)
   - Skip empty files
4. For each qualifying file:
   - Compute SHA-256 hash
   - Read mod_time, size
   - Detect language (extension map: ~90 languages)
   - Read content
   - Chunk into overlapping segments (default 50 lines, 10 overlap)
5. Write to DB in a transaction:
   - Create/update document row
   - Delete old chunks (if updating)
   - Insert new chunks (FTS5 sync triggers populate `chunks_fts`)
6. Update repo: status=`ready`, file_count, last_indexed_at

### Incremental Index (File Change via Watcher)

1. Receive fsnotify event (file created/modified/deleted)
2. Debounce (coalesce rapid events)
3. If file exists and hash unchanged → skip
4. If file exists and hash changed → update document + chunks
5. If file deleted → delete document + chunks (cascade)

## 10. Search Architecture

### FTS5 Virtual Table

```sql
CREATE VIRTUAL TABLE chunks_fts USING fts5(
    content,
    doc_id UNINDEXED,
    content=chunks,
    content_rowid=id,
    tokenize='porter unicode61'
);
```

- `porter`: English stemming (running -> run)
- `unicode61`: Unicode-aware tokenization
- Content-sync: triggers on `chunks` table keep FTS5 in sync automatically

### Query Pipeline

```
Raw query: "auth middleware"
     │
     ▼
  buildMatchQuery()
     ├── Single term -> "term"*    (prefix match)
     ├── Multiple terms -> "t1" "t2" (AND by default)
     ├── Quoted -> preserved as phrase
     ├── Has FTS5 operators (AND, OR, NOT) -> passthrough
     │
     ▼
  SQL: SELECT FROM chunks_fts f
       JOIN chunks c ON c.id = f.rowid
       JOIN documents d ON d.id = c.doc_id
       JOIN repositories r ON r.id = d.repo_id
       WHERE chunks_fts MATCH ?
       ORDER BY rank (+ path boost)
       LIMIT ?
     │
     ▼
  Result: []Result with doc_id, path, language, repo_name, content, score
```

### Ranking

- FTS5 built-in BM25 ranking via `ORDER BY rank`
- Optional path boosting: if query term matches file path, rank is boosted by -10.0

## 11. Daemon Lifecycle

```
              ┌──────────────┐
              │   Start      │
              └──────┬───────┘
                     │
              ┌──────▼───────┐
              │  Load Config  │
              │  (viper)      │
              └──────┬───────┘
                     │
              ┌──────▼───────┐
              │  Open DB     │
              │  Run Migrate │
              └──────┬───────┘
                     │
              ┌──────▼───────┐
              │  Start HTTP  │
              │  API Server  │
              │  (socket/TCP)│
              └──────┬───────┘
                     │
              ┌──────▼───────┐
              │  Signal Wait │
              │  (SIGINT/    │
              │   SIGTERM)   │
              └──────┬───────┘
                     │
              ┌──────▼───────┐
              │  Shutdown    │
              │  1. Stop     │
              │     HTTP Srv │
              │  2. Close DB │
              └──────────────┘
```

## 12. Configuration Structure

### Config File (`~/.config/relith/relith.yaml`)

```yaml
core:
  data_dir: ~/.local/share/relith

daemon:
  socket: ~/.local/share/relith/relith.sock
  tcp_host: 127.0.0.1
  tcp_port: 9876

mcp:
  enabled: true
  transport: stdio
  tcp_port: 9877

indexer:
  concurrency: 4
  max_file_size: 10485760

watcher:
  enabled: true
  debounce: 1s

search:
  max_results: 100
  path_boosting: true

log:
  level: info
  format: console
  output: stderr
```

### Environment Variable Overrides

All values overridable via `RELITH_` prefix: `RELITH_LOG_LEVEL=debug`, `RELITH_DAEMON_TCP_PORT=9876`, etc.

### Precedence (lowest to highest)

1. Default values (hardcoded in `config.go`)
2. Config file
3. Environment variables
4. CLI flags (future)

## 13. Logging Strategy

### Logger Initialization

Uses zerolog (zero-allocation structured logger). Configuration via `log` section in config:
- `level`: debug | info | warn | error | fatal
- `format`: console (human-readable) | json (structured)
- `output`: stderr | stdout | file path

### Log Levels

| Level | What                         | Examples                                                    |
| ----- | ---------------------------- | ----------------------------------------------------------- |
| DEBUG | High-frequency details       | File processing, FTS5 queries                                |
| INFO  | Significant events           | Index started/completed, repo added, daemon ready            |
| WARN  | Recoverable issues           | File skipped (too large), watcher missed event               |
| ERROR | Failures requiring attention | File read error, DB connection lost                          |
| FATAL | Unrecoverable                | Config load failure, DB migration failure, port in use      |

## 14. Version Roadmap

### v0.1 - MVP (Complete)

- Go module with CLI (cobra), daemon entry point, config loading
- SQLite with FTS5, sqlc-generated queries, migrations
- Indexer with walker, language detection, chunking, hash-based change detection
- CLI commands: `repo add`, `repo list`, `index`, `search`, `status`
- REST API: health, repo CRUD, indexing trigger, search
- File watcher (fsnotify + debouncer)
- Zerolog structured logging

### v0.2.1 - MCP & Polish (Current)

- MCP server with 4 tools: search_code, get_file_content, list_repositories, get_repo_summary
- Cross-platform builds (Windows + Linux + macOS)
- Makefile with version injection via ldflags

### Planned (Post-v0.2)

- **Git history indexing**: Extract commit history using go-git
- **Vector embeddings / semantic search**: Natural language queries over code
- **Autocomplete API**: `/v1/search/suggest` endpoint
- **MCP TCP mode**: Run MCP server inside the daemon
- **SSE event stream**: Real-time indexing progress
- **Advanced query filters**: `repo:`, `path:`, `lang:` scoped search
- **Plugin system**: WASM-based plugins for custom processing
- **IDE extensions**: VS Code, JetBrains, Zed
- **CI/CD integration**: Auto-index PRs
