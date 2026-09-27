package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/indexer"
	"github.com/cryskram/relith/internal/reasoning"
	"github.com/cryskram/relith/internal/search"
)

type handlers struct {
	queries  *db.Queries
	indexer  *indexer.Indexer
	searcher *search.Searcher
	reasoner *reasoning.Engine
}

func (h *handlers) reason(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	repo := r.URL.Query().Get("repo")
	maxResults, _ := strconv.Atoi(r.URL.Query().Get("max_results"))
	bundle, err := h.reasoner.Trace(r.Context(), reasoning.TraceRequest{Query: query, RepoName: repo, MaxResults: maxResults})
	if err != nil {
		writeError(w, http.StatusBadRequest, "reason: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.queries.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stats: "+err.Error())
		return
	}

	rawBytes := toInt64(stats.TotalRawBytes)
	chunkBytes := toInt64(stats.TotalChunkBytes)

	rawMB := float64(rawBytes) / (1024 * 1024)
	chunkMB := float64(chunkBytes) / (1024 * 1024)
	var savingsPct float64
	if rawBytes > 0 {
		savingsPct = (1 - float64(chunkBytes)/float64(rawBytes)) * 100
	}

	writeJSON(w, http.StatusOK, struct {
		RepoCount       int64   `json:"repo_count"`
		DocCount        int64   `json:"doc_count"`
		ChunkCount      int64   `json:"chunk_count"`
		TotalRawBytes   int64   `json:"total_raw_bytes"`
		TotalChunkBytes int64   `json:"total_chunk_bytes"`
		RawMB           float64 `json:"raw_mb"`
		ChunkMB         float64 `json:"chunk_mb"`
		SavingsPct      float64 `json:"savings_pct"`
		SymbolCount     int64   `json:"symbol_count"`
		RefCount        int64   `json:"ref_count"`
	}{
		RepoCount:       stats.RepoCount,
		DocCount:        stats.DocCount,
		ChunkCount:      stats.ChunkCount,
		TotalRawBytes:   rawBytes,
		TotalChunkBytes: chunkBytes,
		RawMB:           round2(rawMB),
		ChunkMB:         round2(chunkMB),
		SavingsPct:      round2(savingsPct),
		SymbolCount:     stats.SymbolCount,
		RefCount:        stats.RefCount,
	})
}

func (h *handlers) graph(w http.ResponseWriter, r *http.Request) {
	repoName := r.URL.Query().Get("repo")

	repos, err := h.queries.ListRepos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list repos: "+err.Error())
		return
	}

	var repoID int64
	if repoName != "" {
		found := false
		for _, rp := range repos {
			if rp.Name == repoName {
				repoID = rp.ID
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "repo not found: "+repoName)
			return
		}
	}

	nodes, edges, err := h.buildGraph(r.Context(), repos, repoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "graph: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

type graphNode struct {
	ID     int64  `json:"id"`
	Label  string `json:"label"`
	Path   string `json:"path"`
	Group  string `json:"group"`
	RepoID int64  `json:"repo_id"`
	Size   int    `json:"size"`
}

type graphEdge struct {
	Source int64 `json:"source"`
	Target int64 `json:"target"`
	Weight int64 `json:"weight"`
}

func (h *handlers) buildGraph(ctx context.Context, repos []db.Repository, filterRepoID int64) ([]graphNode, []graphEdge, error) {
	var allNodes []graphNode
	var allEdges []graphEdge
	seenNodes := map[int64]bool{}
	docPaths := map[int64]string{}
	nodeDegree := map[int64]int{}

	for _, r := range repos {
		if filterRepoID > 0 && r.ID != filterRepoID {
			continue
		}
		edges, err := h.queries.GetGraphEdgesFromTable(ctx, r.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("graph edges for repo %s: %w", r.Name, err)
		}
		for _, e := range edges {
			docPaths[e.SourceID] = e.SourcePath
			docPaths[e.TargetID] = e.TargetPath
			nodeDegree[e.SourceID]++
			nodeDegree[e.TargetID]++
		}
	}

	// Trim paths
	trim := func(s string, n int) string {
		if len(s) > n {
			return "..." + s[len(s)-n+3:]
		}
		return s
	}
	for id, p := range docPaths {
		docPaths[id] = trim(p, 30)
	}

	// Limit to top 80 file nodes by degree
	type kv struct {
		id  int64
		deg int
	}
	var sorted []kv
	for id, deg := range nodeDegree {
		sorted = append(sorted, kv{id, deg})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].deg > sorted[i].deg {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if len(sorted) > 300 {
		sorted = sorted[:300]
	}
	keepFile := map[int64]bool{}
	for _, kv := range sorted {
		keepFile[kv.id] = true
	}

	for _, r := range repos {
		if filterRepoID > 0 && r.ID != filterRepoID {
			continue
		}
		edges, err := h.queries.GetGraphEdgesFromTable(ctx, r.ID)
		if err != nil {
			continue
		}
		if len(edges) == 0 {
			continue
		}

		var localEdges []graphEdge
		localFileIDs := map[int64]bool{}
		for _, e := range edges {
			if !keepFile[e.SourceID] && !keepFile[e.TargetID] {
				continue
			}
			localEdges = append(localEdges, graphEdge{
				Source: e.SourceID,
				Target: e.TargetID,
				Weight: e.Weight,
			})
			localFileIDs[e.SourceID] = true
			localFileIDs[e.TargetID] = true
		}
		if len(localEdges) == 0 {
			continue
		}

		// Repo node
		if !seenNodes[-r.ID] {
			allNodes = append(allNodes, graphNode{
				ID:     -r.ID,
				Label:  r.Name,
				Path:   r.Path,
				Group:  "repo",
				RepoID: r.ID,
				Size:   3,
			})
			seenNodes[-r.ID] = true
		}

		// File nodes
		for id := range localFileIDs {
			if seenNodes[id] {
				continue
			}
			allNodes = append(allNodes, graphNode{
				ID:     id,
				Label:  compactGraphPath(docPaths[id]),
				Path:   docPaths[id],
				Group:  "file",
				RepoID: r.ID,
				Size:   nodeDegree[id],
			})
			seenNodes[id] = true
		}

		allEdges = append(allEdges, localEdges...)

		// Link files to repo
		for id := range localFileIDs {
			allEdges = append(allEdges, graphEdge{
				Source: id,
				Target: -r.ID,
				Weight: 1,
			})
		}
	}

	if allNodes == nil {
		allNodes = []graphNode{}
	}
	if allEdges == nil {
		allEdges = []graphEdge{}
	}
	return allNodes, allEdges, nil
}

func compactGraphPath(path string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(parts) <= 2 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func (h *handlers) listRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.queries.ListRepos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list repos: "+err.Error())
		return
	}
	if repos == nil {
		repos = []db.Repository{}
	}
	writeJSON(w, http.StatusOK, repos)
}

func (h *handlers) createRepo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path      string `json:"path"`
		Name      string `json:"name"`
		RemoteURL string `json:"remote_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	repo, err := h.queries.CreateRepo(r.Context(), db.CreateRepoParams{
		Path:      req.Path,
		Name:      req.Name,
		RemoteUrl: sql.NullString{String: req.RemoteURL, Valid: req.RemoteURL != ""},
	})
	if err != nil {
		writeError(w, http.StatusConflict, "create repo: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, repo)
}

func (h *handlers) getRepo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repo id")
		return
	}
	repo, err := h.queries.GetRepo(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "repo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get repo: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (h *handlers) deleteRepo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repo id")
		return
	}
	if err := h.queries.DeleteRepo(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete repo: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) indexRepo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repo id")
		return
	}

	repo, err := h.queries.GetRepo(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "repo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get repo: "+err.Error())
		return
	}

	result, err := h.indexer.IndexRepo(r.Context(), repo.Path, repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "index: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"files_indexed": result.FilesIndexed,
		"files_skipped": result.FilesSkipped,
		"files_error":   result.FilesError,
		"total_chunks":  result.TotalChunks,
		"elapsed":       result.Elapsed.String(),
	})
}

func (h *handlers) content(w http.ResponseWriter, r *http.Request) {
	repoName := r.URL.Query().Get("repo")
	filePath := r.URL.Query().Get("path")
	if repoName == "" || filePath == "" {
		writeError(w, http.StatusBadRequest, "repo and path query params are required")
		return
	}

	repos, err := h.queries.ListRepos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list repos: "+err.Error())
		return
	}

	var repo db.Repository
	found := false
	for _, rp := range repos {
		if rp.Name == repoName {
			repo = rp
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "repo not found: "+repoName)
		return
	}

	doc, err := h.queries.GetDocumentByPath(r.Context(), db.GetDocumentByPathParams{
		RepoID: repo.ID,
		Path:   filePath,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found: "+filePath)
			return
		}
		writeError(w, http.StatusInternalServerError, "get document: "+err.Error())
		return
	}

	fullPath := filepath.Join(repo.Path, doc.Path)
	content, err := indexer.ReadFileContent(fullPath, 10*1024*1024)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read file: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}

func round2(v float64) float64 {
	return float64(int64(v*100)) / 100
}

func (h *handlers) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 0
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	results, err := h.searcher.SearchWithFilters(ctx, query, limit, search.Filters{
		RepoName:   r.URL.Query().Get("repo"),
		Language:   r.URL.Query().Get("language"),
		PathPrefix: r.URL.Query().Get("path"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, results)
}
