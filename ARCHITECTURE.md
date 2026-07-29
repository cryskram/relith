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
14. [Performance Optimizations](#14-performance-optimizations)
15. [Version Roadmap](#15-version-roadmap)

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
│   │   ├── language.go            # Extension-to-language mapping
│   │   ├── symbols.go             # Symbol extraction (functions, types, variables)
│   │   ├── refs.go                # Ref extraction (function calls, references)
│   │   ├── graph.go               # Import & ref edge builder for dependency graph
│   │   ├── cleanup.go             # Multi-table deletion for repo/document removal
│   │   ├── java.go                # Java-specific chunker (handles generics)
│   │   ├── cpp.go                 # C/C++/C#/Kotlin/Swift/ObjC/Scala/Dart/Zig/F# chunker
│   │   ├── php.go                 # PHP chunker
│   │   └── ruby.go                # Ruby chunker
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
│   │   ├── chunks.sql.go          # Chunk CRUD + FTS5 sync
│   │   ├── symbols.sql.go         # Symbol CRUD (functions, types, variables)
│   │   ├── refs.sql.go            # Ref CRUD (function calls, references)
│   │   ├── graph.sql.go           # Graph edge queries
│   │   └── sqlite.go              # Connection, WAL, PRAGMAs (split from db.go)
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
│       ├── chunks.sql
│       ├── symbols.sql
│       ├── refs.sql
│       └── graph.sql
│
├── bin/                           # Build output (gitignored)
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
| `internal/indexer` | Walk filesystems, detect languages, chunk content, hash-based diff, extract symbols/refs, build dependency graph | `internal/db`                                        |
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
                        - extract symbols (functions, types, variables)
                        - extract refs (function calls, imports, references)
                        - batch INSERT symbols + refs
               ──▶ BuildGraphForRepo:
                        - extract import edges (Go/JS/TS/Python/Rust)
                        - compute ref edges via refs JOIN symbols
                        - batch INSERT into graph_edges table
                        - kinds: 'import' (explicit) + 'references' (co-occurrence)
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

-- Symbol definitions (functions, types, variables, macros)
CREATE TABLE symbols (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_id   INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    name     TEXT NOT NULL,
    kind     TEXT NOT NULL DEFAULT 'function'
             CHECK(kind IN ('function','type','variable','constant','method','field','enum','interface','class','struct','macro','module')),
    line     INTEGER NOT NULL DEFAULT 0,
    parent   TEXT
);
CREATE INDEX idx_symbols_doc_id ON symbols(doc_id);
CREATE INDEX idx_symbols_name ON symbols(name);

-- Symbol references (function calls, imports, usages)
CREATE TABLE refs (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_id   INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    name     TEXT NOT NULL,
    kind     TEXT NOT NULL DEFAULT 'call'
             CHECK(kind IN ('call','import','use','write','read','type_ref'))
);
CREATE INDEX idx_refs_doc_id ON refs(doc_id);
CREATE INDEX idx_refs_name ON refs(name);

-- Pre-computed dependency graph edges
CREATE TABLE graph_edges (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id        INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    source_doc_id  INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    target_doc_id  INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    weight         INTEGER NOT NULL DEFAULT 1,
    kind           TEXT NOT NULL DEFAULT 'import'
                   CHECK(kind IN ('import','references')),
    UNIQUE(repo_id, source_doc_id, target_doc_id, kind)
);
CREATE INDEX idx_graph_edges_repo_id ON graph_edges(repo_id);
```

### Schema Decisions

- **INTEGER primary keys (auto-increment)**: Simpler than UUIDs for an MVP. Sequential IDs cluster well in SQLite B-trees.
- **DATETIME (not TEXT)**: SQLite has no native datetime type, but TEXT ISO 8601 is used for portability. INTEGER timestamps are also valid.
- **Content-sync FTS5**: The `content=chunks` declaration tells FTS5 to sync automatically via triggers on the `chunks` table. No manual FTS insert/update/delete needed.
- **`documents` table (not `files`)**: Named `documents` to avoid confusion with filesystem files and to leave room for non-file documents in the future (e.g., documentation pages).
- **`symbols` + `refs` separate**: Symbol extraction captures definitions; ref extraction captures references. The graph engine joins them on `name` to find co-occurrence. The `symbol_freq` CTE filters names appearing in more than 20 docs to avoid combinatorial explosion (critical for large C/C++ repos).
- **`graph_edges` pre-computed**: The dependency graph is computed during `BuildGraphForRepo` and stored in `graph_edges`. The API reads from this table rather than re-running the expensive `refs JOIN symbols` query. Two edge kinds: `import` (explicit imports in Go/JS/TS/Python/Rust) and `references` (co-occurrence via refs/symbols join).
- **FTS content deletion**: FTS5 content-sync triggers only fire on INSERT/UPDATE/DELETE of the `chunks` table. When rows are deleted by FK CASCADE from `documents`, the FTS triggers do NOT fire. Cleanup logic (`DeleteDocuments`, `DeleteRepoWithData`) explicitly walks tables in dependency order (graph_edges → chunks → symbols → refs → documents → repositories) to ensure FTS stays consistent.

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

# Graph Visualization
GET    /v1/graph?repo=<name>           → {nodes: [{id, path}], edges: [{source, target, weight}]}
GET    /v1/graph.html                  → Interactive D3.js force-directed graph (browser)
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
- Web UI beyond graph (planned: search, repo management in browser)

## 8. MCP Tools

The MCP server (`relithmcp`) implements the [Model Context Protocol](https://modelcontextprotocol.io) specification over stdio.

### Tools

| Tool Name             | Description                                      | Parameters                                                                                  |
| --------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| `search_code`         | Full-text search across indexed repos            | `query` (required), `repo_name` (optional), `language` (optional), `max_results` (default 20) |
| `get_file_content`    | Retrieve a file's content by repo name + path    | `repo_name` (required), `path` (required)                                                     |
| `list_repositories`   | List all tracked repos with status and file count | -                                                                                           |
| `get_repo_summary`    | Language breakdown, file/chunk count, last indexed| `repo_name` (required)                                                                        |
| `find_symbol`         | Search symbols by name prefix                    | `name` (required), `kind` (optional), `repo_name` (optional)                                  |
| `find_references`     | Find all call sites for a symbol name            | `name` (required), `repo_name` (optional)                                                     |
| `trace_context`       | Combine search + symbols + graph into one bundle | `query` (required), `repo_name` (optional), `max_results` (default 8)                         |
| `get_file_outline`    | Symbols and refs in a file (metadata + chunks)   | `repo_name` (required), `path` (required)                                                      |
| `get_symbol_definition`| Find exact definition of a symbol               | `name` (required), `repo_name` (optional), `kind` (optional), `max_results` (default 10)       |
| `find_callees`        | Functions called inside a symbol's definition    | `name` (required), `repo_name` (optional), `max_results` (default 15)                          |
| `find_callers`        | Call sites for a symbol (across repos)           | `name` (required), `repo_name` (optional), `max_results` (default 20)                          |
| `get_related_files`   | Graph neighbors for a file                       | `repo_name` (required), `path` (required), `max_results` (default 12)                          |
| `list_hub_files`      | Most connected files (degree centrality)         | `repo_name` (optional), `max_results` (default 15)                                             |
| `query_graph`         | Query graph: neighbors, hotspots, or path        | `mode` (required), `repo_name` (required), `path`, `target_path`, `max_results`                |
| `get_architecture`    | High-level arch overview (langs, dirs, hubs)     | `repo_name` (required), `max_results` (default 10)                                             |
| `trace_dependency`    | Import/reference dependency trace (recursive)    | `repo_name` (required), `path` (required), `direction`, `depth` (default 1), `max_results`     |
| `get_file_tree`       | Browse directory tree (show immediate children)  | `repo_name` (required), `path` (optional, defaults to root)                                    |

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
relith://repos/{id}/files        → File listing per repo (JSON)
relith://repos/{id}/graph        → Graph edges per repo (JSON)
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
   - **Extract symbols**: regex-based parser per language finds function/type/variable definitions
   - **Extract refs**: regex-based parser per language finds function calls, imports, references
5. Write to DB in concurrent batches (multi-row INSERTs):
   - Create/update document row
   - Delete old chunks (if updating)
   - Insert new chunks (FTS5 sync triggers populate `chunks_fts`)
   - Batch INSERT symbols + refs (199 rows per stmt, respects 999 SQLite param limit)
6. Update repo: status=`ready`, file_count, last_indexed_at

### Graph Build (runs after index)

1. Clear existing `graph_edges` for repo
2. Extract import edges per doc (Go: `import "...", JS/TS: `from "...", Python: `import ...`, Rust: `use ...`)
3. Compute ref edges: `SELECT FROM refs JOIN symbols ON name` (filtered by `symbol_freq` CTE to exclude names in >20 docs to avoid combinatorial explosion)
4. Batch INSERT all edges into `graph_edges` table (kinds: `import`, `references`)
5. Non-import-capable languages (C/C++/Java/PHP/Ruby/etc.) skip the per-doc import loop - only ref edges are computed for those

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

## 14. Performance Optimizations

### SQLite Tuning

- `PRAGMA synchronous=NORMAL` - 2x faster writes than FULL with same durability guarantee
- `PRAGMA cache_size=-64000` - 64MB page cache
- `PRAGMA temp_store=MEMORY` - temp tables in memory
- `PRAGMA mmap_size=268435456` - 256MB memory-mapped I/O

### Batch Operations

- All INSERTs use multi-row format (`(?,?), (?,?), ...`) with up to 199 rows per statement
- Respects SQLite's 999 parameter limit per statement
- Shared `batchExecer` interface between indexer and graph builder

### Graph Build Optimization

- `symbol_freq` CTE: filters symbol names appearing in >20 docs to avoid combinatorial explosion in `refs JOIN symbols`
- Import-capable language check: only Go/JS/TS/Python/Rust files get per-doc import loop (C/C++/Java/etc. skipped - ref edges only)
- Edges pre-computed into `graph_edges` table; API reads from table instead of re-running the JOIN
- Graph page filter: only drops edges where both endpoints are outside the top 300 by degree (fix: `&&` not `||`)
- Compound indexes `refs(name, doc_id)` and `symbols(name, doc_id)` for covering index scans on the graph query
- Pre-filter refs and symbols to `symbol_freq` names via `WHERE name IN` before the cross-join (reduces intermediate rows from full cross-product to only names passing frequency filter)

### Linux Kernel Benchmark (94,989 C files)

| Phase | Before | After | Speedup |
|-------|--------|-------|---------|
| Walk + index | ~14min | ~14min 40s | ~1× (I/O bound) |
| Graph build | 24min 12s | **1min 8s** | **21×** |
| **Total** | **38min** | **15min 48s** | **2.4×** |

Remaining indexing bottleneck is per-file processing (read → split → regex parse → write), which is CPU-bound on Go's regex engine and string allocation. See [Remaining Bottlenecks](#remaining-bottlenecks).

### Language-Specific Chunkers

Generic line-based chunking works for every language, but language-specific chunkers provide better boundary alignment:

| Language  | Strategy                                                              |
| --------- | --------------------------------------------------------------------- |
| Java      | Top-level class boundary + handles nested generics `ApiResponse<Page<...>>` |
| C/C++/etc.| Function boundary + brace balancing (C, C++, C#, Kotlin, Swift, ObjC, Scala, Dart, Zig, F#) |
| PHP       | `<?php` / `?>` + function boundary                                     |
| Ruby      | `def` / `end` scoping                                                  |

### Remaining Bottlenecks

Current per-file processing pipeline (index phase):
```
ReadFile (full string) ─┐
  ├── Chunker: strings.Split(content, "\n")    ← split #1
  │       └── regex line-by-line for decls
  ├── ExtractReferences: byte-level scanner    ← no split, no regex
  └── fastHash(content)                         ← FNV hash
                          └── writeBatch (serial, 500 files per batch)
```

Top optimization opportunities (in priority order):
1. ~~Triple `strings.Split` elimination~~ **Done**: ref extractor uses byte-level scanner (no split needed)
2. ~~Replace regex in ref extraction with byte-level scanner~~ **Done**: `ExtractReferences` scans content for `(` then walks backwards for the identifier name — no allocations, no regex, no split
3. ~~Redundant `os.Stat` in ReadFileContent~~ **Done**: `prepareBatchWork` reads directly via `os.ReadFile`, using `fi.Size` from the walk
4. ~~Skip generated files in walker~~ **Done**: added `generated/`, `tools/`, `scripts/` to `skipDirs`
5. Pipeline writes with reads: Producer/consumer channel so DB writes overlap with next batch's file preparation
6. Reuse worker pool: Currently created/destroyed every 500 files × 188 batches; use a long-lived pool

## 15. Version Roadmap

### v0.1 - MVP (Complete)

- Go module with CLI (cobra), daemon entry point, config loading
- SQLite with FTS5, sqlc-generated queries, migrations
- Indexer with walker, language detection, chunking, hash-based change detection
- CLI commands: `repo add`, `repo list`, `index`, `search`, `status`
- REST API: health, repo CRUD, indexing trigger, search
- File watcher (fsnotify + debouncer)
- Zerolog structured logging

### v0.2 - Symbol & Graph

- MCP server with 7 tools: search_code, get_file_content, list_repositories, get_repo_summary, find_symbols, find_refs, graph_hubs
- Cross-platform builds (Windows + Linux + macOS)
- Makefile with version injection via ldflags
- Symbol extraction (functions, types, variables) per language
- Ref extraction (calls, imports, references) per language
- Dependency graph engine (import edges + ref co-occurrence)
- Graph visualization web UI (D3.js force-directed)
- Language-specific chunkers: Java, C++, PHP, Ruby
- SQLite performance tuning (PRAGMAs, batch INSERTs)
- FTS content deletion fix (explicit cleanup for FK CASCADE gaps)

### v0.3 - Reasoning Engine

- Graph-enhanced code reasoning (`internal/reasoning`)
- Seed-based context gathering (seed docs → related files via graph edges → related repos)
- MCP tool: `get_code_context` - collects relevant context for a query
- Browser-based graph UI hardened

### v0.4 - Performance & Scale

- Graph build optimization: `symbol_freq` CTE + `WHERE name IN` pre-filter + compound indexes
- Compound indexes `refs(name, doc_id)` and `symbols(name, doc_id)` for covering index scans
- Pre-filter refs/symbols to `symbol_freq` names via `WHERE name IN` before the cross-join
- Import-capable language filter: only Go/JS/TS/Python/Rust files get per-doc import loop
- Edge filter fix in graph UI: `&&` not `||` - drops edges only when both endpoints are outside top 300
- SQLite PRAGMA tuning: `synchronous=NORMAL`, `cache_size=-64000`, `temp_store=MEMORY`, `mmap_size=268435456`
- Batch multi-row INSERTs for chunks, symbols, refs, graph edges
- FTS cleanup: explicit multi-table deletion (`DeleteDocuments`, `DeleteRepoWithData`)
- Linux kernel 94K files: **15min 48s total** (index 14min 40s + graph build 1min 8s, graph 21× faster)

### v0.5 - Code Intelligence (Current)

- Byte-level ref scanner: eliminated `strings.Split` + regex in `ExtractReferences` (no allocations, no regex engine)
- Redundant `os.Stat` bypass: `prepareBatchWork` reads directly using walker's `fi.Size`
- Walker skip list: added `generated/`, `tools/`, `scripts/` (reduces file count 5-10% for kernel)
- MCP tools: `find_symbol`, `find_references`, `get_symbol_definition`, `find_callees`, `find_callers`
- File intelligence: `get_file_outline` (symbols + refs in a file), `get_file_tree` (directory browser)
- Graph traversal: `get_related_files`, `trace_dependency`, `query_graph`, `get_architecture`, `list_hub_files`
- Context gathering: `trace_context` combines FTS search + symbol matches + references + graph-linked files into one reasoning bundle
- 17 total MCP tools covering search, symbol, reference, graph, and file operations

### Planned

- **Git history indexing**: Extract commit history using go-git
- **Vector embeddings / semantic search**: Natural language queries over code
- **Autocomplete API**: `/v1/search/suggest` endpoint
- **MCP TCP mode**: Run MCP server inside the daemon
- **SSE event stream**: Real-time indexing progress
- **Advanced query filters**: `repo:`, `path:`, `lang:` scoped search
- **Plugin system**: WASM-based plugins for custom processing
- **IDE extensions**: VS Code, JetBrains, Zed
- **CI/CD integration**: Auto-index PRs
