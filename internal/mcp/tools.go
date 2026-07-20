package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/indexer"
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
