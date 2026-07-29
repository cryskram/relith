package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"log/slog"

	"github.com/cryskram/relith/internal/config"
)

type Result struct {
	DocumentID int64   `json:"doc_id"`
	Path       string  `json:"path"`
	Language   string  `json:"language"`
	RepoName   string  `json:"repo_name"`
	ChunkIndex int64   `json:"chunk_index"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
}

type Searcher struct {
	db     *sql.DB
	logger *slog.Logger
	cfg    config.SearchConfig
}

func New(database *sql.DB, logger *slog.Logger, cfg config.SearchConfig) *Searcher {
	return &Searcher{
		db:     database,
		logger: logger,
		cfg:    cfg,
	}
}

func (s *Searcher) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	matchQuery := buildMatchQuery(query)
	if matchQuery == "" {
		return nil, nil
	}

	if limit <= 0 {
		limit = s.cfg.MaxResults
	}
	if limit < 1 {
		limit = 10
	}

	orderClause := "rank"
	var pathBoostTerm string
	if s.cfg.PathBoosting {
		orderClause = `rank + CASE WHEN d.path LIKE ? THEN -10.0 ELSE 0.0 END`
		pathBoostTerm = fmt.Sprintf("%%%s%%", likeEscape(query))
	}

	sqlQuery := fmt.Sprintf(`
		SELECT c.id, c.chunk_index, c.content,
			   d.id, d.path, d.language,
			   r.name, rank
		FROM chunks_fts f
		JOIN chunks c ON c.id = f.rowid
		JOIN documents d ON d.id = c.doc_id
		JOIN repositories r ON r.id = d.repo_id
		WHERE chunks_fts MATCH ?
		ORDER BY %s
		LIMIT ?`, orderClause)

	s.logger.Debug("search", "match_query", matchQuery, "limit", limit)

	var rows *sql.Rows
	var err error

	if s.cfg.PathBoosting {
		rows, err = s.db.QueryContext(ctx, sqlQuery, matchQuery, pathBoostTerm, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, sqlQuery, matchQuery, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		var content string
		var lang, repoName sql.NullString
		var rawDocID int64
		if err := rows.Scan(
			&r.DocumentID,
			&r.ChunkIndex,
			&content,
			&rawDocID,
			&r.Path,
			&lang,
			&repoName,
			&r.Score,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		r.Language = lang.String
		r.RepoName = repoName.String
		r.Content = truncateContent(content, 500)
		r.Score = -r.Score
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if results == nil {
		results = []Result{}
	}
	return results, nil
}

func truncateContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen]) + "..."
}

func likeEscape(s string) string {
	return strings.NewReplacer(
		`%`, `\%`,
		`_`, `\_`,
		`\`, `\\`,
	).Replace(s)
}
