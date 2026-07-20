package indexer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/cryskram/relith/internal/chunker"
	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
)

type IndexResult struct {
	FilesIndexed int
	FilesSkipped int
	FilesError   int
	TotalChunks  int
	Elapsed      time.Duration
}

type Indexer struct {
	db     *sql.DB
	logger zerolog.Logger
	cfg    config.IndexerConfig
}

func New(database *sql.DB, logger zerolog.Logger, cfg config.IndexerConfig) *Indexer {
	return &Indexer{
		db:     database,
		logger: logger,
		cfg:    cfg,
	}
}

func (idx *Indexer) queries() *db.Queries {
	return db.New(idx.db)
}

func (idx *Indexer) IndexRepo(ctx context.Context, repoPath string, repoID int64) (IndexResult, error) {
	start := time.Now()
	idx.logger.Info().Str("path", repoPath).Int64("repo_id", repoID).Msg("indexing repo")

	q := idx.queries()
	if err := q.UpdateRepoStatus(ctx, db.UpdateRepoStatusParams{
		ID:     repoID,
		Status: "indexing",
	}); err != nil {
		return IndexResult{}, fmt.Errorf("set status indexing: %w", err)
	}

	files, err := WalkRepo(repoPath, idx.cfg.MaxFileSize)
	if err != nil {
		return IndexResult{}, fmt.Errorf("walk repo: %w", err)
	}

	idx.logger.Info().Int("files_found", len(files)).Msg("walk complete")

	existingDocs, err := q.ListDocuments(ctx, repoID)
	if err != nil {
		return IndexResult{}, fmt.Errorf("list existing docs: %w", err)
	}

	existingByPath := make(map[string]db.Document, len(existingDocs))
	for _, doc := range existingDocs {
		existingByPath[doc.Path] = doc
	}

	visited := sync.Map{}

	concurrency := idx.cfg.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	result := IndexResult{}
	var wg sync.WaitGroup

	for _, fi := range files {
		fi := fi
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			visited.Store(fi.RelPath, struct{}{})

			existing, exists := existingByPath[fi.RelPath]
			var existingPtr *db.Document
			if exists {
				if existing.Hash == idx.fileHash(ctx, fi.FullPath) {
					mu.Lock()
					result.FilesSkipped++
					mu.Unlock()
					return
				}
				existingPtr = &existing
			}

			n, err := idx.writeFile(ctx, repoID, fi.RelPath, fi.FullPath, fi.Size, fi.ModTime, existingPtr, nil)
			if err != nil {
				idx.logger.Error().Err(err).Str("file", fi.RelPath).Msg("processing file")
				mu.Lock()
				result.FilesError++
				mu.Unlock()
				return
			}
			mu.Lock()
			result.FilesIndexed++
			result.TotalChunks += n
			mu.Unlock()
		}()
	}
	wg.Wait()

	var toDelete []int64
	for _, doc := range existingDocs {
		if _, seen := visited.Load(doc.Path); !seen {
			toDelete = append(toDelete, doc.ID)
		}
	}
	for _, id := range toDelete {
		if err := q.DeleteDocument(ctx, id); err != nil {
			idx.logger.Error().Err(err).Int64("doc_id", id).Msg("delete stale document")
		}
	}

	now := time.Now()
	if err := q.UpdateRepoStatus(ctx, db.UpdateRepoStatusParams{
		ID:            repoID,
		Status:        "ready",
		LastIndexedAt: sql.NullTime{Time: now, Valid: true},
		FileCount:     int64(len(files) - result.FilesError),
	}); err != nil {
		return result, fmt.Errorf("set status ready: %w", err)
	}

	result.Elapsed = time.Since(start)
	idx.logger.Info().
		Int("indexed", result.FilesIndexed).
		Int("skipped", result.FilesSkipped).
		Int("errors", result.FilesError).
		Int("chunks", result.TotalChunks).
		Int("stale_removed", len(toDelete)).
		Dur("elapsed", result.Elapsed).
		Msg("indexing complete")

	return result, nil
}

func (idx *Indexer) IndexFile(ctx context.Context, repoID int64, relPath, fullPath string) error {
	idx.logger.Debug().Str("path", relPath).Int64("repo_id", repoID).Msg("index file")

	q := idx.queries()
	existing, err := q.GetDocumentByPath(ctx, db.GetDocumentByPathParams{
		RepoID: repoID,
		Path:   relPath,
	})
	exists := err == nil

	if exists {
		if existing.Hash == idx.fileHash(ctx, fullPath) {
			idx.logger.Debug().Str("path", relPath).Msg("file unchanged, skipping")
			return nil
		}
		_, err = idx.writeFile(ctx, repoID, relPath, fullPath, 0, 0, &existing, nil)
		return err
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("get existing doc: %w", err)
	}

	_, err = idx.writeFile(ctx, repoID, relPath, fullPath, 0, 0, nil, nil)
	return err
}

func (idx *Indexer) DeleteFile(ctx context.Context, repoID int64, relPath string) error {
	idx.logger.Debug().Str("path", relPath).Int64("repo_id", repoID).Msg("delete file")

	q := idx.queries()
	doc, err := q.GetDocumentByPath(ctx, db.GetDocumentByPathParams{
		RepoID: repoID,
		Path:   relPath,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("get doc: %w", err)
	}

	if err := q.DeleteDocument(ctx, doc.ID); err != nil {
		return fmt.Errorf("delete doc: %w", err)
	}

	q.UpdateRepoStatus(ctx, db.UpdateRepoStatusParams{
		Status: "ready",
		ID:     repoID,
	})
	return nil
}

func (idx *Indexer) fileHash(ctx context.Context, fullPath string) string {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (idx *Indexer) writeFile(
	ctx context.Context,
	repoID int64,
	relPath, fullPath string,
	size, modTime int64,
	existing *db.Document,
	result *IndexResult,
) (int, error) {
	if size <= 0 {
		info, err := os.Stat(fullPath)
		if err != nil {
			return 0, fmt.Errorf("stat: %w", err)
		}
		size = info.Size()
		modTime = info.ModTime().Unix()
	}

	content, err := ReadFileContent(fullPath, idx.cfg.MaxFileSize)
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	if content == "" {
		return 0, nil
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

	if existing != nil && existing.Hash == hash {
		return 0, nil
	}

	lang := DetectLanguage(relPath)
	mimeStr := ""
	if lang != "" {
		mimeStr = "text/" + lang
	}
	langStr := lang

	var chunks []chunker.Chunk
	langChunker := chunker.ForLanguage(lang)
	if langChunker != nil {
		chunks = langChunker(content)
	}
	if len(chunks) == 0 {
		return 0, nil
	}

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := idx.queries().WithTx(tx)

	var docID int64
	if existing != nil {
		if err := qtx.UpdateDocument(ctx, db.UpdateDocumentParams{
			ID:       existing.ID,
			Size:     size,
			Hash:     hash,
			ModTime:  time.Unix(modTime, 0),
			MimeType: sql.NullString{String: mimeStr, Valid: mimeStr != ""},
			Language: sql.NullString{String: langStr, Valid: langStr != ""},
		}); err != nil {
			return 0, fmt.Errorf("update doc: %w", err)
		}
		if err := qtx.DeleteChunksByDoc(ctx, existing.ID); err != nil {
			return 0, fmt.Errorf("delete chunks: %w", err)
		}
		if err := qtx.DeleteSymbolsByDoc(ctx, existing.ID); err != nil {
			return 0, fmt.Errorf("delete symbols: %w", err)
		}
		if err := qtx.DeleteRefsByDoc(ctx, existing.ID); err != nil {
			return 0, fmt.Errorf("delete refs: %w", err)
		}
		docID = existing.ID
	} else {
		doc, err := qtx.CreateDocument(ctx, db.CreateDocumentParams{
			RepoID:   repoID,
			Path:     relPath,
			Size:     size,
			Hash:     hash,
			ModTime:  time.Unix(modTime, 0),
			MimeType: sql.NullString{String: mimeStr, Valid: mimeStr != ""},
			Language: sql.NullString{String: langStr, Valid: langStr != ""},
		})
		if err != nil {
			return 0, fmt.Errorf("create doc: %w", err)
		}
		docID = doc.ID
	}

	for _, c := range chunks {
		if _, err := qtx.CreateChunk(ctx, db.CreateChunkParams{
			DocID:      docID,
			ChunkIndex: int64(c.Index),
			Content:    c.Content,
		}); err != nil {
			return 0, fmt.Errorf("create chunk: %w", err)
		}
		for _, sym := range c.Symbols {
			if _, err := qtx.CreateSymbol(ctx, db.CreateSymbolParams{
				DocID: docID,
				Name:  sym.Name,
				Kind:  sym.Kind,
				Line:  int64(sym.Line),
				Col:   int64(sym.Col),
			}); err != nil {
				return 0, fmt.Errorf("create symbol: %w", err)
			}
		}
	}

	refs := ExtractReferences(content)
	for _, r := range refs {
		if _, err := qtx.CreateRef(ctx, db.CreateRefParams{
			DocID:   docID,
			Name:    r.Name,
			Line:    int64(r.Line),
			Col:     int64(r.Col),
			Context: r.Context,
		}); err != nil {
			return 0, fmt.Errorf("create ref: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return len(chunks), nil
}

func IsGitRepo(path string) bool {
	info, err := os.Stat(path + "/.git")
	if err != nil {
		return false
	}
	return info.IsDir()
}
