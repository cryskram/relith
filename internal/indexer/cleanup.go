package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func DeleteDocuments(ctx context.Context, db *sql.DB, repoID int64, docIDs []int64) error {
	if len(docIDs) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	placeholders := make([]string, len(docIDs))
	args := make([]interface{}, len(docIDs)+1)
	args[0] = repoID
	for i, id := range docIDs {
		placeholders[i] = "?"
		args[i+1] = id
	}
	inClause := strings.Join(placeholders, ", ")

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM graph_edges WHERE source_doc_id IN (`+inClause+`) OR target_doc_id IN (`+inClause+`)`,
		append(args, args[1:]...)...,
	); err != nil {
		return fmt.Errorf("delete graph_edges: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM chunks WHERE doc_id IN (`+inClause+`)`,
		args...,
	); err != nil {
		return fmt.Errorf("delete chunks: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM symbols WHERE doc_id IN (`+inClause+`)`,
		args...,
	); err != nil {
		return fmt.Errorf("delete symbols: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM refs WHERE doc_id IN (`+inClause+`)`,
		args...,
	); err != nil {
		return fmt.Errorf("delete refs: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM documents WHERE repo_id = ? AND id IN (`+inClause+`)`,
		args...,
	); err != nil {
		return fmt.Errorf("delete documents: %w", err)
	}

	return tx.Commit()
}

func DeleteRepoWithData(ctx context.Context, db *sql.DB, repoID int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM graph_edges WHERE repo_id = ?`, repoID); err != nil {
		return fmt.Errorf("delete graph_edges: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE doc_id IN (SELECT id FROM documents WHERE repo_id = ?)`, repoID); err != nil {
		return fmt.Errorf("delete chunks: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM symbols WHERE doc_id IN (SELECT id FROM documents WHERE repo_id = ?)`, repoID); err != nil {
		return fmt.Errorf("delete symbols: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM refs WHERE doc_id IN (SELECT id FROM documents WHERE repo_id = ?)`, repoID); err != nil {
		return fmt.Errorf("delete refs: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE repo_id = ?`, repoID); err != nil {
		return fmt.Errorf("delete documents: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM repositories WHERE id = ?`, repoID); err != nil {
		return fmt.Errorf("delete repo: %w", err)
	}

	return tx.Commit()
}
