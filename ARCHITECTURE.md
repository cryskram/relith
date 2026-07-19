# CogniQ Architecture

Cognitive Network for Intelligent Queries

> One context. Every AI.

## Table of Contents

1. [High-Level Architecture](#1-high-level-architecture)
2. [Folder Structure](#2-folder-structure)
3. [Package Responsibilities](#3-package-responsibilities)
4. [Data Flow](#4-data-flow)
5. [Component Interactions](#5-component-interactions)
6. [Database Schema](#6-database-schema)
7. [API Design](#7-api-design)
8. [MCP Tools](#8-mcp-tools)
9. [Event Model](#9-event-model)
10. [Indexing Workflow](#10-indexing-workflow)
11. [Search Architecture](#11-search-architecture)
12. [Daemon Lifecycle](#12-daemon-lifecycle)
13. [Plugin Architecture](#13-plugin-architecture)
14. [Configuration Structure](#14-configuration-structure)
15. [Error Handling Strategy](#15-error-handling-strategy)
16. [Logging Strategy](#16-logging-strategy)
17. [Testing Strategy](#17-testing-strategy)
18. [Architecture Decision Records](#18-architecture-decision-records)
19. [Version Roadmap](#19-version-roadmap)
20. [Risks and Tradeoffs](#20-risks-and-tradeoffs)

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
         │  (internal/  │   │ (internal/   │
         │   mcp/)      │   │  api/)       │
         └──────┬───────┘   └──────┬───────┘
                │                  │
         ┌──────▼──────────────────▼───────┐
         │         Daemon (internal/daemon/)│
         │         ┌──────────────────┐     │
         │         │    Event Bus     │     │
         │         │ (internal/events)│     │
         │         └────────┬─────────┘     │
         │    ┌─────────────┼─────────────┐  │
         │    ▼             ▼             ▼  │
         │ ┌───────┐  ┌────────┐  ┌──────┐  │
         │ │Indexer│  │Watcher │  │Search│  │
         │ └───┬───┘  └────────┘  └──┬───┘  │
         │     │                     │       │
         │     └──────────┬──────────┘       │
         │                ▼                   │
         │         ┌──────────┐              │
         │         │    DB    │              │
         │         │ (SQLite) │              │
         │         └──────────┘              │
         └────────────────────────────────────┘
                      ▲
                      │ HTTP (Unix socket + TCP)
                      │
         ┌────────────┴───────────┐
         │       CLI (cmd/cogniq)  │
         │       (cobra-based)     │
         └────────────────────────┘
```

### Key Decisions

- **Single binary, dual personality**: The same binary linked as `cogniq` behaves as a CLI client; linked as `cogniqd` (or invoked with a subcommand) runs as a daemon. Docker does this — eliminates version mismatch, simplifies installation, enables atomic upgrades.
- **Unix socket for CLI-daemon**: File-system permissions as security boundary, no port conflicts, no network exposure. Windows falls back to localhost TCP with ACL-based access control.
- **HTTP between CLI and daemon**: Standard, well-understood, trivially debuggable with curl. No need for gRPC on localhost.

## 2. Folder Structure

```
cogniq/
├── cmd/
│   ├── cogniq/                    # CLI client entry point
│   │   └── main.go
│   └── cogniqd/                   # Daemon entry point (tiny)
│       └── main.go
│
├── internal/
│   ├── api/                       # REST API layer
│   │   ├── router.go              # Route registration
│   │   ├── middleware.go           # Logging, recovery, request ID
│   │   ├── response.go            # JSON response helpers
│   │   ├── errors.go              # HTTP error mapping
│   │   └── handlers/              # One file per resource group
│   │       ├── repos.go
│   │       ├── search.go
│   │       ├── files.go
│   │       └── status.go
│   │
│   ├── mcp/                       # Model Context Protocol server
│   │   ├── server.go              # MCP session lifecycle
│   │   ├── tools.go               # Tool definitions + handlers
│   │   ├── resources.go           # Resource definitions
│   │   └── transport.go           # stdio / TCP transport
│   │
│   ├── indexer/                   # Core indexing engine
│   │   ├── indexer.go             # Orchestrator
│   │   ├── walker.go              # Directory walk + .gitignore
│   │   ├── git.go                 # Git history extraction
│   │   ├── analyzer.go            # Language/MIME detection
│   │   └── sink.go                # Batch DB writer
│   │
│   ├── watcher/                   # Filesystem event watcher
│   │   ├── watcher.go             # fsnotify wrapper
│   │   └── debouncer.go           # Coalesce rapid events
│   │
│   ├── db/                        # Data access layer
│   │   ├── db.go                  # Connection pool, migrations
│   │   ├── models.go              # Generated types (sqlc)
│   │   ├── repos.sql.go           # Generated queries (sqlc)
│   │   ├── files.sql.go
│   │   ├── commits.sql.go
│   │   └── search.sql.go
│   │
│   ├── search/                    # Search abstraction over FTS5
│   │   ├── search.go
│   │   ├── query.go               # Query parser
│   │   └── ranking.go             # BM25 + custom scoring
│   │
│   ├── daemon/                    # Orchestrator
│   │   └── daemon.go              # Start/stop/health
│   │
│   ├── config/                    # Configuration
│   │   └── config.go              # Viper setup, defaults
│   │
│   └── events/                    # In-process event bus
│       └── bus.go
│
├── pkg/
│   ├── types/                     # Shared types (no deps)
│   │   └── types.go
│   └── errors/                    # Sentinel errors
│       └── errors.go
│
├── sql/
│   ├── migrations/                # SQL migration files
│   │   ├── 001_initial.up.sql
│   │   └── 001_initial.down.sql
│   └── queries/                   # sqlc query definitions
│       ├── repos.sql
│       ├── files.sql
│       ├── commits.sql
│       └── search.sql
│
├── api/
│   └── openapi.yaml               # Single source of truth for API
│
├── docs/
│   └── adr/                       # Architecture Decision Records
│       ├── 001-use-go.md
│       ├── 002-single-binary.md
│       ├── 003-sqlite-fts5.md
│       ├── 004-local-first.md
│       ├── 005-unix-socket-http.md
│       ├── 006-viper-config.md
│       ├── 007-sqlc.md
│       ├── 008-in-process-events.md
│       ├── 009-mcp-stdio-tcp.md
│       ├── 010-go-git.md
│       ├── 011-wasm-plugins.md
│       ├── 012-zerolog.md
│       ├── 013-fsnotify-rescan.md
│       ├── 014-uuid-v7.md
│       └── 015-sentinel-errors.md
│
├── tests/
│   ├── fixtures/
│   │   ├── small-repo/
│   │   ├── multi-lang-repo/
│   │   ├── large-file-repo/
│   │   └── binary-repo/
│   ├── e2e/
│   │   └── daemon_test.go
│   ├── testutil/
│   │   ├── db.go
│   │   ├── daemon.go
│   │   └── repo.go
│   └── integration/
│       ├── api_repos_test.go
│       ├── api_search_test.go
│       └── indexer_test.go
│
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yaml
└── README.md
```

### Why this structure

- **`internal/`**: Go's `internal` package visibility ensures these packages cannot be imported by external consumers. The public API surface is strictly the CLI commands, REST API, and MCP protocol.
- **`pkg/`**: Only types that must cross the internal boundary. Goal is <200 lines. No external dependencies.
- **`sql/` separate from `db/`**: The `sql/` directory holds the source of truth (migration SQL, sqlc queries). `internal/db/` holds generated Go code. A DBA can review the actual SQL without wading through Go.
- **`cmd/`**: Thin — parse flags, create config, launch daemon or dispatch CLI command. Zero business logic.

---

## 3. Package Responsibilities

| Package            | Responsibility                                                       | Dependencies (internal)                              |
| ------------------ | -------------------------------------------------------------------- | ---------------------------------------------------- |
| `cmd/cogniq`       | Parse CLI flags (cobra), build HTTP requests to daemon               | `pkg/types`, `pkg/errors`                            |
| `cmd/cogniqd`      | Parse flags, load config, instantiate daemon, block on signal        | `internal/daemon`, `internal/config`                 |
| `internal/api`     | HTTP routing, request validation, JSON marshaling, error formatting  | `internal/db`, `internal/search`                     |
| `internal/mcp`     | JSON-RPC over stdio/TCP, tool/resource registration, dispatch        | `internal/db`, `internal/search`                     |
| `internal/indexer` | Walk filesystems, extract git history, detect languages, batch-write | `internal/db`, `internal/watcher`, `internal/events` |
| `internal/watcher` | Wrap fsnotify, debounce, filter, publish events                      | `internal/events`                                    |
| `internal/db`      | Connection lifecycle, migration runner, sqlc-generated methods       | None (sqlite driver only)                            |
| `internal/search`  | FTS5 query construction, snippet extraction, result ranking          | `internal/db`                                        |
| `internal/daemon`  | Component wiring, graceful shutdown, signal handling, lock file      | All `internal/*`                                     |
| `internal/config`  | Load/merge config from file + env + flags, validate, defaults        | viper                                                |
| `internal/events`  | Typed event bus with sync/async delivery                             | None (pure channels)                                 |
| `pkg/types`        | `FileInfo`, `RepoInfo`, `SearchResult`, `CommitInfo` structs         | None                                                 |
| `pkg/errors`       | Sentinel errors, error codes                                         | None                                                 |

### Dependency Rule

Dependencies flow **inward**. `internal/daemon` depends on everything. `internal/db` depends on nothing (except the sqlite driver). `pkg/` depends on nothing. This prevents circular imports and makes each package independently testable.

---

## 4. Data Flow

### A. Adding and Indexing a Repository

```
User: cogniq repo add /path/to/project

CLI ── POST /v1/repos ──▶ Daemon
                              │
                              ▼
                          API Handler
                              │
                              ▼
                          DB: INSERT INTO repositories
                              │
                              ▼
                          Event Bus: RepoAdded{Path: "/path/to/project"}
                              │
                              ▼
                          Indexer (subscribes to RepoAdded)
                              │
                      ┌───────┴────────┐
                      ▼                 ▼
                  Walker.go        git.go
                      │                 │
                      ▼                 ▼
                  For each file:    For each commit:
                  - hash             - SHA, author, msg
                  - detect language  - store in git_commits
                  - store in files
                  - insert into FTS5
                      │                 │
                      └────────┬────────┘
                               ▼
                           DB: Batch INSERT (transaction)
                               │
                               ▼
                           Event Bus: IndexCompleted{RepoID}
```

### B. Search Query

```
AI Tool ── MCP "search_code" ──▶ MCP Server
                                      │
                                      ▼
                                  search.go: ParseQuery("auth middleware")
                                      │
                                      ▼
                                  DB: SELECT FROM fts_files WHERE fts MATCH ?
                                      │
                                      ▼
                                  Rank results (BM25 + path boost)
                                      │
                                      ▼
                                  Return []SearchResult to MCP client
```

### C. File Change Detection

```
Filesystem change (editor saves)
       │
       ▼
   fsnotify event
       │
       ▼
   Debouncer (coalesces within 1s window)
       │
       ▼
   Event Bus: FileChanged{RepoID, Path}
       │
       ▼
   Indexer: Re-index changed file
       - compute new hash
       - if different, update files + FTS5
       - emit FileIndexed event
```

## 5. Component Interactions

Three interaction patterns:

### Pattern 1: Request-Response (CLI → API → DB)

```
CLI → HTTP → API Handler → DB → JSON response → CLI
```

Used for: adding repos, listing repos, triggering re-index, health checks.

### Pattern 2: Event-Driven (Watcher → EventBus → Indexer → DB)

```
Watcher → EventBus → Indexer → DB → EventBus(IndexCompleted)
```

Used for: file changes, initial indexing progress, periodic maintenance.

### Pattern 3: MCP Request (AI Tool → MCP → Search/DB)

```
AI Tool (JSON-RPC) → MCP Server → Search/DB → JSON-RPC response
```

Used for: all AI interactions. MCP server reuses the same search and DB packages.

### Why not...

- **gRPC?** MCP uses JSON-RPC, so gRPC would translate to JSON-RPC for no benefit. REST is simpler for CLI and third-party consumers. MCP for AI tools is non-negotiable.
- **GraphQL?** Overkill for MVP. The data model is simple (repos, files, commits, search). GraphQL adds complexity with no benefit at this scale.

## 6. Database Schema

```sql
-- Enable WAL mode for concurrent reads
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;

-- Tracked repositories
CREATE TABLE repositories (
    id              TEXT PRIMARY KEY,        -- UUID v7 (time-ordered)
    path            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,            -- Derived from path
    remote_url      TEXT,                     -- From git remote
    status          TEXT NOT NULL DEFAULT 'pending',
                    -- pending | indexing | ready | failed
    last_indexed_at TEXT,                     -- ISO 8601
    file_count      INTEGER DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

-- File metadata
CREATE TABLE files (
    id              TEXT PRIMARY KEY,         -- UUID v7
    repo_id         TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    path            TEXT NOT NULL,            -- Relative to repo root
    size            INTEGER NOT NULL,
    hash            TEXT NOT NULL,            -- SHA-256 hex
    mod_time        TEXT NOT NULL,            -- ISO 8601
    mime_type       TEXT,
    language        TEXT,                     -- Detected programming language
    created_at      TEXT NOT NULL,
    UNIQUE(repo_id, path)
);
CREATE INDEX idx_files_repo ON files(repo_id);
CREATE INDEX idx_files_language ON files(language);

-- Git commits
CREATE TABLE git_commits (
    id              TEXT PRIMARY KEY,         -- UUID v7
    repo_id         TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    sha             TEXT NOT NULL,
    author_name     TEXT NOT NULL,
    author_email    TEXT NOT NULL,
    message         TEXT NOT NULL,
    committed_at    TEXT NOT NULL,            -- ISO 8601
    files_changed   INTEGER DEFAULT 0,
    UNIQUE(repo_id, sha)
);
CREATE INDEX idx_commits_repo ON git_commits(repo_id);
CREATE INDEX idx_commits_author ON git_commits(author_email);
CREATE INDEX idx_commits_date ON git_commits(committed_at);

-- Full-text search index (FTS5)
CREATE VIRTUAL TABLE fts_files USING fts5(
    file_id     UNINDEXED,                     -- References files.id
    repo_id     UNINDEXED,
    path,
    content,
    language    UNINDEXED,
    tokenize='porter unicode61',
    prefix='2,3'
);

-- Polymorphic tags (future: for tagging repos, files, commits)
CREATE TABLE tags (
    id              TEXT PRIMARY KEY,
    taggable_id     TEXT NOT NULL,
    taggable_type   TEXT NOT NULL,            -- 'repo' | 'file' | 'commit'
    tag             TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    UNIQUE(taggable_id, taggable_type, tag)
);
CREATE INDEX idx_tags_tag ON tags(tag);

-- Internal key-value store
CREATE TABLE metadata (
    key     TEXT PRIMARY KEY,
    value   TEXT NOT NULL
);

-- Event log (debugging, future sync)
CREATE TABLE event_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type  TEXT NOT NULL,
    payload     TEXT,                          -- JSON
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### Schema Decisions

- **UUID v7**: Time-ordered UUIDs cluster well in SQLite B-trees, unlike random UUID v4. No need for a centralized sequence generator.
- **TEXT timestamps (ISO 8601)**: Lexicographically sortable, human-readable, no y2k38 problem. Performance difference vs INTEGER is negligible at this scale.
- **Standalone FTS5 virtual table** (not content-sync): Simpler to control explicitly. Can migrate to content-sync later.
- **`event_log` table**: Append-only event log enables debugging, crash recovery, and future incremental cloud sync without additional infrastructure.

## 7. API Design

### Conventions

- Base path: `/v1/`
- JSON request/response bodies
- `Content-Type: application/json`
- Standard error envelope: `{"error": {"code": "NOT_FOUND", "message": "..."}}`
- Pagination: `?offset=0&limit=50`
- Request IDs: `X-Request-ID` header (auto-generated if not provided)

### Endpoints

```
# Lifecycle
GET    /v1/health                     → {"status":"ok","version":"0.1.0"}
GET    /v1/status                     → {"daemon":{"uptime":"...","indexing":false},"repos":{...}}

# Repositories
GET    /v1/repos                      → {"repos":[...], "total": N}
POST   /v1/repos                      → {"repo": {...}}  (body: {"path": "/..."})
GET    /v1/repos/:id                  → {"repo": {...}}
DELETE /v1/repos/:id                  → 204 No Content
POST   /v1/repos/:id/index            → {"status": "indexing_started"}

# Files
GET    /v1/repos/:id/files            → {"files": [...], "total": N}  (?language=go&offset=0&limit=50)
GET    /v1/repos/:id/files/:file_id   → {"file": {...}, "content": "..."}

# Commits
GET    /v1/repos/:id/commits          → {"commits": [...], "total": N}  (?author=&since=&limit=50)
GET    /v1/repos/:id/commits/:sha     → {"commit": {...}}  (future: +diff)

# Search
GET    /v1/search?q=<query>           → {"results": [...], "total": N, "took": "12ms"}
         &repo=<repo_id>
         &language=go
         &path=src/auth/
         &limit=20
         &offset=0

GET    /v1/search/suggest?q=<prefix>  → {"suggestions": ["auth", "authenticate", ...]}

# Events (Server-Sent Events)
GET    /v1/events                     → SSE stream of indexer events
```

### Why offset pagination, not cursor?

Offset pagination is simpler and sufficient for MVP. Data volumes are moderate (thousands of files, not millions). Cursor-based pagination can be added later for scroll-based UX patterns.

### Why SSE, not WebSocket?

SSE is simpler for unidirectional server-to-client streaming. Every HTTP library supports it. WebSocket adds complexity (bidirectional, framing, reconnection logic) that we don't need — the daemon pushes events, clients don't send data over the event stream.

## 8. MCP Tools

The MCP server follows the [Model Context Protocol](https://modelcontextprotocol.io) specification.

### Tools

| Tool Name             | Description                                      | Parameters                                                                                  |
| --------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| `search_code`         | Full-text search across indexed repos            | `query` (required), `repo_id` (optional), `language` (optional), `max_results` (default 20) |
| `get_file_content`    | Retrieve a file's content by repo + path         | `repo_id` (required), `path` (required)                                                     |
| `list_repositories`   | List all tracked repos                           | -                                                                                           |
| `get_repo_summary`    | Get language breakdown, file count, last indexed | `repo_id` (required)                                                                        |
| `get_git_history`     | Query commit history                             | `repo_id` (required), `path` (optional), `limit` (default 50), `author` (optional)          |
| `get_recent_activity` | Recently changed files across all repos          | `since` (ISO 8601, default 24h), `limit` (default 20)                                       |

### Resources

```
repo://<repo_id>                  → Repository metadata + file tree
repo://<repo_id>/tree/<path>      → Directory listing
repo://<repo_id>/file/<path>      → File content + metadata
repo://<repo_id>/commits          → Recent commits
```

### Prompts (future)

```
code-review <repo_id> <file_path>  → Structured context for code review
```

### Transport

- **stdio** (default): AI assistant spawns MCP server as subprocess. Simplest integration, no port management.
- **TCP** (daemon mode): For persistent connections. MCP server runs inside the daemon on port 9877.

### Why tools over resources?

Tools model actions ("search", "get") which is more natural for AI assistants than hierarchical resource URIs. Resources are secondary for direct access.

## 9. Event Model

```go
package events

import (
    "context"
    "sync"
    "time"
)

type Type int

const (
    RepoAdded              Type = iota
    RepoRemoved
    RepoIndexingStarted
    RepoIndexingProgress
    RepoIndexingCompleted
    RepoIndexingFailed
    FileChanged
    FileAdded
    FileRemoved
    DaemonStarted
    DaemonShuttingDown
    ConfigReloaded
)

type Event struct {
    Type      Type
    Timestamp time.Time
    Payload   any
}

type Bus struct {
    subscribers map[Type][]chan Event
    mu          sync.RWMutex
}
```

### Key Design

- **Channels, not callbacks**: Channels are composable with `select`, cancellable via context, and trivially debounceable. Callbacks would need their own lifecycle management.
- **Synchronous publish by default**: Provides ordering guarantees. Subscribers that need non-blocking delivery use goroutines internally.
- **`PublishAsync` for fire-and-forget**: For high-frequency events (file change notifications).
- **No external message broker**: Single-machine daemon. Channels handle hundreds of thousands of events per second in-process. Adding NATS, RabbitMQ, or Redis adds deployment complexity and resource usage for zero benefit at this scale.

---

## 10. Indexing Workflow

### Initial Index (Full Pass)

```
1. Receive RepoAdded event (or manual re-index request)
2. Set repo status → "indexing"
3. Walk directory tree:
   a. Parse .gitignore (and .ignore, .fdignore)
   b. Skip excluded patterns (node_modules, .git, __pycache__, vendor)
   c. Skip binary files (>10MB or detected as binary by magic bytes)
   d. For each qualifying file:
      i.   Compute SHA-256 hash
      ii.  Read mod_time, size
      iii. Detect MIME type (net/http detection + extension map)
      iv.  Detect programming language (extension + shebang)
      v.   Read content for FTS5 (first 10MB only)
4. Extract git history (for git repos):
   a. Open repo with go-git
   b. Iterate commits (newest-first, limited to max_commits config)
   c. For each commit: SHA, author, message, timestamp, files changed
   d. Store in git_commits table (batch insert)
5. Batch INSERT all files + FTS5 content in a single transaction
   (batches of 500 files to keep transactions manageable)
6. Update repo: status="ready", file_count, last_indexed_at
7. Publish RepoIndexingCompleted event
```

### Incremental Index (File Change)

```
1. Receive FileChanged event from watcher
2. Stat the file to get current mod_time + size
3. If file no longer exists → handle as deletion (remove from DB + FTS5)
4. Compute SHA-256 hash
5. Look up existing hash in DB for this file
6. If hash matches → skip (only metadata updated, not content)
7. If hash differs:
   a. Update files table row
   b. Update FTS5 content
   c. Update repo's file_count if needed
```

### Git Re-index Strategy

- **Initial index**: Full git history walk (paginated, batch-inserted)
- **Incremental**: Do NOT re-scan git history on file change
- **Manual trigger**: `POST /v1/repos/:id/index?git=true` for full git re-index
- **Periodic sync**: New commits since `last_indexed_at` using `git log --after=<timestamp>`

### Why not full git history on every change?

A large repo like Linux has 1M+ commits. Walking git history takes minutes. The most common use case is "I saved a file" — handle that in milliseconds, not minutes.

---

## 11. Search Architecture

### FTS5 Virtual Table

```sql
CREATE VIRTUAL TABLE fts_files USING fts5(
    file_id, repo_id, path, content, language,
    tokenize='porter unicode61',
    prefix='2,3'
);
```

- **porter**: English stemming (running → run)
- **unicode61**: Unicode-aware tokenization (handles non-ASCII identifiers)
- **prefix='2,3'**: Enables prefix queries for autocomplete

### Query Pipeline

```
Raw query: "auth middleware sql injection"
     │
     ▼
  QueryParser
     ├── Remove special characters
     ├── Tokenize
     ├── Detect quoted phrases ("exact match")
     ├── Detect field filters (lang:go, repo:myapp)
     └── Build FTS5 MATCH string
     │
     ▼
  FTS5 MATCH query: "auth AND middleware AND sql AND injection"
     │
     ▼
  SQL: SELECT file_id, path,
       snippet(fts_files, 2, '<b>', '</b>', '...', 64)
       FROM fts_files
       WHERE fts_files MATCH ?
         AND repo_id = ?           -- optional filter
         AND language = ?          -- optional filter
       ORDER BY rank               -- BM25 ranking
       LIMIT ? OFFSET ?
     │
     ▼
  ResultProcessor
     ├── Parse snippet (highlighted excerpts)
     ├── Enrich with file metadata (JOIN with files table)
     ├── Rank with optional path boosting (src/ > test/)
     └── Return []SearchResult
```

### Ranking Strategy

FTS5's default BM25 is good. Improvements (implemented post-query):

- **Path boosting**: `src/` files score higher than `test/`
- **Language boosting** (future): Boost files matching user's primary language
- **Recency boosting** (future): Recently modified files score higher

### Autocomplete / Suggest

```sql
SELECT DISTINCT term FROM fts_files('{prefix}*')
ORDER BY bm25(fts_files, :query)
LIMIT 10
```

FTS5 supports prefix queries natively. For `"auth"`, search `"auth*"` and return top matching terms.

### Why FTS5 instead of Bleve or Meilisearch?

**Zero dependencies.** FTS5 is compiled into SQLite. No separate process, no HTTP calls, no NPM/Java runtime. For indexing thousands (not millions) of files, FTS5 delivers single-digit millisecond queries. Meilisearch is great but requires a separate binary — antithetical to "single binary, lightweight."

**Tradeoff**: FTS5's ranking is less sophisticated. No typo tolerance, no learning-to-rank. Acceptable for MVP. If users demand typo-tolerant search, we can add Levenshtein-based post-filtering or consider Tantivy (via CGo) in a future version.

---

## 12. Daemon Lifecycle

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
                │  Acquire     │
                │  Lock File   │
                └──────┬───────┘
                       │
                ┌──────▼───────┐
                │  Start       │
                │  Components  │
                │  ┌─────────┐ │
                │  │ HTTP    │ │
                │  │ Server  │ │
                │  ├─────────┤ │
                │  │ MCP     │ │
                │  │ Server  │ │
                │  ├─────────┤ │
                │  │ Watcher │ │
                │  └─────────┘ │
                └──────┬───────┘
                       │
                ┌──────▼───────┐
                │  Main Loop   │
                │  ┌─────────┐ │
                │  │Signal   │ │
                │  │Handling │ │
                │  ├─────────┤ │
                │  │Periodic │ │
                │  │Tasks    │ │
                │  └─────────┘ │
                └──────┬───────┘
                       │
           SIGTERM/SIGINT
                       │
                ┌──────▼───────┐
                │  Graceful    │
                │  Shutdown    │
                │  1. Stop MCP │
                │  2. Stop HTTP│
                │  3. Stop     │
                │     Watcher  │
                │  4. Flush    │
                │     Events   │
                │  5. Close DB │
                │  6. Release  │
                │     Lock     │
                └──────────────┘
```

### Signal Handling

| Signal  | Action                                      |
| ------- | ------------------------------------------- |
| SIGTERM | Graceful shutdown (systemd, orchestrators)  |
| SIGINT  | Graceful shutdown (Ctrl+C)                  |
| SIGHUP  | Reload config (re-read file, apply changes) |
| SIGUSR1 | Trigger full re-index of all repos          |
| SIGUSR2 | Toggle debug logging                        |

### Lock File

- Location: `$COGNIQ_DATA_DIR/cogniq.lock` (default `~/.local/share/cogniq/cogniq.lock`)
- Contains daemon PID
- On startup: if lock exists, check if PID is alive; if alive → exit with error; if dead → remove stale lock and proceed
- Linux: `flock` system call (atomic, auto-released on crash)
- Windows: `CreateFile` with `FILE_FLAG_DELETE_ON_CLOSE`

---

## 13. Plugin Architecture

### Design for Plugins Without Building Them

The MVP defines the interfaces but does NOT implement a plugin loader. This prevents premature complexity while ensuring the architecture can accommodate plugins later.

```go
package plugin

type Hook int

const (
    HookBeforeFileIndex  Hook = iota
    HookAfterFileIndex
    HookBeforeSearch
    HookAfterSearch
    HookBeforeRepoAdd
    HookAfterRepoAdd
)

type Plugin interface {
    Name() string
    Version() string
    Handle(ctx context.Context, hook Hook, data any) (any, error)
}

type Registry struct {
    plugins map[string]Plugin
}

func NewRegistry() *Registry {
    return &Registry{plugins: make(map[string]Plugin)}
}

func (r *Registry) Register(p Plugin) { ... }
func (r *Registry) Execute(ctx context.Context, hook Hook, data any) (any, error) { ... }
```

### MVP Behavior

The daemon creates a `Registry` and passes it to indexer, search, and API layers. The registry is empty and `Execute` is a no-op.

### Future Plugin Loading (Post-v1.0)

1. Scan `~/.local/share/cogniq/plugins/` directory
2. Load WASM modules (sandboxed, memory-safe, cross-platform via `wazero`)
3. Register loaded plugins with the registry
4. Call `Execute` at each hook point

### Why WASM for plugins?

- **Sandboxed**: Memory-safe, CPU/memory limits, cannot crash host
- **Cross-platform**: Not Linux-only like Go's `plugin` package
- **Language-agnostic**: Plugins can be written in Go, Rust, C, etc.
- **Version-isolated**: No Go version lock-in

### Alternative: External Process Plugins

For plugins that need heavy computation or network access that WASM can't provide, communicate via subprocess + stdin/stdout JSON-RPC (same pattern as MCP).

---

## 14. Configuration Structure

### Config File (`~/.config/cogniq/config.yaml`)

```yaml
# CogniQ Configuration v1

core:
  data_dir: ~/.local/share/cogniq # Database, lock file, plugins

daemon:
  socket: ~/.local/share/cogniq/cogniq.sock
  tcp_host: 127.0.0.1
  tcp_port: 9876

mcp:
  enabled: true
  transport: stdio # stdio | tcp
  tcp_port: 9877

indexer:
  concurrency: 4
  max_file_size: 10485760 # 10MB
  max_commits: 10000
  exclude_patterns: []

watcher:
  enabled: true
  debounce: 1s

search:
  max_results: 100
  path_boosting: true

log:
  level: info # debug | info | warn | error
  format: console # console | json
  output: stderr # stdout | stderr | path/to/file
```

### Environment Variable Overrides

All values overridable via `COGNIQ_` prefix:

```bash
COGNIQ_LOG_LEVEL=debug
COGNIQ_DAEMON_TCP_PORT=9876
COGNIQ_INDEXER_CONCURRENCY=8
COGNIQ_CORE_DATA_DIR=/custom/path
```

### Precedence (lowest → highest)

1. Default values (hardcoded)
2. Config file
3. Environment variables
4. CLI flags

### Why viper?

Viper handles all three sources with a unified API. Supports YAML, TOML, JSON, env variable binding, flag binding, config file watching (for SIGHUP reload), and nested key access. It's the de facto standard for Go configuration.

---

## 15. Error Handling Strategy

### Error Types

```go
package errors

type Code string

const (
    NotFound          Code = "NOT_FOUND"
    AlreadyExists     Code = "ALREADY_EXISTS"
    InvalidInput      Code = "INVALID_INPUT"
    IndexingFailed    Code = "INDEXING_FAILED"
    RepoNotTracked    Code = "REPO_NOT_TRACKED"
    Internal          Code = "INTERNAL_ERROR"
    DaemonNotRunning  Code = "DAEMON_NOT_RUNNING"
)

type CogniQError struct {
    Code    Code   `json:"code"`
    Message string `json:"message"`
    Err     error  `json:"-"`   // Wrapped error, not serialized
}
```

### Principles

1. **Sentinel errors for expected failures**: `ErrRepoNotTracked`, `ErrNotFound`
2. **Wrapped errors for unexpected failures**: `fmt.Errorf("indexing %s: %w", path, err)`
3. **API translates to HTTP status codes**: `NotFound` → 404, `AlreadyExists` → 409, `InvalidInput` → 400, `Internal` → 500
4. **Never expose internals in API responses**: No file paths, stack traces, or SQL queries
5. **Always log the full error internally**: API handler logs wrapped error, returns sanitized version
6. **Graceful degradation**: One file fails → log and continue; don't abort entire index
7. **Retry transient failures**: SQLITE_BUSY (retry 3x with backoff), file locks (retry once)

### HTTP Error Response

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Repository not found: my-project"
  },
  "request_id": "abc-123-def"
}
```

### CLI Error Presentation

```
$ cogniq search --query "auth"
Error: Daemon is not running
Hint: Start the daemon with: cogniqd

$ cogniq repo add /nonexistent
Error: Invalid input: path does not exist: /nonexistent
```

---

## 16. Logging Strategy

### Logger Initialization

```go
func ConfigureLogging(cfg LogConfig) {
    zerolog.TimeFieldFormat = time.RFC3339Nano
    zerolog.DurationFieldInteger = true

    var output io.Writer = os.Stderr
    if cfg.Output != "" && cfg.Output != "stderr" {
        f, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err == nil {
            output = f
        }
    }

    if cfg.Format == "json" {
        log.Logger = zerolog.New(output).With().Timestamp().Logger()
    } else {
        log.Logger = zerolog.New(
            zerolog.ConsoleWriter{Out: output, TimeFormat: "15:04:05"},
        ).With().Timestamp().Logger()
    }

    level, _ := zerolog.ParseLevel(cfg.Level)
    zerolog.SetGlobalLevel(level)
}
```

### Per-Component Logging

Each component creates a child logger with context:

```go
var log = zerolog.New(os.Stderr).With().
    Str("component", "indexer").
    Logger()

func (idx *Indexer) indexRepo(ctx context.Context, repo types.RepoInfo) error {
    l := log.With().
        Str("repo_id", repo.ID).
        Str("repo_path", repo.Path).
        Logger()
    l.Info().Msg("starting index")
    // ...
    l.Info().Int("files", count).Dur("took", elapsed).Msg("index complete")
}
```

### Log Levels

| Level | What                         | Examples                                                    |
| ----- | ---------------------------- | ----------------------------------------------------------- |
| DEBUG | High-frequency details       | File processing, event bus messages, SQL queries (dev only) |
| INFO  | Significant events           | Index started/completed, repo added, daemon ready           |
| WARN  | Recoverable issues           | File skipped (too large), watcher missed event              |
| ERROR | Failures requiring attention | File read error, DB connection lost                         |
| FATAL | Unrecoverable                | Config load failure, DB migration failure, port in use      |

### Correlation IDs

Each HTTP request gets a `X-Request-ID` header (auto-generated if not provided), propagated through the context. Critical for debugging across component boundaries.

### Why zerolog?

Fastest structured logger for Go. Zero allocations in the hot path. Supports leveled logging, context fields, multiple outputs. JSON for production (structured, machine-parseable), console for development.

---

## 17. Testing Strategy

### Test Pyramid

```
        ┌──────────┐
        │   E2E    │  ← Few: full daemon lifecycle (5-10)
        │          │
       ┌┴──────────┴┐
       │Integration │  ← Some: API + DB tests (20-40)
       │            │
      ┌┴─────────────┴┐
      │   Unit Tests   │  ← Many: isolated packages (200+)
      └────────────────┘
```

### Unit Tests

Every package has unit tests. Target >80% coverage for core packages (indexer, search, config, events).

```go
func TestBus_PublishSubscribe(t *testing.T) {
    bus := New()
    ch := bus.Subscribe(RepoAdded, RepoRemoved)

    bus.Publish(context.Background(), Event{Type: RepoAdded, Payload: "repo1"})
    bus.Publish(context.Background(), Event{Type: FileChanged, Payload: "file1"})

    select {
    case evt := <-ch:
        assert.Equal(t, RepoAdded, evt.Type)
    case <-time.After(time.Second):
        t.Fatal("timeout")
    }
}
```

### Integration Tests

Test API + DB together with in-memory SQLite:

```go
func TestReposHandler_AddAndList(t *testing.T) {
    db := testutil.NewTestDB(t)           // in-memory SQLite, migrations applied
    router := api.NewRouter(db)
    ts := httptest.NewServer(router)
    defer ts.Close()

    resp, err := http.Post(ts.URL+"/v1/repos", "application/json",
        strings.NewReader(`{"path": "/tmp/test-repo"}`))
    assert.NoError(t, err)
    assert.Equal(t, 201, resp.StatusCode)
}
```

### E2E Tests

Test daemon as black box:

```go
func TestDaemon_AddRepoAndSearch(t *testing.T) {
    repoDir := testutil.CreateTestRepo(t)  // creates a small git repo
    d := testutil.StartDaemon(t, testutil.WithDataDir(t.TempDir()))
    defer d.Stop()

    output, err := testutil.RunCLI(t, "repo", "add", repoDir)
    assert.NoError(t, err)
    assert.Contains(t, output, "added")

    testutil.WaitForIndex(t, d, time.Second*5)

    output, err = testutil.RunCLI(t, "search", "--query", "main")
    assert.NoError(t, err)
    assert.Contains(t, output, "main.go")
}
```

### Test Fixtures

```
tests/
├── fixtures/
│   ├── small-repo/          # Minimal git repo with Go files
│   ├── multi-lang-repo/     # Python, JS, Rust files
│   ├── large-file-repo/     # Contains a file >10MB (should be skipped)
│   └── binary-repo/         # Contains binary files (should be skipped)
├── e2e/
│   └── daemon_test.go
├── testutil/
│   ├── db.go                # NewTestDB, RunMigrations
│   ├── daemon.go            # StartDaemon, StopDaemon, WaitForIndex
│   └── repo.go              # CreateTestRepo, CreateTestFile
└── integration/
    ├── api_repos_test.go
    ├── api_search_test.go
    └── indexer_test.go
```

### Additional Testing

- **Fuzz testing**: Search query parser (handle all special characters, injections)
- **Benchmarks**: Search latency, indexing throughput, file hashing
- **Race detection**: Run tests with `-race` in CI

---

## 18. Architecture Decision Records

These ADRs should be written as individual files in `docs/adr/`.

| ADR | Title           | Decision                   | Key Alternatives                                                 |
| --- | --------------- | -------------------------- | ---------------------------------------------------------------- |
| 001 | Language        | Go                         | Rust (more perf, less ecosystem), Python (too slow)              |
| 002 | Single Binary   | CLI + daemon in one binary | Separate binaries (version sync issues)                          |
| 003 | Database        | SQLite with FTS5           | Bleve (more features, more deps), Meilisearch (external process) |
| 004 | Local-First     | No cloud, no auth          | Cloud-sync-first (adds complexity, privacy concerns)             |
| 005 | Communication   | Unix socket + HTTP         | gRPC (overkill), named pipes (platform issues)                   |
| 006 | Config          | Viper + YAML + Env         | envconfig (no file support), koanf (less mature)                 |
| 007 | SQL Access      | sqlc (type-safe SQL)       | GORM (magic, runtime overhead), sqlx (string-based)              |
| 008 | Event Bus       | In-process channels        | NATS (external dep), Redis (external dep)                        |
| 009 | MCP             | stdio + TCP transport      | Custom protocol (no ecosystem)                                   |
| 010 | Git Access      | go-git                     | Git CLI (version dep, slower), libgit2 (C dep)                   |
| 011 | Plugin System   | WASM (future)              | Go plugins (Linux-only), Lua (limited)                           |
| 012 | Logging         | zerolog                    | zap (slower but more features), logrus (deprecated)              |
| 013 | File Watching   | fsnotify + periodic rescan | inotify directly (platform specific), polling (slow)             |
| 014 | UUID Generation | UUID v7                    | Sequential IDs (collision risk), UUID v4 (bad B-tree perf)       |
| 015 | Error Handling  | Sentinel + wrapped errors  | Panics (not Go idiom), checked exceptions                        |

### ADR Format

```markdown
# ADR-NNN: Title

## Status

Accepted

## Context

Why this decision is needed. What problem does it solve? What constraints exist?

## Decision

What we decided. Be specific.

## Consequences

What becomes easier, what becomes harder. Tradeoffs.

## Alternatives Considered

List of alternatives with brief reasoning for rejection.

## References

Links to relevant docs, discussions, issues.
```

---

## 19. Version Roadmap

### v0.1 — MVP (Weeks 1–8)

**Goal**: A developer can install CogniQ, point it at a project, and search its code from any MCP-compatible AI assistant.

| Week | Milestone | Deliverables                                                             |
| ---- | --------- | ------------------------------------------------------------------------ |
| 1–2  | Skeleton  | Go module, CLI (cobra), daemon entry point, config loading               |
| 3–4  | Database  | SQLite schema, migrations, sqlc setup, basic CRUD for repos/files        |
| 5–6  | Indexing  | Walker, .gitignore, hash-based change detection, FTS5 content insertion  |
| 7    | API + MCP | REST endpoints (repos, search, files), MCP tools (search_code, get_file) |
| 8    | Polish    | File watcher, Makefile, docs, basic tests, Linux dev guide               |

**Ships**: `cogniq repo add`, `cogniq search`, MCP integration with Cursor/OpenCode/Claude Code. Daemon runs in foreground (`cogniqd`). Linux only.

### v0.2 — Foundation (Weeks 9–16)

| Feature                              | Rationale                                  |
| ------------------------------------ | ------------------------------------------ |
| Git history indexing                 | Context without git history is incomplete  |
| macOS + Windows support              | Cross-platform is a core principle         |
| Autocomplete API (`/search/suggest`) | AI tools need suggestions, not just search |
| SSE event stream                     | CLI shows indexing progress                |
| Performance optimization             | Profiling, batch sizes, memory tuning      |
| Homebrew formula                     | Easy macOS installation                    |
| CI/CD (goreleaser)                   | Automated builds for all platforms         |
| systemd service file                 | Production daemon management               |

### v0.3 — Ecosystem (Weeks 17–24)

| Feature                                | Rationale                           |
| -------------------------------------- | ----------------------------------- |
| Language detection improvements        | Better shebang + extension parsing  |
| Query language (scoped, filtered)      | `repo:myapp path:src/auth lang:go`  |
| Plugin interface (defined, not loaded) | API stability guarantee             |
| Refined ranking                        | BM25 tuning, path boosting, recency |
| Benchmark suite                        | Prevent regressions                 |
| VS Code extension (basic)              | Broader IDE support                 |

### v1.0 — Production (Weeks 25–36)

| Feature                 | Rationale                            |
| ----------------------- | ------------------------------------ |
| Full test coverage      | >80%, property-based tests           |
| Security audit          | Sandboxing, no privilege escalation  |
| Windows MSI installer   | Production-quality Windows           |
| Plugin loader (WASM)    | Extensibility without forking        |
| Performance targets met | <100MB idle, <10ms p99 search        |
| Documentation site      | API docs, MCP integration guide      |
| Migration system        | Schema versioning, seamless upgrades |

### Post-v1.0 (Future — Not Planned)

- Vector embeddings / semantic search
- Cloud sync (opt-in, E2E encrypted)
- Team collaboration (shared context)
- IDE plugins for VS Code, JetBrains, Zed
- CI/CD pipeline integration (auto-index PRs)
- Dash/Zeal-style docset import

---

## 20. Risks and Tradeoffs

### Critical Risks

| #   | Risk                                                      | Likelihood | Impact | Mitigation                                                                                                          |
| --- | --------------------------------------------------------- | ---------- | ------ | ------------------------------------------------------------------------------------------------------------------- |
| 1   | SQLite performance on large monorepos (100k+ files)       | Medium     | High   | WAL mode, batch writes, partition by repo, periodic `PRAGMA optimize`. Consider read replicas or sharding if needed |
| 2   | fsnotify unreliability (missed events, especially macOS)  | Medium     | Medium | Periodic full rescan fallback (configurable interval). Track file hashes, compare on rescan                         |
| 3   | go-git limitations (slow on huge repos, no shallow clone) | Medium     | Medium | Fallback to git CLI for `git log`. go-git for basic operations                                                      |
| 4   | Single binary size (Go + SQLite + go-git = 20-40MB)       | Low        | Low    | UPX compression → ~10MB. Acceptable for developer tooling                                                           |
| 5   | MVP scope creep ("just add embeddings", "just add auth")  | High       | High   | Strict scope enforcement. All feature requests go to v1.0+ milestone                                                |
| 6   | Cross-platform path handling (Windows vs Unix)            | Medium     | High   | Use `path/filepath` everywhere. Test on Windows in CI from v0.2                                                     |

### Deliberate Tradeoffs

| Tradeoff                           | Decision       | What We Give Up                  | Why It's Worth It                                                                    |
| ---------------------------------- | -------------- | -------------------------------- | ------------------------------------------------------------------------------------ |
| Go vs Rust                         | Go             | Memory safety, peak performance  | Team productivity, fast compilation, goroutine ergonomics, larger devtools ecosystem |
| FTS5 vs Meilisearch                | FTS5           | Advanced ranking, typo tolerance | Zero dependencies, single binary, no external process                                |
| No plugins in MVP                  | Interface only | Ecosystem, extensibility         | Core stability first; plugin API designed with real usage data                       |
| In-process events vs message queue | Channels       | Persistence, cross-process       | Simplicity, zero deps, sufficient for single-machine                                 |
| Offset pagination vs cursor        | Offset         | Stable ordering under write load | Simpler API, data volumes are moderate                                               |
| No vector embeddings in MVP        | No             | Semantic search                  | FTS5 is good enough for code search; embeddings add ML deps, complexity, binary size |
| SQLite WAL vs no WAL               | WAL            | Slightly larger DB file          | Concurrent reads without blocking, critical for daemon responsiveness                |
| go-git vs shelling out             | go-git         | Full git feature set             | No system dependency, consistent across platforms, faster for batch ops              |

### Assumptions

1. **Single developer, single machine**: Multi-machine sync is explicitly out of scope
2. **Codebases < 1GB**: Typical project sizes. Google/Facebook-scale monorepos are not the target
3. **Git as primary VCS**: Generic file tracking works, but git-specific features assume git. Other VCS is future work
4. **MCP becomes the standard**: If a different protocol wins, only `internal/mcp/` needs replacement
5. **Developers are the users**: Code search, git history, file metadata — all developer concerns

### Worst-Case Scenarios

- **If SQLite can't handle the scale**: The `db` package abstracts the storage engine. Swap via the interface.
- **If MCP loses to a competing protocol**: Replace `internal/mcp/`. The rest of the system remains unchanged.
- **If Go's WASM support matures slowly**: Use external process plugins (subprocess + JSON-RPC) instead. Same plugin interface.
- **If the project needs multi-machine support**: The event bus and DB layer are the main changes. API, indexer, and MCP remain largely unchanged.

---

## Appendix: Go Module Dependencies (MVP)

```
github.com/spf13/cobra         # CLI framework
github.com/spf13/viper         # Configuration
github.com/rs/zerolog          # Structured logging
modernc.org/sqlite             # Pure Go SQLite (no CGo)
github.com/google/uuid         # UUID v7 generation
github.com/go-git/go-git/v5    # Git history extraction
github.com/fsnotify/fsnotify   # File system watcher
github.com/go-chi/chi/v5       # HTTP router (lightweight, stdlib-compatible)
```

### Why `modernc.org/sqlite` instead of `mattn/go-sqlite3`?

`mattn/go-sqlite3` requires CGo. `modernc.org/sqlite` is a pure Go translation of SQLite. This means:

- Cross-compilation without a C compiler
- No CGo overhead
- Static linking (true single binary)
- Easier CI setup

### Why `chi` instead of `gorilla/mux` or `gin`?

- `chi` is idiomatic, lightweight, and compatible with `net/http`. No framework lock-in.
- `gorilla/mux` is effectively abandoned (archived).
- `gin` uses a custom context that diverges from `net/http` — harder to test, harder to integrate with stdlib middleware.
