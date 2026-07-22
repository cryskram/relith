package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/indexer"
	"github.com/cryskram/relith/internal/reasoning"
)

func (s *Server) handleSearchCode(ctx context.Context, params map[string]any) CallToolResult {
	query := strParam(params, "query")
	if query == "" {
		return s.errorContent("query is required")
	}

	ok, err := s.hasRepos(ctx)
	if err != nil {
		return s.errorContent("check repos: " + err.Error())
	}
	if !ok {
		return s.textContent("No results found. " + noReposHelp)
	}

	repoName := strParam(params, "repo_name")
	language := strParam(params, "language")
	maxResults := intParam(params, "max_results", 20)

	results, err := s.searcher.Search(ctx, query, maxResults)
	if err != nil {
		return s.errorContent(fmt.Sprintf("search failed: %v", err))
	}

	var filtered []struct {
		DocID    int64   `json:"doc_id"`
		Path     string  `json:"path"`
		Language string  `json:"language"`
		RepoName string  `json:"repo_name"`
		Content  string  `json:"content"`
		Score    float64 `json:"score"`
	}

	for _, r := range results {
		if repoName != "" && r.RepoName != repoName {
			continue
		}
		if language != "" && !strings.EqualFold(r.Language, language) {
			continue
		}
		filtered = append(filtered, struct {
			DocID    int64   `json:"doc_id"`
			Path     string  `json:"path"`
			Language string  `json:"language"`
			RepoName string  `json:"repo_name"`
			Content  string  `json:"content"`
			Score    float64 `json:"score"`
		}{
			DocID: r.DocumentID, Path: r.Path, Language: r.Language,
			RepoName: r.RepoName, Content: r.Content, Score: r.Score,
		})
	}

	if len(filtered) == 0 {
		return s.textContent("No results found.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d result(s):\n\n", len(filtered)))
	for i, r := range filtered {
		sb.WriteString(fmt.Sprintf("--- Result %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Repo: %s\n", r.RepoName))
		sb.WriteString(fmt.Sprintf("File: %s\n", r.Path))
		sb.WriteString(fmt.Sprintf("Language: %s\n", r.Language))
		sb.WriteString(fmt.Sprintf("Score: %.2f\n", r.Score))
		sb.WriteString(fmt.Sprintf("Content:\n%s\n\n", r.Content))
	}

	return s.textContent(sb.String())
}

func (s *Server) handleFindReferences(ctx context.Context, params map[string]any) CallToolResult {
	name := strParam(params, "name")
	if name == "" {
		return s.errorContent("name is required")
	}

	repoName := strParam(params, "repo_name")

	ok, err := s.hasRepos(ctx)
	if err != nil {
		return s.errorContent("check repos: " + err.Error())
	}
	if !ok {
		return s.textContent("No references found. " + noReposHelp)
	}

	type refRow struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		RepoName string `json:"repo_name"`
		Line     int64  `json:"line"`
		Col      int64  `json:"col"`
		Context  string `json:"context"`
	}

	var rows []refRow

	if repoName != "" {
		data, qErr := s.queries.FindRefsByNameAndRepo(ctx, db.FindRefsByNameAndRepoParams{
			Name:    repoName,
			Column2: sql.NullString{String: name, Valid: true},
		})
		if qErr != nil {
			return s.errorContent(fmt.Sprintf("search refs failed: %v", qErr))
		}
		for _, r := range data {
			rows = append(rows, refRow{
				Name: r.Name, Path: r.Path,
				RepoName: r.RepoName, Line: r.Line,
				Col: r.Col, Context: r.Context,
			})
		}
	} else {
		data, qErr := s.queries.FindRefsByName(ctx, sql.NullString{String: name, Valid: true})
		if qErr != nil {
			return s.errorContent(fmt.Sprintf("search refs failed: %v", qErr))
		}
		for _, r := range data {
			rows = append(rows, refRow{
				Name: r.Name, Path: r.Path,
				RepoName: r.RepoName, Line: r.Line,
				Col: r.Col, Context: r.Context,
			})
		}
	}

	if len(rows) == 0 {
		return s.textContent("No references found matching: " + name)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d reference(s) to %q:\n\n", len(rows), name))
	for i, r := range rows {
		sb.WriteString(fmt.Sprintf("--- %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Context: %s\n", r.Context))
		sb.WriteString(fmt.Sprintf("File:   %s\n", r.Path))
		sb.WriteString(fmt.Sprintf("Repo:   %s\n", r.RepoName))
		sb.WriteString(fmt.Sprintf("Line:   %d : %d\n\n", r.Line, r.Col))
	}

	return s.textContent(sb.String())
}

func (s *Server) handleTraceContext(ctx context.Context, params map[string]any) CallToolResult {
	query := strParam(params, "query")
	if query == "" {
		return s.errorContent("query is required")
	}

	repoName := strParam(params, "repo_name")
	maxResults := intParam(params, "max_results", 8)
	bundle, err := s.reasoner.Trace(ctx, reasoning.TraceRequest{Query: query, RepoName: repoName, MaxResults: maxResults})
	if err != nil {
		return s.errorContent(fmt.Sprintf("trace context failed: %v", err))
	}
	return s.textContent(bundle.Text())
}

func (s *Server) handleGetFileOutline(ctx context.Context, params map[string]any) CallToolResult {
	repoName := strParam(params, "repo_name")
	filePath := strParam(params, "path")
	if repoName == "" || filePath == "" {
		return s.errorContent("repo_name and path are required")
	}

	repo, err := s.findRepo(ctx, repoName)
	if err != nil {
		return s.errorContent(err.Error())
	}

	doc, err := s.queries.GetDocumentByPath(ctx, db.GetDocumentByPathParams{RepoID: repo.ID, Path: filePath})
	if err != nil {
		return s.errorContent(fmt.Sprintf("file not found: %s in %s", filePath, repoName))
	}

	chunks, err := s.queries.ListChunks(ctx, doc.ID)
	if err != nil {
		return s.errorContent(fmt.Sprintf("list chunks failed: %v", err))
	}

	symbolRows, err := s.db.QueryContext(ctx, `SELECT id, name, kind, line, col FROM symbols WHERE doc_id = ? ORDER BY line`, doc.ID)
	if err != nil {
		return s.errorContent(fmt.Sprintf("list symbols failed: %v", err))
	}
	defer symbolRows.Close()

	type symbol struct {
		ID   int64
		Name string
		Kind string
		Line int64
		Col  int64
	}
	var symbols []symbol
	for symbolRows.Next() {
		var srow symbol
		if err := symbolRows.Scan(&srow.ID, &srow.Name, &srow.Kind, &srow.Line, &srow.Col); err != nil {
			return s.errorContent(fmt.Sprintf("scan symbols failed: %v", err))
		}
		symbols = append(symbols, srow)
	}

	refRows, err := s.db.QueryContext(ctx, `SELECT id, name, line, col, context FROM refs WHERE doc_id = ? ORDER BY line LIMIT 40`, doc.ID)
	if err != nil {
		return s.errorContent(fmt.Sprintf("list refs failed: %v", err))
	}
	defer refRows.Close()

	type ref struct {
		ID      int64
		Name    string
		Line    int64
		Col     int64
		Context string
	}
	var refs []ref
	for refRows.Next() {
		var r ref
		if err := refRows.Scan(&r.ID, &r.Name, &r.Line, &r.Col, &r.Context); err != nil {
			return s.errorContent(fmt.Sprintf("scan refs failed: %v", err))
		}
		refs = append(refs, r)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File Outline: %s/%s\n", repo.Name, doc.Path))
	sb.WriteString(fmt.Sprintf("Language: %s\n", doc.Language.String))
	sb.WriteString(fmt.Sprintf("Size: %d bytes\n", doc.Size))
	sb.WriteString(fmt.Sprintf("Chunks: %d\n\n", len(chunks)))

	sb.WriteString("Symbols:\n")
	if len(symbols) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, sym := range symbols {
			sb.WriteString(fmt.Sprintf("  - %s (%s) at %d:%d\n", sym.Name, sym.Kind, sym.Line, sym.Col))
		}
	}

	sb.WriteString("\nReferences:\n")
	if len(refs) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, r := range refs {
			sb.WriteString(fmt.Sprintf("  - %s at %d:%d\n    %s\n", r.Name, r.Line, r.Col, r.Context))
		}
	}

	return s.textContent(sb.String())
}

func (s *Server) handleFindCallers(ctx context.Context, params map[string]any) CallToolResult {
	name := strParam(params, "name")
	if name == "" {
		return s.errorContent("name is required")
	}
	repoName := strParam(params, "repo_name")
	maxResults := intParam(params, "max_results", 20)

	query := `SELECT r.id, r.doc_id, r.name, r.line, r.col, r.context, d.path, r2.name AS repo_name
FROM refs r
JOIN documents d ON d.id = r.doc_id
JOIN repositories r2 ON r2.id = d.repo_id
WHERE r.name = ?`
	args := []any{name}
	if repoName != "" {
		query += ` AND r2.name = ?`
		args = append(args, repoName)
	}
	query += ` ORDER BY r2.name, d.path, r.line LIMIT ?`
	args = append(args, maxResults)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return s.errorContent(fmt.Sprintf("find callers failed: %v", err))
	}
	defer rows.Close()

	type caller struct {
		Path     string
		RepoName string
		Line     int64
		Col      int64
		Context  string
	}
	var callers []caller
	for rows.Next() {
		var c caller
		var id, docID int64
		var refName string
		if err := rows.Scan(&id, &docID, &refName, &c.Line, &c.Col, &c.Context, &c.Path, &c.RepoName); err != nil {
			return s.errorContent(fmt.Sprintf("scan callers failed: %v", err))
		}
		callers = append(callers, c)
	}

	if len(callers) == 0 {
		return s.textContent("No callers found for: " + name)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Callers for %q:\n\n", name))
	for i, c := range callers {
		sb.WriteString(fmt.Sprintf("--- %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Repo:   %s\n", c.RepoName))
		sb.WriteString(fmt.Sprintf("File:   %s\n", c.Path))
		sb.WriteString(fmt.Sprintf("Line:   %d:%d\n", c.Line, c.Col))
		sb.WriteString(fmt.Sprintf("Context: %s\n\n", c.Context))
	}
	return s.textContent(sb.String())
}

func (s *Server) handleGetRelatedFiles(ctx context.Context, params map[string]any) CallToolResult {
	repoName := strParam(params, "repo_name")
	filePath := strParam(params, "path")
	maxResults := intParam(params, "max_results", 12)
	if repoName == "" || filePath == "" {
		return s.errorContent("repo_name and path are required")
	}

	repo, err := s.findRepo(ctx, repoName)
	if err != nil {
		return s.errorContent(err.Error())
	}

	doc, err := s.queries.GetDocumentByPath(ctx, db.GetDocumentByPathParams{RepoID: repo.ID, Path: filePath})
	if err != nil {
		return s.errorContent(fmt.Sprintf("file not found: %s in %s", filePath, repoName))
	}

	edges, err := s.queries.GetGraphEdges(ctx, repo.ID)
	if err != nil {
		return s.errorContent(fmt.Sprintf("graph edges failed: %v", err))
	}

	type related struct {
		Path   string
		Weight int64
	}
	byPath := map[string]int64{}
	for _, e := range edges {
		switch {
		case e.SourceID == doc.ID:
			byPath[e.TargetPath] += e.Weight
		case e.TargetID == doc.ID:
			byPath[e.SourcePath] += e.Weight
		}
	}

	var relatedFiles []related
	for path, weight := range byPath {
		relatedFiles = append(relatedFiles, related{Path: path, Weight: weight})
	}
	sort.Slice(relatedFiles, func(i, j int) bool {
		if relatedFiles[i].Weight == relatedFiles[j].Weight {
			return relatedFiles[i].Path < relatedFiles[j].Path
		}
		return relatedFiles[i].Weight > relatedFiles[j].Weight
	})
	if len(relatedFiles) > maxResults {
		relatedFiles = relatedFiles[:maxResults]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Related files for %s/%s:\n\n", repo.Name, doc.Path))
	if len(relatedFiles) == 0 {
		sb.WriteString("  (none)\n")
		return s.textContent(sb.String())
	}
	for i, r := range relatedFiles {
		sb.WriteString(fmt.Sprintf("--- %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Path:   %s\n", r.Path))
		sb.WriteString(fmt.Sprintf("Weight: %d\n\n", r.Weight))
	}
	return s.textContent(sb.String())
}

func (s *Server) handleListHubFiles(ctx context.Context, params map[string]any) CallToolResult {
	repoName := strParam(params, "repo_name")
	maxResults := intParam(params, "max_results", 15)

	repos, err := s.queries.ListRepos(ctx)
	if err != nil {
		return s.errorContent(fmt.Sprintf("list repos failed: %v", err))
	}

	var repoIDs []db.Repository
	for _, r := range repos {
		if repoName == "" || r.Name == repoName {
			repoIDs = append(repoIDs, r)
		}
	}
	if len(repoIDs) == 0 {
		return s.textContent("No repositories matched.")
	}

	type hub struct {
		RepoName string
		Path     string
		Degree   int64
	}
	var hubs []hub
	for _, repo := range repoIDs {
		edges, err := s.queries.GetGraphEdges(ctx, repo.ID)
		if err != nil {
			continue
		}
		degrees := map[int64]*hub{}
		for _, e := range edges {
			if _, ok := degrees[e.SourceID]; !ok {
				degrees[e.SourceID] = &hub{RepoName: repo.Name, Path: e.SourcePath}
			}
			if _, ok := degrees[e.TargetID]; !ok {
				degrees[e.TargetID] = &hub{RepoName: repo.Name, Path: e.TargetPath}
			}
			degrees[e.SourceID].Degree += e.Weight
			degrees[e.TargetID].Degree += e.Weight
		}
		for _, h := range degrees {
			hubs = append(hubs, *h)
		}
	}

	sort.Slice(hubs, func(i, j int) bool {
		if hubs[i].Degree == hubs[j].Degree {
			if hubs[i].RepoName == hubs[j].RepoName {
				return hubs[i].Path < hubs[j].Path
			}
			return hubs[i].RepoName < hubs[j].RepoName
		}
		return hubs[i].Degree > hubs[j].Degree
	})
	if len(hubs) > maxResults {
		hubs = hubs[:maxResults]
	}

	var sb strings.Builder
	sb.WriteString("Hub files:\n\n")
	for i, h := range hubs {
		sb.WriteString(fmt.Sprintf("--- %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Repo:   %s\n", h.RepoName))
		sb.WriteString(fmt.Sprintf("Path:   %s\n", h.Path))
		sb.WriteString(fmt.Sprintf("Degree: %d\n\n", h.Degree))
	}
	if len(hubs) == 0 {
		sb.WriteString("  (none)\n")
	}
	return s.textContent(sb.String())
}

func (s *Server) handleGetFileContent(ctx context.Context, params map[string]any) CallToolResult {
	repoName := strParam(params, "repo_name")
	filePath := strParam(params, "path")

	if repoName == "" || filePath == "" {
		return s.errorContent("repo_name and path are required")
	}

	ok, err := s.hasRepos(ctx)
	if err != nil {
		return s.errorContent("check repos: " + err.Error())
	}
	if !ok {
		return s.errorContent("repository not found: " + repoName + ". " + noReposHelp)
	}

	repos, err := s.queries.ListRepos(ctx)
	if err != nil {
		return s.errorContent(fmt.Sprintf("failed to list repos: %v", err))
	}

	var repo db.Repository
	found := false
	for _, r := range repos {
		if r.Name == repoName {
			repo = r
			found = true
			break
		}
	}
	if !found {
		return s.errorContent(fmt.Sprintf("repository not found: %s", repoName))
	}

	doc, err := s.queries.GetDocumentByPath(ctx, db.GetDocumentByPathParams{
		RepoID: repo.ID,
		Path:   filePath,
	})
	if err != nil {
		return s.errorContent(fmt.Sprintf("file not found: %s in %s", filePath, repoName))
	}

	fullPath := filepath.Join(repo.Path, doc.Path)
	content, err := indexer.ReadFileContent(fullPath, 10*1024*1024)
	if err != nil {
		return s.errorContent(fmt.Sprintf("failed to read file: %v", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s\n", doc.Path))
	sb.WriteString(fmt.Sprintf("Size: %d bytes\n", doc.Size))
	sb.WriteString(fmt.Sprintf("Language: %s\n", doc.Language.String))
	sb.WriteString(fmt.Sprintf("Hash: %s\n", doc.Hash))
	sb.WriteString(fmt.Sprintf("Modified: %s\n\n", doc.ModTime.Format("2006-01-02 15:04:05")))
	sb.WriteString(content)

	return s.textContent(sb.String())
}

func (s *Server) findRepo(ctx context.Context, repoName string) (db.Repository, error) {
	repos, err := s.queries.ListRepos(ctx)
	if err != nil {
		return db.Repository{}, fmt.Errorf("list repos: %v", err)
	}
	for _, r := range repos {
		if r.Name == repoName {
			return r, nil
		}
	}
	return db.Repository{}, fmt.Errorf("repository not found: %s", repoName)
}

func (s *Server) handleListRepos(ctx context.Context, params map[string]any) CallToolResult {
	repos, err := s.queries.ListRepos(ctx)
	if err != nil {
		return s.errorContent(fmt.Sprintf("failed to list repos: %v", err))
	}

	if len(repos) == 0 {
		return s.textContent("No repositories tracked. Add one with `relith repo add <path>`.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tracked repositories (%d):\n\n", len(repos)))
	for _, r := range repos {
		lastIndexed := "never"
		if r.LastIndexedAt.Valid {
			lastIndexed = r.LastIndexedAt.Time.Format("2006-01-02 15:04:05")
		}
		sb.WriteString(fmt.Sprintf("  ID:      %d\n", r.ID))
		sb.WriteString(fmt.Sprintf("  Name:    %s\n", r.Name))
		sb.WriteString(fmt.Sprintf("  Path:    %s\n", r.Path))
		sb.WriteString(fmt.Sprintf("  Status:  %s\n", r.Status))
		sb.WriteString(fmt.Sprintf("  Files:   %d\n", r.FileCount))
		sb.WriteString(fmt.Sprintf("  Indexed: %s\n", lastIndexed))
		sb.WriteString("\n")
	}

	return s.textContent(sb.String())
}

func (s *Server) handleFindSymbol(ctx context.Context, params map[string]any) CallToolResult {
	name := strParam(params, "name")
	if name == "" {
		return s.errorContent("name is required")
	}

	kind := strParam(params, "kind")
	repoName := strParam(params, "repo_name")

	ok, err := s.hasRepos(ctx)
	if err != nil {
		return s.errorContent("check repos: " + err.Error())
	}
	if !ok {
		return s.textContent("No symbols found. " + noReposHelp)
	}

	var rows []db.FindSymbolsByNameRow
	switch {
	case repoName != "" && kind != "":
		data, qErr := s.queries.FindSymbolsByRepoAndKind(ctx, db.FindSymbolsByRepoAndKindParams{
			Name:    repoName,
			Column2: sql.NullString{String: name, Valid: true},
			Kind:    kind,
		})
		if qErr != nil {
			return s.errorContent(fmt.Sprintf("search symbols failed: %v", qErr))
		}
		for _, r := range data {
			rows = append(rows, db.FindSymbolsByNameRow{
				ID: r.ID, DocID: r.DocID, Name: r.Name,
				Kind: r.Kind, Line: r.Line, Col: r.Col,
				Path: r.Path, RepoID: r.RepoID, RepoName: r.RepoName,
			})
		}
	case repoName != "":
		data, qErr := s.queries.FindSymbolsByRepo(ctx, db.FindSymbolsByRepoParams{
			Name:    repoName,
			Column2: sql.NullString{String: name, Valid: true},
		})
		if qErr != nil {
			return s.errorContent(fmt.Sprintf("search symbols failed: %v", qErr))
		}
		for _, r := range data {
			rows = append(rows, db.FindSymbolsByNameRow{
				ID: r.ID, DocID: r.DocID, Name: r.Name,
				Kind: r.Kind, Line: r.Line, Col: r.Col,
				Path: r.Path, RepoID: r.RepoID, RepoName: r.RepoName,
			})
		}
	case kind != "":
		data, qErr := s.queries.FindSymbolsByNameAndKind(ctx, db.FindSymbolsByNameAndKindParams{
			Column1: sql.NullString{String: name, Valid: true},
			Kind:    kind,
		})
		if qErr != nil {
			return s.errorContent(fmt.Sprintf("search symbols failed: %v", qErr))
		}
		for _, r := range data {
			rows = append(rows, db.FindSymbolsByNameRow{
				ID: r.ID, DocID: r.DocID, Name: r.Name,
				Kind: r.Kind, Line: r.Line, Col: r.Col,
				Path: r.Path, RepoID: r.RepoID, RepoName: r.RepoName,
			})
		}
	default:
		var qErr error
		rows, qErr = s.queries.FindSymbolsByName(ctx, sql.NullString{String: name, Valid: true})
		if qErr != nil {
			return s.errorContent(fmt.Sprintf("search symbols failed: %v", qErr))
		}
	}

	if len(rows) == 0 {
		return s.textContent("No symbols found matching: " + name)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d symbol(s) matching %q:\n\n", len(rows), name))
	for i, r := range rows {
		sb.WriteString(fmt.Sprintf("--- %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Name:     %s\n", r.Name))
		sb.WriteString(fmt.Sprintf("Kind:     %s\n", r.Kind))
		sb.WriteString(fmt.Sprintf("File:     %s\n", r.Path))
		sb.WriteString(fmt.Sprintf("Repo:     %s\n", r.RepoName))
		sb.WriteString(fmt.Sprintf("Line:     %d\n", r.Line))
		sb.WriteString(fmt.Sprintf("Column:   %d\n\n", r.Col))
	}

	return s.textContent(sb.String())
}

func (s *Server) handleGetRepoSummary(ctx context.Context, params map[string]any) CallToolResult {
	repoName := strParam(params, "repo_name")
	if repoName == "" {
		return s.errorContent("repo_name is required")
	}

	ok, err := s.hasRepos(ctx)
	if err != nil {
		return s.errorContent("check repos: " + err.Error())
	}
	if !ok {
		return s.errorContent("repository not found: " + repoName + ". " + noReposHelp)
	}

	repos, err := s.queries.ListRepos(ctx)
	if err != nil {
		return s.errorContent(fmt.Sprintf("failed to list repos: %v", err))
	}

	var repo db.Repository
	found := false
	for _, r := range repos {
		if r.Name == repoName {
			repo = r
			found = true
			break
		}
	}
	if !found {
		return s.errorContent(fmt.Sprintf("repository not found: %s", repoName))
	}

	docs, err := s.queries.ListDocuments(ctx, repo.ID)
	if err != nil {
		return s.errorContent(fmt.Sprintf("failed to list documents: %v", err))
	}

	langCount := make(map[string]int)
	for _, d := range docs {
		lang := d.Language.String
		if lang == "" {
			lang = "unknown"
		}
		langCount[lang]++
	}

	chunkCounts, err := s.queries.GetChunkCountsByRepo(ctx, repo.ID)
	if err != nil {
		chunkCounts = nil
	}
	totalChunks := int64(0)
	for _, c := range chunkCounts {
		totalChunks += c.ChunkCount
	}

	lastIndexed := "never"
	if repo.LastIndexedAt.Valid {
		lastIndexed = repo.LastIndexedAt.Time.Format("2006-01-02 15:04:05")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Repository: %s\n", repo.Name))
	sb.WriteString(fmt.Sprintf("  Path:         %s\n", repo.Path))
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", repo.Status))
	sb.WriteString(fmt.Sprintf("  Files:        %d\n", repo.FileCount))
	sb.WriteString(fmt.Sprintf("  Total Chunks: %d\n", totalChunks))
	sb.WriteString(fmt.Sprintf("  Last Indexed: %s\n", lastIndexed))
	sb.WriteString("\nLanguage Breakdown:\n")

	total := len(docs)
	if total == 0 {
		sb.WriteString("  (no files indexed)\n")
	} else {
		for lang, count := range langCount {
			pct := float64(count) / float64(total) * 100
			sb.WriteString(fmt.Sprintf("  %-15s %5d files (%5.1f%%)\n", lang+":", count, pct))
		}
	}

	return s.textContent(sb.String())
}
