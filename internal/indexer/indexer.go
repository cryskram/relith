package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
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

	visited := make(map[string]struct{}, len(files))
	result := IndexResult{}

	batchSize := 500
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}
		batch := files[i:end]

		var work []batchWork
		jobs := make(chan batchJob, len(batch))
		resultsCh := make(chan batchResult, len(batch))
		workers := idx.cfg.Concurrency
		if workers < 1 {
			workers = 1
		}
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					prep, skipped, err := idx.prepareBatchWork(job.fi, job.existing)
					resultsCh <- batchResult{work: prep, skipped: skipped, err: err, relPath: job.fi.RelPath}
				}
			}()
		}
		go func() {
			wg.Wait()
			close(resultsCh)
		}()

		for _, fi := range batch {
			visited[fi.RelPath] = struct{}{}
			existing, _ := existingByPath[fi.RelPath]
			jobs <- batchJob{fi: fi, existing: existing}
		}
		close(jobs)

		for res := range resultsCh {
			if res.err != nil {
				idx.logger.Error().Err(res.err).Str("file", res.relPath).Msg("prepare file")
				result.FilesError++
				continue
			}
			if res.skipped {
				result.FilesSkipped++
				continue
			}
			work = append(work, res.work)
			result.FilesIndexed++
			result.TotalChunks += len(res.work.chunks)
		}

		if len(work) == 0 {
			continue
		}

		if err := idx.writeBatch(ctx, repoID, work, existingByPath); err != nil {
			return result, fmt.Errorf("write batch: %w", err)
		}
	}

	var toDelete []int64
	for _, doc := range existingDocs {
		if _, seen := visited[doc.Path]; !seen {
			toDelete = append(toDelete, doc.ID)
		}
	}
	for _, id := range toDelete {
		if err := q.DeleteDocument(ctx, id); err != nil {
			idx.logger.Error().Err(err).Int64("doc_id", id).Msg("delete stale document")
		}
	}

	now := time.Now()
	if err := idx.BuildGraphForRepo(ctx, repoID, repoPath); err != nil {
		idx.logger.Warn().Err(err).Msg("graph build failed (non-fatal)")
	}

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

	var existingPtr *db.Document
	if exists {
		if existing.Hash == fastHashFile(fullPath) {
			idx.logger.Debug().Str("path", relPath).Msg("file unchanged, skipping")
			return nil
		}
		existingPtr = &existing
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("get existing doc: %w", err)
	}

	prep, skipped, err := idx.prepareBatchWork(FileInfo{RelPath: relPath, FullPath: fullPath}, existing)
	if err != nil {
		return err
	}
	if skipped {
		return nil
	}
	content := prep.content
	hash := prep.hash
	lang := prep.lang
	chunks := prep.chunks
	refs := prep.refs

	langStr := lang
	mimeStr := ""
	if lang != "" {
		mimeStr = "text/" + lang
	}

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := idx.queries().WithTx(tx)

	var docID int64
	if existingPtr != nil {
		if err := qtx.UpdateDocument(ctx, db.UpdateDocumentParams{
			ID:       existingPtr.ID,
			Size:     int64(len(content)),
			Hash:     hash,
			ModTime:  time.Now(),
			MimeType: sql.NullString{String: mimeStr, Valid: mimeStr != ""},
			Language: sql.NullString{String: langStr, Valid: langStr != ""},
		}); err != nil {
			return fmt.Errorf("update doc: %w", err)
		}
		if err := qtx.DeleteChunksByDoc(ctx, existingPtr.ID); err != nil {
			return fmt.Errorf("delete chunks: %w", err)
		}
		if err := qtx.DeleteSymbolsByDoc(ctx, existingPtr.ID); err != nil {
			return fmt.Errorf("delete symbols: %w", err)
		}
		if err := qtx.DeleteRefsByDoc(ctx, existingPtr.ID); err != nil {
			return fmt.Errorf("delete refs: %w", err)
		}
		docID = existingPtr.ID
	} else {
		doc, err := qtx.CreateDocument(ctx, db.CreateDocumentParams{
			RepoID:   repoID,
			Path:     relPath,
			Size:     int64(len(content)),
			Hash:     hash,
			ModTime:  time.Now(),
			MimeType: sql.NullString{String: mimeStr, Valid: mimeStr != ""},
			Language: sql.NullString{String: langStr, Valid: langStr != ""},
		})
		if err != nil {
			return fmt.Errorf("create doc: %w", err)
		}
		docID = doc.ID
	}

	for _, c := range chunks {
		if _, err := qtx.CreateChunk(ctx, db.CreateChunkParams{
			DocID:      docID,
			ChunkIndex: int64(c.Index),
			Content:    c.Content,
		}); err != nil {
			return fmt.Errorf("create chunk: %w", err)
		}
		for _, sym := range c.Symbols {
			if _, err := qtx.CreateSymbol(ctx, db.CreateSymbolParams{
				DocID: docID,
				Name:  sym.Name,
				Kind:  sym.Kind,
				Line:  int64(sym.Line),
				Col:   int64(sym.Col),
			}); err != nil {
				return fmt.Errorf("create symbol: %w", err)
			}
		}
	}

	for _, r := range refs {
		if _, err := qtx.CreateRef(ctx, db.CreateRefParams{
			DocID:   docID,
			Name:    r.Name,
			Line:    int64(r.Line),
			Col:     int64(r.Col),
			Context: r.Context,
		}); err != nil {
			return fmt.Errorf("create ref: %w", err)
		}
	}

	repoPath := strings.TrimSuffix(fullPath, relPath)
	repoPath = strings.TrimSuffix(repoPath, "/")
	if err := idx.updateGraphForFile(ctx, repoID, repoPath, relPath, docID); err != nil {
		idx.logger.Warn().Err(err).Str("path", relPath).Msg("graph update failed (non-fatal)")
	}

	return tx.Commit()
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

func fastHash(content string) string {
	h := fnv.New64a()
	h.Write([]byte(content))
	return fmt.Sprintf("%016x", h.Sum(nil))
}

func fastHashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return fastHash(string(data))
}

type batchWork struct {
	relPath  string
	fullPath string
	content  string
	hash     string
	chunks   []chunker.Chunk
	refs     []Ref
	lang     string
	size     int64
	modTime  int64
}

type batchJob struct {
	fi       FileInfo
	existing db.Document
}

type batchResult struct {
	work    batchWork
	skipped bool
	err     error
	relPath string
}

func (idx *Indexer) prepareBatchWork(fi FileInfo, existing db.Document) (batchWork, bool, error) {
	if existing.Path != "" && existing.Size == fi.Size && existing.ModTime.Unix() == fi.ModTime {
		return batchWork{}, true, nil
	}

	content, err := ReadFileContent(fi.FullPath, idx.cfg.MaxFileSize)
	if err != nil {
		return batchWork{}, false, fmt.Errorf("read %s: %w", fi.RelPath, err)
	}
	if content == "" {
		return batchWork{}, true, nil
	}

	hash := fastHash(content)
	if existing.Path != "" && existing.Hash == hash {
		return batchWork{}, true, nil
	}

	lang := DetectLanguage(fi.RelPath)
	langChunker := chunker.ForLanguage(lang)
	var chunks []chunker.Chunk
	if langChunker != nil {
		chunks = langChunker(content)
	}
	if len(chunks) == 0 {
		return batchWork{}, true, nil
	}

	refs := ExtractReferences(content)

	return batchWork{
		relPath:  fi.RelPath,
		fullPath: fi.FullPath,
		content:  content,
		hash:     hash,
		chunks:   chunks,
		refs:     refs,
		lang:     lang,
		size:     fi.Size,
		modTime:  fi.ModTime,
	}, false, nil
}

func (idx *Indexer) writeBatch(
	ctx context.Context,
	repoID int64,
	work []batchWork,
	existingByPath map[string]db.Document,
) error {
	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := idx.queries().WithTx(tx)

	for _, w := range work {
		existing := existingByPath[w.relPath]

		mimeStr := ""
		if w.lang != "" {
			mimeStr = "text/" + w.lang
		}

		var docID int64
		if existing.Path != "" {
			if err := qtx.UpdateDocument(ctx, db.UpdateDocumentParams{
				ID:       existing.ID,
				Size:     w.size,
				Hash:     w.hash,
				ModTime:  time.Unix(w.modTime, 0),
				MimeType: sql.NullString{String: mimeStr, Valid: mimeStr != ""},
				Language: sql.NullString{String: w.lang, Valid: w.lang != ""},
			}); err != nil {
				return fmt.Errorf("update doc %s: %w", w.relPath, err)
			}
			if err := qtx.DeleteChunksByDoc(ctx, existing.ID); err != nil {
				return fmt.Errorf("delete chunks %s: %w", w.relPath, err)
			}
			if err := qtx.DeleteSymbolsByDoc(ctx, existing.ID); err != nil {
				return fmt.Errorf("delete symbols %s: %w", w.relPath, err)
			}
			if err := qtx.DeleteRefsByDoc(ctx, existing.ID); err != nil {
				return fmt.Errorf("delete refs %s: %w", w.relPath, err)
			}
			docID = existing.ID
		} else {
			doc, err := qtx.CreateDocument(ctx, db.CreateDocumentParams{
				RepoID:   repoID,
				Path:     w.relPath,
				Size:     w.size,
				Hash:     w.hash,
				ModTime:  time.Unix(w.modTime, 0),
				MimeType: sql.NullString{String: mimeStr, Valid: mimeStr != ""},
				Language: sql.NullString{String: w.lang, Valid: w.lang != ""},
			})
			if err != nil {
				return fmt.Errorf("create doc %s: %w", w.relPath, err)
			}
			docID = doc.ID
		}

		for _, c := range w.chunks {
			if _, err := qtx.CreateChunk(ctx, db.CreateChunkParams{
				DocID:      docID,
				ChunkIndex: int64(c.Index),
				Content:    c.Content,
			}); err != nil {
				return fmt.Errorf("create chunk %s: %w", w.relPath, err)
			}
			for _, sym := range c.Symbols {
				if _, err := qtx.CreateSymbol(ctx, db.CreateSymbolParams{
					DocID: docID,
					Name:  sym.Name,
					Kind:  sym.Kind,
					Line:  int64(sym.Line),
					Col:   int64(sym.Col),
				}); err != nil {
					return fmt.Errorf("create symbol %s: %w", w.relPath, err)
				}
			}
		}

		for _, r := range w.refs {
			if _, err := qtx.CreateRef(ctx, db.CreateRefParams{
				DocID:   docID,
				Name:    r.Name,
				Line:    int64(r.Line),
				Col:     int64(r.Col),
				Context: r.Context,
			}); err != nil {
				return fmt.Errorf("create ref %s: %w", w.relPath, err)
			}
		}
	}

	return tx.Commit()
}

func IsGitRepo(path string) bool {
	info, err := os.Stat(path + "/.git")
	if err != nil {
		return false
	}
	return info.IsDir()
}
