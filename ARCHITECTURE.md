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
13. [Terminal UI](#13-terminal-ui)
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
         │  Bubble Tea TUI         │
         └────────────────────────┘
```

### Key Decisions

- **Three separate binaries**: `relith` (CLI), `relithd` (daemon), `relithmcp` (MCP server). MCP requires pure JSON-RPC over stdio with no flag parsing, so it must be its own binary.
- **CLI opens DB directly**: No daemon HTTP hop for CLI operations. Simpler, faster, no dependency on a running daemon.
- **Unix socket for daemon API**: File-system permissions as security boundary, no port conflicts. Windows falls back to localhost TCP.
- **SQLite with FTS5**: Zero-dependency embedded database with full-text search. WAL mode allows concurrent readers (MCP + daemon + CLI can coexist).
- **Bubble Tea TUI**: Interactive CLI commands (`index`, `remove`, `serve`) use Bubble Tea for progress bars, spinners, and server dashboard when run in a terminal. Falls back to plain output when piped.
- **Stdlib slog**: All internal logging uses `log/slog`. CLI commands use `slog.DiscardHandler` in TUI mode; daemon and MCP binary use `slog.NewTextHandler(os.Stderr, nil)`.

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
│   │   ├── handlers_admin.go      # Admin handlers (stats, graph)
│   │   ├── response.go            # JSON response helpers
│   │   ├── server.go              # HTTP server, routing
│   │   └── web/                   # Embedded dashboard + graph UI
│   │       └── index.html
│   │
│   ├── mcp/                       # MCP protocol server
│   │   ├── mcp.go                 # Protocol types (JSON-RPC, capabilities)
│   │   ├── server.go              # JSON-RPC dispatcher, lifecycle
│   │   ├── tools.go               # Tool handlers (search_code, etc.)
│   │   ├── tools_git.go           # Git tools (commits, history, blame, diff)
│   │   └── resources.go           # Resource URI handlers
│   │
│   ├── indexer/                   # Core indexing engine
│   │   ├── indexer.go             # Orchestrator (IndexRepo, IndexFile, DeleteFile)
│   │   ├── walker.go              # Directory walk + binary/hidden file filter
│   │   ├── language.go            # Extension-to-language mapping
│   │   ├── refs.go                # Byte-level ref scanner (function calls)
│   │   ├── graph.go               # Import & ref edge builder for dependency graph
│   │   └── cleanup.go             # Multi-table deletion for repo/document removal
│   │
│   ├── chunker/                   # Language-specific code chunkers
│   │   ├── chunker.go             # Chunker interface + registry + fallback
│   │   ├── golang_ast.go          # Go AST-based chunker
│   │   ├── python.go              # Python indentation-based chunker
│   │   ├── js.go                  # JS/TS/Rust chunker
│   │   ├── java.go                # Java chunker
│   │   ├── cpp.go                 # C/C++/C#/Kotlin/Swift/ObjC/Scala/Dart/Zig/F#
│   │   ├── php.go                 # PHP chunker
│   │   ├── ruby.go                # Ruby chunker
│   │   └── brace.go               # Generic brace-based chunker (Perl, F#)
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
│   │   └── graph.sql.go           # Graph edge queries
│   │
│   ├── search/                    # Search abstraction over FTS5
│   │   ├── search.go              # Searcher with FTS5 queries
│   │   └── query.go               # Query builder (prefix, phrase, operators)
│   │
│   ├── git/                       # Git-aware context (shells out to `git`)
│   │   └── git.go                 # Commits, file history, blame, diff helpers
│   │
│   ├── reasoning/                 # Context gathering engine
│   │   └── reasoning.go           # Trace() — combines search + symbols + graph
│   │
│   ├── tui/                       # Terminal UI components (Bubble Tea)
│   │   ├── styles.go              # Lipgloss styles (orange/amber theme)
│   │   ├── progress.go            # Indexing progress bar model
│   │   ├── spinner.go             # Generic spinner for short ops (remove)
│   │   └── server.go              # Server dashboard model (serve)
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
│   │   ├── repo.go                # repo parent command
│   │   ├── repo_add.go            # repo add
│   │   ├── repo_list.go           # repo list
│   │   ├── repo_remove.go         # repo remove (TUI spinner)
│   │   ├── index.go               # index (TUI progress bar)
│   │   ├── search.go              # search
│   │   ├── status.go              # status (styled output via tui styles)
│   │   ├── serve.go               # serve (TUI server dashboard)
│   │   ├── install.go             # install MCP for agents
│   │   ├── uninstall.go           # uninstall MCP from agents
│   │   ├── db.go                  # db vacuum
│   │   ├── version.go             # Version command + ldflags injection
│   │   └── util.go                # Shared DB open helper
│   │
│   └── app/                       # Shared application struct
│       └── app.go
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

- **`internal/`**: Go visibility enforcement — these packages cannot be imported by external consumers.
- **`sql/` separate from `db/`**: Source of truth (SQL migrations + sqlc queries) vs generated Go code.
- **`cmd/`**: Thin entry points — parse flags, load config, launch component. Zero business logic.
- **`bin/`**: Build output, gitignored.

## 3. Package Responsibilities

| Package            | Responsibility                                                       | Dependencies (internal)                              |
| ------------------ | -------------------------------------------------------------------- | ---------------------------------------------------- |
| `cmd/relith`       | Parse CLI flags (cobra), open DB, dispatch commands                  | `internal/cli`, `internal/config`, `internal/db`     |
| `cmd/relithd`      | Parse flags, load config, instantiate daemon, block on signal        | `internal/daemon`, `internal/config`                 |
| `cmd/relithmcp`    | Load config, open DB, start MCP server over stdio                    | `internal/mcp`, `internal/config`, `internal/db`     |
| `internal/api`     | HTTP routing, request validation, JSON marshaling                    | `internal/db`, `internal/search`, `internal/indexer`, `internal/reasoning` |
| `internal/mcp`     | JSON-RPC over stdio, tool/resource registration, dispatch            | `internal/db`, `internal/search`, `internal/reasoning`, `internal/git` |
| `internal/indexer` | Walk filesystems, detect languages, chunk content, hash-based diff, extract symbols/refs, build dependency graph | `internal/db`, `internal/chunker`                     |
| `internal/chunker` | Language-specific code chunking (AST, regex, brace-matching)         | None                                                  |
| `internal/watcher` | Wrap fsnotify, debounce, filter, call IndexFile/DeleteFile           | `internal/indexer`                                   |
| `internal/db`      | Connection lifecycle, migration runner, sqlc-generated methods       | None (sqlite driver only)                            |
| `internal/search`  | FTS5 query construction, BM25 ranking, result formatting             | `internal/db`                                        |
| `internal/git`     | Git-aware context: commits, file history, blame, diffs                | None (invokes the `git` CLI)                        |
| `internal/reasoning`| Combine search + symbols + refs + graph into a context bundle       | `internal/db`, `internal/search`                     |
| `internal/tui`     | Bubble Tea models (progress bar, spinner, server dashboard)          | `internal/indexer` (for ProgressEvent type)          |
| `internal/daemon`  | Component wiring, graceful shutdown, signal handling                 | `internal/api`, `internal/config`                    |
| `internal/config`  | Load/merge config from file + env, validate, defaults                | viper                                                |
| `internal/cli`     | Cobra command definitions, TUI dispatch                              | `internal/db`, `internal/indexer`, `internal/search`, `internal/tui` |
| `internal/app`     | Shared App struct (Config, Logger, DB)                               | `internal/config`                                    |

## 4. Data Flow

### A. Adding and Indexing a Repository

```
User: relith repo add /path/to/project

CLI ── open DB ──▶ INSERT INTO repositories
               ──▶ Indexer: WalkRepo → for each file:
                        - compute FNV-64a hash
                        - detect language
                        - chunk content (50 lines, 0 overlap for fallback;
                          AST/regex boundaries for language-specific)
                        - write document + chunks to DB
                        - FTS5 sync triggers populate chunks_fts
                        - extract refs (function calls via byte-level scanner)
                        - batch INSERT refs
               ──▶ BuildGraphForRepo:
                        - extract import edges (Go/JS/TS/Python/Rust)
                        - compute ref edges via refs JOIN symbols
                          (filtered by symbol_freq CTE to avoid explosion)
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
                                   SELECT FROM chunks_fts JOIN chunks
                                   JOIN documents JOIN repositories
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
CLI → open DB → Queries/Indexer → DB → output (TUI or plain text)
```

Used for: adding repos, listing repos, indexing, search, status, remove, db vacuum.

### Pattern 2: Daemon HTTP API

```
Client (curl/app) → HTTP → API Handler → Queries/Searcher → JSON response
```

Used for: health checks, programmatic access, remote queries, graph visualization.

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
    hash            TEXT NOT NULL,            -- FNV-64a hex
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

-- Symbol definitions (functions, types, variables, macros)
CREATE TABLE symbols (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_id   INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    name     TEXT NOT NULL,
    kind     TEXT NOT NULL DEFAULT 'function'
             CHECK(kind IN ('function','type','variable','constant','method','field','enum','interface','class','struct','macro','module')),
    line     INTEGER NOT NULL DEFAULT 0,
    col      INTEGER NOT NULL DEFAULT 0,
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
             CHECK(kind IN ('call','import','use','write','read','type_ref')),
    line     INTEGER NOT NULL DEFAULT 0,
    col      INTEGER NOT NULL DEFAULT 0,
    context  TEXT NOT NULL DEFAULT ''
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

# Content
GET    /v1/content?repo=&path=         → File content (raw)

# Reasoning
GET    /v1/reason?q=&repo=&max_results= → TraceBundle JSON

# Stats
GET    /v1/stats                       → Aggregated stats with storage savings %

# Graph
GET    /v1/graph?repo=<name>           → {nodes: [...], edges: [...]} (JSON for API consumers)
GET    /v1/graph                       → Interactive D3.js force-directed graph (HTML for browser)
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

# Graph as JSON
curl -s "http://127.0.0.1:9876/v1/graph?repo=my-repo"
```

## 8. MCP Tools

The MCP server (`relithmcp`) implements the [Model Context Protocol](https://modelcontextprotocol.io) specification over stdio.

### Tools

| Tool Name             | Description                                      | Parameters                                                                                  |
| --------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| `search_code`         | Full-text search across indexed repos            | `query` (required), `repo_name` (optional), `language` (optional), `max_results` (default 20) |
| `get_file_content`    | Retrieve a file's content by repo name + path    | `repo_name` (required), `path` (required)                                                     |
| `list_repositories`   | List all tracked repos with status and file count | —                                                                                           |
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
| `get_recent_commits`  | Recent git commits (hash, date, author, subject)  | `repo_name` (required), `max` (default 20)                                                      |
| `get_file_history`    | Commit history for a file (follows renames)       | `repo_name` (required), `path` (required), `max` (default 20)                                   |
| `get_blame`           | Per-line authorship for a file or line range      | `repo_name` (required), `path` (required), `start_line`, `end_line`                             |
| `get_diff`            | Stat summary + full patch between two refs        | `repo_name` (required), `base` (default HEAD~1), `head` (default HEAD), `max_stat`              |

### Git-Aware Context

The four git tools shell out to the system `git` binary (running with the repo root as workdir) — no git library dependency. They enrich MCP answers with *when/why/who* context: recent changes (`get_recent_commits`), what one file went through (`get_file_history`), who owns each line (`get_blame`), and exactly what a change did (`get_diff`). A repo must be a git worktree (`.git/` present) or the tools return a clear error. Parser utilities live in `internal/git/git.go`; handlers in `internal/mcp/tools_git.go`. Default `get_diff` is `HEAD~1...HEAD`, answering "what did the last commit change".

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
   - Prioritize high-value files (main entry points: `main.go`, `package.json`, `go.mod`, etc.)
4. Process files in batches of 500 with N concurrent workers (default 4):
   - Read file content
   - Compute FNV-64a hash for change detection
   - Check mod_time/size against existing document — skip if unchanged
   - Detect language (extension map: ~90 languages)
   - Chunk via language-specific chunker; fall back to line-based (50 lines, 0 overlap)
   - **Extract refs**: byte-level scanner finds `identifier()` patterns (no allocations, no regex)
   - Send batch to serial writer goroutine
5. Writer goroutine batch-inserts to DB (multi-row INSERTs, 199 rows per stmt):
   - Create/update document row
   - Delete old chunks/symbols/refs (if updating)
   - Insert new chunks (FTS5 sync triggers populate `chunks_fts`)
   - Batch INSERT refs
6. Delete stale documents (in DB but not on disk)
7. Update repo: status=`ready`, file_count, last_indexed_at

### Graph Build (runs after index)

1. Clear existing `graph_edges` for repo
2. Extract import edges per doc: Go (`import "..."`), JS/TS (`from "..."`), Python (`import ...`), Rust (`use ...`)
3. Compute ref edges: `SELECT FROM refs JOIN symbols ON name` (filtered by `symbol_freq` CTE to exclude names in >20 docs — avoids combinatorial explosion)
4. Batch INSERT all edges into `graph_edges` table (kinds: `import`, `references`)
5. Non-import-capable languages (C/C++/Java/PHP/Ruby/etc.) skip the per-doc import loop — only ref edges are computed

### Incremental Index (File Change via Watcher)

1. Receive fsnotify event (file created/modified/deleted)
2. Debounce (coalesce rapid events within configurable window)
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

- `porter`: English stemming (running → run)
- `unicode61`: Unicode-aware tokenization
- Content-sync: triggers on `chunks` table keep FTS5 in sync automatically

### Query Pipeline

```
Raw query: "auth middleware"
     │
     ▼
  buildMatchQuery()
     ├── Single term → "term"*    (prefix match)
     ├── Multiple terms → "t1" "t2" (AND by default)
     ├── Quoted → preserved as phrase
     ├── Has FTS5 operators (AND, OR, NOT) → passthrough
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
  socket: ""                    # empty = TCP; set to use Unix socket
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
```

### Environment Variable Overrides

All values overridable via `RELITH_` prefix: `RELITH_DAEMON_TCP_PORT=9877`, `RELITH_INDEXER_CONCURRENCY=8`, etc.

### Precedence (lowest to highest)

1. Default values (hardcoded in `config.go`)
2. Config file
3. Environment variables

## 13. Terminal UI

CLI commands use [Bubble Tea](https://github.com/charmbracelet/bubbletea) for interactive terminal output when `os.Stdout` is a terminal. When piped or redirected, they fall back to plain text output.

### Components

All TUI components live in `internal/tui/`:

| Component | File | Used By | Description |
|-----------|------|---------|-------------|
| `Progress` | `progress.go` | `relith index` | Animated progress bar with ETA, elapsed time, file count, error count. Phases: "Walking...", progress bar during indexing, "Building graph..." |
| `ServerModel` | `server.go` | `relith serve` | Live dashboard: server URL plus repo/file/chunk/symbol/ref counts, refreshed every 2s. Polls `GetStats` via a `StatsFunc` passed from the CLI. |
| `Spinner` | `spinner.go` | `relith repo remove` | Simple spinner that blocks on a `doneCh` — spins until backend operation completes, then prints result and exits |

### Theme

Orange/amber warm theme via [Lipgloss](https://github.com/charmbracelet/lipgloss):

- Primary: Orange `#FF7700`
- Accents: Yellow `#FFB347`, Gold `#FFD700`
- Success: Green `#00CC66`
- Error: Red `#FF4444`
- Text: WarmWhite `#F0E6D0`, Grey `#888888`
- Borders: Rounded with orange

### TUI Detection

```
if term.IsTerminal(int(os.Stdout.Fd())) {
    return runTUI(...)
}
return runPlain(...)
```

Commands using TUI: `index`, `remove`, `serve`.
Commands with styled output (but not interactive TUI): `status`, `repo list`.

## 14. Performance Optimizations

### SQLite Tuning

- `PRAGMA synchronous=NORMAL` — 2× faster writes than FULL with same durability guarantee
- `PRAGMA cache_size=-64000` — 64MB page cache
- `PRAGMA temp_store=MEMORY` — temp tables in memory
- `PRAGMA mmap_size=268435456` — 256MB memory-mapped I/O

### Batch Operations

- All INSERTs use multi-row format (`(?,?), (?,?), ...`) with up to 199 rows per statement
- Respects SQLite's 999 parameter limit per statement
- Shared `batchExecer` interface between indexer and graph builder

### Graph Build Optimization

- `symbol_freq` CTE: filters symbol names appearing in >20 docs to avoid combinatorial explosion in `refs JOIN symbols`
- Import-capable language check: only Go/JS/TS/Python/Rust files get per-doc import loop (C/C++/Java/etc. skipped — ref edges only)
- Edges pre-computed into `graph_edges` table; API reads from table instead of re-running the JOIN
- Compound indexes `refs(name, doc_id)` and `symbols(name, doc_id)` for covering index scans on the graph query
- Pre-filter refs and symbols to `symbol_freq` names via `WHERE name IN` before the cross-join (reduces intermediate rows from full cross-product to only names passing frequency filter)

### Linux Kernel Benchmark (94,989 C files)

| Phase | Before | After | Speedup |
|-------|--------|-------|---------|
| Walk + index | ~14min | ~14min 40s | ~1× (I/O bound) |
| Graph build | 24min 12s | **1min 8s** | **21×** |
| **Total** | **38min** | **15min 48s** | **2.4×** |

### Language-Specific Chunkers

| Language  | Chunker            | Strategy                                                              |
| --------- | ------------------ | --------------------------------------------------------------------- |
| Go        | `GoChunkerAST`     | Full AST via `go/parser` + `go/ast`. Functions, methods, types, structs, interfaces, variables, constants |
| Python    | `PythonChunker`    | Regex-based. Tracks indentation depth. `def` → function, `class` → class, handles decorators |
| JavaScript| `JSChunker`        | Regex + brace matching. Functions, classes, arrow functions, methods  |
| TypeScript| `JSChunker`        | Same as JavaScript chunker                                            |
| Rust      | `JSChunker`        | Handles `fn`, `impl`, `trait`, `struct`, `enum` via Rust-specific patterns |
| Java      | `JavaChunker`      | Regex + brace matching. Classes (inner + outer), methods, constructors, annotations |
| C/C++/C#/Kotlin/Swift/ObjC/Scala/Dart/Zig/F# | `CppChunker` | Massive pattern set. Handles C++ constructors, C# records/delegates/events, Kotlin fun/class/object, Swift func/class/struct/enum/protocol. Brace matching |
| PHP       | `PHPChunker`       | Regex + brace matching. Functions, classes, interfaces, traits, enums. Skips PHP tags and imports |
| Ruby      | `RubyChunker`      | Regex-based. `def`/`end` depth tracking for scope                      |
| Perl, F#  | `BraceChunker`     | Generic brace-based. Detects class/struct/interface/trait/enum + function/fn |
| All others| `FallbackChunker`  | Line-based: 50 lines per chunk, 0 overlap                              |

### Per-File Processing Pipeline

Current pipeline (index phase):
```
ReadFile (full string) ─┐
  ├── Chunker: strings.Split(content, "\n")
  │       └── regex or scanner line-by-line for decls
  ├── ExtractReferences: byte-level scanner    ← no split, no regex
  └── fastHash(content)                        ← FNV-64a
                       └── writeBatch (serial, 500 files per batch)
```

Remaining optimization opportunities (in priority order):
1. Pipeline writes with reads: Producer/consumer channel so DB writes overlap with next batch's file preparation
2. Reuse worker pool: Currently created/destroyed every 500 files × 188 batches; use a long-lived pool

## 15. Version Roadmap

### v0.1 — MVP (Complete)

- Go module with CLI (cobra), daemon entry point, config loading
- SQLite with FTS5, sqlc-generated queries, migrations
- Indexer with walker, language detection, chunking, hash-based change detection
- CLI commands: `repo add`, `repo list`, `index`, `search`, `status`
- REST API: health, repo CRUD, indexing trigger, search
- File watcher (fsnotify + debouncer)

### v0.2 — Symbol & Graph (Complete)

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

### v0.3 — Reasoning Engine (Complete)

- Graph-enhanced code reasoning (`internal/reasoning`)
- Seed-based context gathering (seed docs → related files via graph edges → related repos)
- MCP tool: `get_code_context` (later renamed `trace_context`)
- Browser-based graph UI hardened

### v0.4 — Performance & Scale (Complete)

- Graph build optimization: `symbol_freq` CTE + `WHERE name IN` pre-filter + compound indexes (21× graph build speedup)
- Import-capable language filter: only Go/JS/TS/Python/Rust files get per-doc import loop
- SQLite PRAGMA tuning + batch multi-row INSERTs
- FTS cleanup: explicit multi-table deletion (`DeleteDocuments`, `DeleteRepoWithData`)
- Linux kernel 94K files: **15min 48s total** (index 14min 40s + graph build 1min 8s)

### v0.5 — Code Intelligence (Complete)

- Byte-level ref scanner no-allocation, no-regex `ExtractReferences`
- Walker skip list additions (`generated/`, `tools/`, `scripts/`)
- Expanded to 17 MCP tools: symbol definition, callers/callees, file outline, file tree, dependency trace, graph queries, architecture overview, context tracing
- Chunk memory fix: removed `content` from `batchWork` struct (unused in batch path), `strings.Clone` on ref context to prevent substring pinning of large files
- Log migration: removed zerolog, migrated all packages to stdlib `log/slog`
- Terminal UI: Bubble Tea progress bar for `index`, spinner for `remove`, server dashboard for `serve`
- Config cleanup: removed `log` section from config

### v0.6 — Git-Aware Context (Complete)

- 4 new MCP tools (17 → 21 total): `get_recent_commits`, `get_file_history`, `get_blame`, `get_diff`
- New `internal/git` package that shells out to the system `git` binary (no go-git dependency)
- Answers *when / why / who* questions: recent changes, per-file history (follows renames), line authorship, and full patches between refs
- `GetRepoByName` query for name-based repo lookup

### v0.7 — Work In Progress

- **Storage optimization**: Reduce chunk storage overhead (deduplicate identical chunks, optional comment stripping for FTS)

### Planned

- **Vector embeddings / semantic search**: Natural language queries over code
- **Autocomplete API**: `/v1/search/suggest` endpoint
- **MCP TCP mode**: Run MCP server inside the daemon
- **SSE event stream**: Real-time indexing progress
- **Advanced query filters**: `repo:`, `path:`, `lang:` scoped search
- **Plugin system**: WASM-based plugins for custom processing
- **IDE extensions**: VS Code, JetBrains, Zed
- **CI/CD integration**: Auto-index PRs
