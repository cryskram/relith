# Contributing to Relith

Thanks for considering a contribution! Relith is a local-first context engine for AI-assisted coding, built in Go. This guide covers how to set up a dev environment and what we expect from changes.

## Code of conduct

This project follows a [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to abide by its terms.

## Reporting issues

- **Bugs** - use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md). Include the relith version (`relith version`), your OS/arch, the repo you indexed, and any error output.
- **Feature requests** - use the [feature request template](.github/ISSUE_TEMPLATE/feature_request.md).
- **Security issues** - do **not** open a public issue. See [SECURITY.md](SECURITY.md).

## Development environment

Requirements:

- Go 1.26 or newer
- `git`
- Optional but recommended: `golangci-lint` (v2), `sqlc` (only if you change queries)

Clone and build:

```bash
git clone https://github.com/cryskram/relith.git && cd relith
make build-all          # builds bin/relith, bin/relithd, bin/relithmcp
```

## Common tasks

```bash
make build        # compile all packages
make test         # race-enabled unit tests
make coverage     # test coverage + html report (coverage.html)
make vet          # go vet
make lint         # golangci-lint (v2)
make fmt          # gofmt
make tidy         # go mod tidy
make sqlc         # regenerate sqlc code after editing sql/ queries
```

## Repository layout

| Path | Purpose |
|------|---------|
| `cmd/relith` | CLI + TUI entrypoint |
| `cmd/relithd` | Daemon (REST API + graph UI + file watcher) entrypoint |
| `cmd/relithmcp` | MCP server entrypoint |
| `internal/api` | REST API handlers |
| `internal/chunker` | Language-aware code chunking |
| `internal/cli` | CLI commands and TUI wiring |
| `internal/db` | sqlc-generated query layer + migrations |
| `internal/git` | git-aware context tools (commits, blame, diffs) |
| `internal/indexer` | Walk → chunk → extract → graph pipeline |
| `internal/mcp` | MCP server (JSON-RPC over stdio) |
| `internal/reasoning` | `trace_context` hybrid retrieval engine |
| `internal/search` | FTS5 search |
| `sql/migrations` | Goose SQL migrations |

## Making changes

1. **Create a branch** (`git checkout -b feature/your-change`).
2. **Write or update tests.** New behavior should ship with tests. Relith targets solid coverage, and CI enforces it via Codecov.
3. **Run the full gate locally:**
   ```bash
   make test
   make lint
   make vet
   make fmt
   ```
4. **Commit** with a Conventional Commit message, e.g. `feat(mcp): add tool to ...`, `fix(search): handle ...`, `docs: update ...`.
5. **Open a pull request** against `main`, using the [pull request template](.github/PULL_REQUEST_TEMPLATE.md).

## Adding a SQL migration

Migrations live in `sql/migrations` and are embedded into the binary via `//go:embed` - no build step needed. To add one:

1. Create `sql/migrations/000NN_description.sql` using the same `-- +goose Up` / `-- +goose Down` format as existing files.
2. Add a test that exercises the new schema (see `internal/db/sqlite_test.go` for patterns).

## Adding or changing a database query

Query source files live in `sql/queries/*.sql` and are compiled with [sqlc](https://sqlc.dev) into `internal/db/*.sql.go`. After editing a query:

```bash
sqlc generate    # or: make sqlc
```

## Adding an MCP tool

1. Implement the handler in `internal/mcp/tools.go` following the existing `handleX` pattern.
2. Register it in `Server.registerTools` (`internal/mcp/server.go`).
3. Add a JSON-RPC test in `internal/mcp/server_test.go`.
4. Document the tool in the README tool table.

## Benchmarks

The kernel-index benchmark is scripted in `bench/` - see `docs/benchmarks.md` for how to run and reproduce it.

## Questions

Open a [discussion](https://github.com/cryskram/relith/discussions) or a draft PR - happy to help.
