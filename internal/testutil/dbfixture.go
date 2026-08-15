// Package testutil provides shared test fixtures for relith tests.
package testutil

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
)

// DB is a migrated in-memory SQLite database with helpers for seeding fixtures.
type DB struct {
	t   *testing.T
	SQL *sql.DB
}

// NewDB opens a migrated in-memory database and registers cleanup with t.
func NewDB(t *testing.T) *DB {
	t.Helper()

	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close db: %v", err)
		}
	})

	if err := db.Migrate(context.Background(), sqlDB); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &DB{t: t, SQL: sqlDB}
}

// DiscardLogger returns a slog logger that discards all output.
func DiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// Queries returns a sqlc query object bound to the test database.
func (d *DB) Queries() *db.Queries {
	return db.New(d.SQL)
}

// Config returns a default configuration for tests.
func (d *DB) Config() *config.Config {
	return &config.Config{
		Indexer: config.IndexerConfig{Concurrency: 2, MaxFileSize: 10 * 1024 * 1024},
		Search:  config.SearchConfig{MaxResults: 100, PathBoosting: true},
	}
}

// CreateRepo inserts a repository and returns it.
func (d *DB) CreateRepo(path, name string) db.Repository {
	d.t.Helper()

	repo, err := d.Queries().CreateRepo(context.Background(), db.CreateRepoParams{
		Path: path,
		Name: name,
	})
	if err != nil {
		d.t.Fatalf("create repo %s: %v", name, err)
	}
	return repo
}

// CreateDocument inserts a document under repoID and returns it.
func (d *DB) CreateDocument(repoID int64, path, language string) db.Document {
	d.t.Helper()

	doc, err := d.Queries().CreateDocument(context.Background(), db.CreateDocumentParams{
		RepoID:  repoID,
		Path:    path,
		Size:    int64(len(path)),
		Hash:    path,
		ModTime: time.Now(),
		MimeType: sql.NullString{
			String: "text/plain",
			Valid:  true,
		},
		Language: sql.NullString{String: language, Valid: language != ""},
	})
	if err != nil {
		d.t.Fatalf("create document %s: %v", path, err)
	}
	return doc
}

// CreateChunk inserts a chunk under docID; the FTS index updates via trigger.
func (d *DB) CreateChunk(docID int64, index int, content string) db.Chunk {
	d.t.Helper()

	chunk, err := d.Queries().CreateChunk(context.Background(), db.CreateChunkParams{
		DocID:      docID,
		ChunkIndex: int64(index),
		Content:    content,
	})
	if err != nil {
		d.t.Fatalf("create chunk: %v", err)
	}
	return chunk
}

// CreateSymbol inserts a symbol under docID and returns it.
func (d *DB) CreateSymbol(docID int64, name, kind string, line, col int) db.Symbol {
	d.t.Helper()

	sym, err := d.Queries().CreateSymbol(context.Background(), db.CreateSymbolParams{
		DocID: docID,
		Name:  name,
		Kind:  kind,
		Line:  int64(line),
		Col:   int64(col),
	})
	if err != nil {
		d.t.Fatalf("create symbol %s: %v", name, err)
	}
	return sym
}

// CreateRef inserts a reference under docID and returns it.
func (d *DB) CreateRef(docID int64, name string, line, col int, ctxText string) db.Ref {
	d.t.Helper()

	ref, err := d.Queries().CreateRef(context.Background(), db.CreateRefParams{
		DocID:   docID,
		Name:    name,
		Line:    int64(line),
		Col:     int64(col),
		Context: ctxText,
	})
	if err != nil {
		d.t.Fatalf("create ref %s: %v", name, err)
	}
	return ref
}

// CreateGraphEdge inserts a row into graph_edges.
func (d *DB) CreateGraphEdge(repoID, sourceDoc, targetDoc int64, kind string, weight int64) {
	d.t.Helper()

	const query = `INSERT INTO graph_edges (repo_id, source_doc_id, target_doc_id, kind, weight)
		VALUES (?, ?, ?, ?, ?)`
	if _, err := d.SQL.ExecContext(context.Background(), query, repoID, sourceDoc, targetDoc, kind, weight); err != nil {
		d.t.Fatalf("create graph edge: %v", err)
	}
}
