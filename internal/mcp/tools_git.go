package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/cryskram/relith/internal/git"
)

func (s *Server) resolveGitRepo(ctx context.Context, params map[string]any) (string, error) {
	repoName := strParam(params, "repo_name")
	if repoName == "" {
		return "", fmt.Errorf("repo_name is required")
	}

	ok, err := s.hasRepos(ctx)
	if err != nil {
		return "", fmt.Errorf("check repos: %v", err)
	}
	if !ok {
		return "", fmt.Errorf("repository not found: %s. %s", repoName, noReposHelp)
	}

	repo, err := s.findRepo(ctx, repoName)
	if err != nil {
		return "", err
	}

	if !git.IsRepo(repo.Path) {
		return "", fmt.Errorf("%s is not a git repository: %s", repoName, repo.Path)
	}

	return repo.Path, nil
}

func (s *Server) handleGetRecentCommits(ctx context.Context, params map[string]any) CallToolResult {
	workdir, err := s.resolveGitRepo(ctx, params)
	if err != nil {
		return s.errorContent(err.Error())
	}

	max := intParam(params, "max", 20)
	out, err := git.RecentCommits(ctx, workdir, max)
	if err != nil {
		return s.errorContent(fmt.Sprintf("git log failed: %v", err))
	}

	commits := parseCommits(out)
	if len(commits) == 0 {
		return s.textContent("No commits found.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d commit(s):\n\n", len(commits)))
	for _, c := range commits {
		sb.WriteString(fmt.Sprintf("%s  %s  %s <%s>\n    %s\n\n", c.ShortHash, c.Date, c.Author, c.Email, c.Subject))
	}

	return s.textContent(sb.String())
}

func (s *Server) handleGetFileHistory(ctx context.Context, params map[string]any) CallToolResult {
	workdir, err := s.resolveGitRepo(ctx, params)
	if err != nil {
		return s.errorContent(err.Error())
	}

	file := strParam(params, "path")
	if file == "" {
		return s.errorContent("path is required")
	}

	max := intParam(params, "max", 20)
	out, err := git.FileHistory(ctx, workdir, file, max)
	if err != nil {
		return s.errorContent(fmt.Sprintf("git log for %s failed: %v", file, err))
	}

	commits := parseCommits(out)
	if len(commits) == 0 {
		return s.textContent("No commit history found for: " + file)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Commit history for %s (%d):\n\n", file, len(commits)))
	for _, c := range commits {
		sb.WriteString(fmt.Sprintf("%s  %s  %s <%s>\n    %s\n\n", c.ShortHash, c.Date, c.Author, c.Email, c.Subject))
	}

	return s.textContent(sb.String())
}

func (s *Server) handleGetBlame(ctx context.Context, params map[string]any) CallToolResult {
	workdir, err := s.resolveGitRepo(ctx, params)
	if err != nil {
		return s.errorContent(err.Error())
	}

	file := strParam(params, "path")
	if file == "" {
		return s.errorContent("path is required")
	}

	start := intParam(params, "start_line", 0)
	end := intParam(params, "end_line", 0)
	out, err := git.Blame(ctx, workdir, file, start, end)
	if err != nil {
		return s.errorContent(fmt.Sprintf("git blame for %s failed: %v", file, err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Blame for %s:\n\n", file))
	sb.WriteString(out)

	return s.textContent(sb.String())
}

func (s *Server) handleGetDiff(ctx context.Context, params map[string]any) CallToolResult {
	workdir, err := s.resolveGitRepo(ctx, params)
	if err != nil {
		return s.errorContent(err.Error())
	}

	base := strParam(params, "base")
	head := strParam(params, "head")
	if base == "" {
		base = "HEAD~1"
	}
	if head == "" {
		head = "HEAD"
	}

	maxStat := intParam(params, "max_stat", 120)
	out, err := git.Diff(ctx, workdir, base, head, maxStat)
	if err != nil {
		return s.errorContent(fmt.Sprintf("git diff %s %s failed: %v", base, head, err))
	}

	if strings.TrimSpace(out) == "" {
		return s.textContent(fmt.Sprintf("No differences between %s and %s.", base, head))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Diff %s...%s:\n\n", base, head))
	sb.WriteString(out)

	return s.textContent(sb.String())
}

type commit struct {
	ShortHash string
	Date      string
	Author    string
	Email     string
	Subject   string
}

func parseCommits(out string) []commit {
	var commits []commit
	for _, record := range strings.Split(out, "\x1e") {
		fields := strings.Split(record, "\x1f")
		if len(fields) < 5 {
			continue
		}
		hash := fields[0]
		if len(hash) < 7 {
			continue
		}
		commits = append(commits, commit{
			ShortHash: hash[:7],
			Date:      fields[1],
			Author:    fields[2],
			Email:     fields[3],
			Subject:   fields[4],
		})
	}
	return commits
}
