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

func (s *Server) handleGetSymbolDefinition(ctx context.Context, params map[string]any) CallToolResult {
	name := strParam(params, "name")
	if name == "" {
		return s.errorContent("name is required")
	}
	repoName := strParam(params, "repo_name")
	kind := strParam(params, "kind")
	maxResults := intParam(params, "max_results", 10)

	rows, err := s.queries.FindSymbolsByName(ctx, sql.NullString{String: name, Valid: true})
	if err != nil {
		return s.errorContent(fmt.Sprintf("find symbols failed: %v", err))
	}

	reposByID := map[int64]db.Repository{}
	if repoName != "" {
		repo, err := s.findRepo(ctx, repoName)
		if err != nil {
			return s.errorContent(err.Error())
		}
		reposByID[repo.ID] = repo
	}

	type def struct {
		Name    string
		Kind    string
		Repo    string
		Path    string
		Line    int64
		Col     int64
		Snippet string
	}
	var defs []def
	for _, row := range rows {
		if row.Name != name {
			continue
		}
		if kind != "" && !strings.EqualFold(row.Kind, kind) {
			continue
		}
		if repoName != "" && row.RepoName != repoName {
			continue
		}
		repo, ok := reposByID[row.RepoID]
		if !ok {
			repo, err = s.findRepo(ctx, row.RepoName)
			if err != nil {
				continue
			}
			reposByID[row.RepoID] = repo
		}
		content, err := indexer.ReadFileContent(filepath.Join(repo.Path, row.Path), 10*1024*1024)
		if err != nil {
			continue
		}
		defs = append(defs, def{
			Name:    row.Name,
			Kind:    row.Kind,
			Repo:    row.RepoName,
			Path:    row.Path,
			Line:    row.Line,
			Col:     row.Col,
			Snippet: lineWindow(content, int(row.Line), 3),
		})
		if len(defs) >= maxResults {
			break
		}
	}

	if len(defs) == 0 {
		return s.textContent("No symbol definitions found matching: " + name)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Symbol definition(s) for %q:\n\n", name))
	for i, d := range defs {
		sb.WriteString(fmt.Sprintf("--- %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Repo:   %s\n", d.Repo))
		sb.WriteString(fmt.Sprintf("File:   %s\n", d.Path))
		sb.WriteString(fmt.Sprintf("Kind:   %s\n", d.Kind))
		sb.WriteString(fmt.Sprintf("Line:   %d:%d\n", d.Line, d.Col))
		sb.WriteString(fmt.Sprintf("Snippet:\n%s\n\n", d.Snippet))
	}
	return s.textContent(sb.String())
}

func (s *Server) handleFindCallees(ctx context.Context, params map[string]any) CallToolResult {
	name := strParam(params, "name")
	if name == "" {
		return s.errorContent("name is required")
	}
	repoName := strParam(params, "repo_name")
	maxResults := intParam(params, "max_results", 15)

	rows, err := s.queries.FindSymbolsByName(ctx, sql.NullString{String: name, Valid: true})
	if err != nil {
		return s.errorContent(fmt.Sprintf("find symbols failed: %v", err))
	}

	type callee struct {
		Name  string
		Count int64
	}
	var out strings.Builder
	out.WriteString(fmt.Sprintf("Callees for %q:\n\n", name))
	printedAny := false
	for _, sym := range rows {
		if sym.Name != name {
			continue
		}
		if repoName != "" && sym.RepoName != repoName {
			continue
		}

		query := `SELECT name, COUNT(*) AS c FROM refs WHERE doc_id = ? GROUP BY name ORDER BY c DESC, name LIMIT ?`
		refRows, err := s.db.QueryContext(ctx, query, sym.DocID, maxResults)
		if err != nil {
			continue
		}

		var callees []callee
		for refRows.Next() {
			var c callee
			if err := refRows.Scan(&c.Name, &c.Count); err != nil {
				continue
			}
			if c.Name == name {
				continue
			}
			callees = append(callees, c)
		}
		refRows.Close()
		if len(callees) == 0 {
			continue
		}

		printedAny = true
		out.WriteString(fmt.Sprintf("Definition: %s/%s (%s) %d:%d\n", sym.RepoName, sym.Path, sym.Kind, sym.Line, sym.Col))
		for i, c := range callees {
			out.WriteString(fmt.Sprintf("  %d. %s (%d)\n", i+1, c.Name, c.Count))
		}
		out.WriteString("\n")
	}

	if !printedAny {
		return s.textContent("No callees found for: " + name)
	}
	return s.textContent(out.String())
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

func lineWindow(content string, line, radius int) string {
	if line < 1 {
		line = 1
	}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return ""
	}
	start := line - radius - 1
	if start < 0 {
		start = 0
	}
	end := line + radius
	if end > len(lines) {
		end = len(lines)
	}
	var sb strings.Builder
	for i := start; i < end; i++ {
		prefix := " "
		if i == line-1 {
			prefix = ">"
		}
		sb.WriteString(fmt.Sprintf("%s %4d | %s\n", prefix, i+1, lines[i]))
	}
	return sb.String()
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

func (s *Server) handleQueryGraph(ctx context.Context, params map[string]any) CallToolResult {
	mode := strParam(params, "mode")
	repoName := strParam(params, "repo_name")
	path := strParam(params, "path")
	targetPath := strParam(params, "target_path")
	maxResults := intParam(params, "max_results", 20)

	if mode == "" {
		return s.errorContent("mode is required (neighbors, hotspots, path)")
	}
	if repoName == "" {
		return s.errorContent("repo_name is required")
	}

	repo, err := s.findRepo(ctx, repoName)
	if err != nil {
		return s.errorContent(err.Error())
	}

	switch mode {
	case "neighbors":
		return s.queryGraphNeighbors(ctx, repo, path, maxResults)
	case "hotspots":
		return s.queryGraphHotspots(ctx, repo, maxResults)
	case "path":
		return s.queryGraphPath(ctx, repo, path, targetPath, maxResults)
	default:
		return s.errorContent(fmt.Sprintf("unknown mode: %s (use neighbors, hotspots, or path)", mode))
	}
}

func (s *Server) queryGraphNeighbors(ctx context.Context, repo db.Repository, path string, maxResults int) CallToolResult {
	if path == "" {
		return s.errorContent("path is required for neighbors mode")
	}

	var docID int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM documents WHERE repo_id = ? AND path = ?`, repo.ID, path).Scan(&docID)
	if err != nil {
		return s.errorContent(fmt.Sprintf("file not found: %s", path))
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT target_doc_id, kind, weight FROM graph_edges WHERE source_doc_id = ? AND repo_id = ?
		UNION ALL
		SELECT source_doc_id, kind, weight FROM graph_edges WHERE target_doc_id = ? AND repo_id = ?
		ORDER BY weight DESC LIMIT ?`, docID, repo.ID, docID, repo.ID, maxResults)
	if err != nil {
		return s.errorContent(fmt.Sprintf("query graph: %v", err))
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Neighbors of %s:\n\n", path))
	count := 0
	for rows.Next() {
		var neighborID int64
		var kind string
		var weight int64
		if err := rows.Scan(&neighborID, &kind, &weight); err != nil {
			continue
		}
		var p string
		s.db.QueryRowContext(ctx, `SELECT path FROM documents WHERE id = ?`, neighborID).Scan(&p)
		sb.WriteString(fmt.Sprintf("  %s (weight=%d, kind=%s)\n", p, weight, kind))
		count++
	}
	if count == 0 {
		sb.WriteString("  (no connections)\n")
	}
	return s.textContent(sb.String())
}

func (s *Server) queryGraphHotspots(ctx context.Context, repo db.Repository, maxResults int) CallToolResult {
	rows, err := s.db.QueryContext(ctx, `
		SELECT doc_id, COUNT(*) AS cnt FROM (
			SELECT source_doc_id AS doc_id FROM graph_edges WHERE repo_id = ?
			UNION ALL
			SELECT target_doc_id AS doc_id FROM graph_edges WHERE repo_id = ?
		) GROUP BY doc_id ORDER BY cnt DESC LIMIT ?`, repo.ID, repo.ID, maxResults)
	if err != nil {
		return s.errorContent(fmt.Sprintf("query hotspots: %v", err))
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Hotspots (most connected files) for %s:\n\n", repo.Name))
	count := 0
	for rows.Next() {
		var docID, cnt int64
		if err := rows.Scan(&docID, &cnt); err != nil {
			continue
		}
		var p string
		s.db.QueryRowContext(ctx, `SELECT path FROM documents WHERE id = ?`, docID).Scan(&p)
		sb.WriteString(fmt.Sprintf("  %4d  %s\n", cnt, p))
		count++
	}
	if count == 0 {
		sb.WriteString("  (no graph edges yet — index the repo first)\n")
	}
	return s.textContent(sb.String())
}

func (s *Server) queryGraphPath(ctx context.Context, repo db.Repository, fromPath, toPath string, maxResults int) CallToolResult {
	if fromPath == "" || toPath == "" {
		return s.errorContent("both path and target_path are required for path mode")
	}

	var fromID, toID int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM documents WHERE repo_id = ? AND path = ?`, repo.ID, fromPath).Scan(&fromID); err != nil {
		return s.errorContent(fmt.Sprintf("file not found: %s", fromPath))
	}
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM documents WHERE repo_id = ? AND path = ?`, repo.ID, toPath).Scan(&toID); err != nil {
		return s.errorContent(fmt.Sprintf("file not found: %s", toPath))
	}

	visited := map[int64]bool{}
	var path []string
	var found bool
	var bfs func(id int64, depth int) bool
	bfs = func(id int64, depth int) bool {
		if id == toID || found {
			return true
		}
		if depth > maxResults || visited[id] {
			return false
		}
		visited[id] = true

		rows, err := s.db.QueryContext(ctx, `SELECT target_doc_id FROM graph_edges WHERE source_doc_id = ? AND repo_id = ? LIMIT 10`, id, repo.ID)
		if err != nil {
			return false
		}
		defer rows.Close()

		for rows.Next() {
			var targetID int64
			if err := rows.Scan(&targetID); err != nil {
				continue
			}
			if visited[targetID] {
				continue
			}
			if bfs(targetID, depth+1) {
				var p string
				s.db.QueryRowContext(ctx, `SELECT path FROM documents WHERE id = ?`, targetID).Scan(&p)
				path = append([]string{p}, path...)
				return true
			}
		}
		return false
	}

	if bfs(fromID, 0) {
		var fromPathName string
		s.db.QueryRowContext(ctx, `SELECT path FROM documents WHERE id = ?`, fromID).Scan(&fromPathName)
		path = append([]string{fromPathName}, path...)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Dependency path from %s to %s:\n\n", fromPath, toPath))
	if len(path) == 0 {
		sb.WriteString("  (no path found)\n")
	} else {
		for i, p := range path {
			arrow := "  "
			if i < len(path)-1 {
				arrow = "→ "
			}
			sb.WriteString(fmt.Sprintf("  %s %s\n", arrow, p))
		}
	}
	return s.textContent(sb.String())
}

func (s *Server) handleGetArchitecture(ctx context.Context, params map[string]any) CallToolResult {
	repoName := strParam(params, "repo_name")
	maxResults := intParam(params, "max_results", 10)

	if repoName == "" {
		return s.errorContent("repo_name is required")
	}

	repo, err := s.findRepo(ctx, repoName)
	if err != nil {
		return s.errorContent(err.Error())
	}

	docs, err := s.queries.ListDocuments(ctx, repo.ID)
	if err != nil {
		return s.errorContent(fmt.Sprintf("list docs: %v", err))
	}
	if len(docs) == 0 {
		return s.textContent("No files indexed. Run `relith index` first.")
	}

	docMap := make(map[int64]string, len(docs))
	langCount := make(map[string]int)
	pkgCount := make(map[string]int)
	for _, d := range docs {
		docMap[d.ID] = d.Path
		lang := d.Language.String
		if lang == "" {
			lang = "unknown"
		}
		langCount[lang]++

		dir := filepath.Dir(d.Path)
		if dir == "." {
			dir = "/"
		}
		pkgCount[dir]++
	}

	type pkgInfo struct {
		name  string
		count int
	}
	var pkgs []pkgInfo
	for name, count := range pkgCount {
		pkgs = append(pkgs, pkgInfo{name, count})
	}
	sort.Slice(pkgs, func(i, j int) bool {
		if pkgs[i].count == pkgs[j].count {
			return pkgs[i].name < pkgs[j].name
		}
		return pkgs[i].count > pkgs[j].count
	})
	if len(pkgs) > maxResults {
		pkgs = pkgs[:maxResults]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Architecture overview for %s:\n\n", repoName))
	sb.WriteString(fmt.Sprintf("Total files: %d\n\n", len(docs)))

	sb.WriteString("Languages:\n")
	type langInfo struct {
		name  string
		count int
	}
	var langs []langInfo
	for name, count := range langCount {
		langs = append(langs, langInfo{name, count})
	}
	sort.Slice(langs, func(i, j int) bool { return langs[i].count > langs[j].count })
	for _, l := range langs {
		pct := float64(l.count) / float64(len(docs)) * 100
		sb.WriteString(fmt.Sprintf("  %-15s %5d files (%5.1f%%)\n", l.name+":", l.count, pct))
	}

	sb.WriteString("\nTop packages/directories:\n")
	for _, p := range pkgs {
		sb.WriteString(fmt.Sprintf("  %-30s %5d files\n", p.name, p.count))
	}

	hotRows, err := s.db.QueryContext(ctx, `
		SELECT doc_id, COUNT(*) AS cnt FROM (
			SELECT source_doc_id AS doc_id FROM graph_edges WHERE repo_id = ?
			UNION ALL
			SELECT target_doc_id AS doc_id FROM graph_edges WHERE repo_id = ?
		) GROUP BY doc_id ORDER BY cnt DESC LIMIT ?`, repo.ID, repo.ID, maxResults)
	if err == nil {
		defer hotRows.Close()
		var hasHot bool
		var hotSb strings.Builder
		hotSb.WriteString("\nHotspots (most connected files):\n")
		for hotRows.Next() {
			var docID, cnt int64
			if err := hotRows.Scan(&docID, &cnt); err != nil {
				continue
			}
			p := docMap[docID]
			if p == "" {
				continue
			}
			hasHot = true
			hotSb.WriteString(fmt.Sprintf("  %4d connections  %s\n", cnt, p))
		}
		if hasHot {
			sb.WriteString(hotSb.String())
		}
		hotRows.Close()
	}

	entryRows, err := s.db.QueryContext(ctx, `
		SELECT source_doc_id, COUNT(*) AS cnt FROM graph_edges WHERE repo_id = ? AND kind = 'imports' GROUP BY source_doc_id ORDER BY cnt DESC LIMIT ?`, repo.ID, maxResults)
	if err == nil {
		defer entryRows.Close()
		var hasEntry bool
		var entrySb strings.Builder
		entrySb.WriteString("\nEntry points (most outbound imports):\n")
		for entryRows.Next() {
			var docID, cnt int64
			if err := entryRows.Scan(&docID, &cnt); err != nil {
				continue
			}
			p := docMap[docID]
			if p == "" {
				continue
			}
			hasEntry = true
			entrySb.WriteString(fmt.Sprintf("  %4d imports  %s\n", cnt, p))
		}
		if hasEntry {
			sb.WriteString(entrySb.String())
		}
		entryRows.Close()
	}

	return s.textContent(sb.String())
}

func (s *Server) handleTraceDependency(ctx context.Context, params map[string]any) CallToolResult {
	repoName := strParam(params, "repo_name")
	path := strParam(params, "path")
	direction := strParam(params, "direction")
	depth := intParam(params, "depth", 1)
	maxResults := intParam(params, "max_results", 20)

	if repoName == "" || path == "" {
		return s.errorContent("repo_name and path are required")
	}
	if direction == "" {
		direction = "both"
	}

	repo, err := s.findRepo(ctx, repoName)
	if err != nil {
		return s.errorContent(err.Error())
	}

	type dep struct {
		path   string
		kind   string
		weight int64
		level  int
	}

	var allDeps []dep
	seen := map[string]bool{}
	var walk func(currentPath string, currentDir string, remaining int)
	walk = func(currentPath string, currentDir string, remaining int) {
		if remaining < 0 || seen[currentPath] {
			return
		}
		seen[currentPath] = true

		var docID int64
		if err := s.db.QueryRowContext(ctx, `SELECT id FROM documents WHERE repo_id = ? AND path = ?`, repo.ID, currentPath).Scan(&docID); err != nil {
			return
		}

		if direction == "outbound" || direction == "both" {
			rows, err := s.db.QueryContext(ctx, `SELECT target_doc_id, kind, weight FROM graph_edges WHERE source_doc_id = ? AND repo_id = ? LIMIT ?`, docID, repo.ID, maxResults)
			if err == nil {
				for rows.Next() {
					var tgtID int64
					var k string
					var w int64
					if err := rows.Scan(&tgtID, &k, &w); err != nil {
						continue
					}
					var p string
					s.db.QueryRowContext(ctx, `SELECT path FROM documents WHERE id = ?`, tgtID).Scan(&p)
					if p == "" || seen[p] {
						continue
					}
					allDeps = append(allDeps, dep{p, "out:" + k, w, depth - remaining})
					if remaining > 0 {
						walk(p, filepath.Dir(p), remaining-1)
					}
				}
				rows.Close()
			}
		}

		if direction == "inbound" || direction == "both" {
			rows, err := s.db.QueryContext(ctx, `SELECT source_doc_id, kind, weight FROM graph_edges WHERE target_doc_id = ? AND repo_id = ? LIMIT ?`, docID, repo.ID, maxResults)
			if err == nil {
				for rows.Next() {
					var srcID int64
					var k string
					var w int64
					if err := rows.Scan(&srcID, &k, &w); err != nil {
						continue
					}
					var p string
					s.db.QueryRowContext(ctx, `SELECT path FROM documents WHERE id = ?`, srcID).Scan(&p)
					if p == "" || seen[p] {
						continue
					}
					allDeps = append(allDeps, dep{p, "in:" + k, w, depth - remaining})
					if remaining > 0 {
						walk(p, filepath.Dir(p), remaining-1)
					}
				}
				rows.Close()
			}
		}
	}

	walk(path, filepath.Dir(path), depth)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Dependencies for %s (direction=%s, depth=%d):\n\n", path, direction, depth))
	if len(allDeps) == 0 {
		sb.WriteString("  (no dependencies found)\n")
	} else {
		for _, d := range allDeps {
			indent := strings.Repeat("  ", d.level)
			sb.WriteString(fmt.Sprintf("  %s%s [%s] (weight=%d)\n", indent, d.path, d.kind, d.weight))
		}
	}
	return s.textContent(sb.String())
}
