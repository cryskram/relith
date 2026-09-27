package indexer

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/testutil"
)

// TestIndexFileDeleteFileRepoCounters verifies that IndexFile keeps
// repositories.file_count / last_indexed_at consistent and DeleteFile
// decrements the counter without relying on stale status updates.
func TestIndexFileDeleteFileRepoCounters(t *testing.T) {
	fdb := testutil.NewDB(t)
	fixture := fdb.CreateRepo("/tmp/repo-x", "repo-x")

	idx := New(fdb.SQL, testutil.DiscardLogger(), config.IndexerConfig{Concurrency: 2, MaxFileSize: 10 * 1024 * 1024})
	ctx := context.Background()

	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(fileA, []byte(strings.Repeat("hello\n", 60)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte(strings.Repeat("world\n", 60)), 0644); err != nil {
		t.Fatal(err)
	}

	if err := idx.IndexFile(ctx, fixture.ID, "a.txt", fileA); err != nil {
		t.Fatalf("IndexFile a.txt: %v", err)
	}
	if err := idx.IndexFile(ctx, fixture.ID, "b.txt", fileB); err != nil {
		t.Fatalf("IndexFile b.txt: %v", err)
	}

	repo, err := fdb.Queries().GetRepo(ctx, fixture.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repo.FileCount != 2 {
		t.Errorf("expected file_count 2 after two IndexFile calls, got %d", repo.FileCount)
	}
	if !repo.LastIndexedAt.Valid {
		t.Error("expected last_indexed_at to be set after IndexFile")
	}

	// Unchanged content: IndexFile skips early, count must not change.
	if err := idx.IndexFile(ctx, fixture.ID, "a.txt", fileA); err != nil {
		t.Fatalf("re-IndexFile a.txt: %v", err)
	}

	// Modified content: doc update, count must not double-increment.
	if err := os.WriteFile(fileA, []byte(strings.Repeat("hello\n", 70)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile(ctx, fixture.ID, "a.txt", fileA); err != nil {
		t.Fatalf("update IndexFile a.txt: %v", err)
	}

	repo, err = fdb.Queries().GetRepo(ctx, fixture.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repo.FileCount != 2 {
		t.Errorf("expected file_count still 2 after update, got %d", repo.FileCount)
	}

	// Delete one file: count decrements, document is removed (FK CASCADE).
	if err := idx.DeleteFile(ctx, fixture.ID, "a.txt"); err != nil {
		t.Fatalf("DeleteFile a.txt: %v", err)
	}

	repo, err = fdb.Queries().GetRepo(ctx, fixture.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repo.FileCount != 1 {
		t.Errorf("expected file_count 1 after delete, got %d", repo.FileCount)
	}
	if !repo.LastIndexedAt.Valid {
		t.Error("expected last_indexed_at to be set after DeleteFile")
	}

	_, err = fdb.Queries().GetDocumentByPath(ctx, db.GetDocumentByPathParams{RepoID: fixture.ID, Path: "a.txt"})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected a.txt document to be removed, got err %v", err)
	}

	// Deleting a non-existent file is a no-op.
	if err := idx.DeleteFile(ctx, fixture.ID, "missing.txt"); err != nil {
		t.Fatalf("DeleteFile missing.txt: %v", err)
	}
}
