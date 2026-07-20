package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cryskram/relith/internal/db"
)

var repoAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a repository to the index",
	Long:  `Add a repository path to the database for indexing. If the path is a git repository, the remote URL is detected automatically.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]

		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("stat path: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", absPath)
		}

		app, err := openDB()
		if err != nil {
			return err
		}
		defer app.close()

		q := db.New(app.db)

		existing, err := q.GetRepoByPath(context.Background(), absPath)
		if err == nil {
			return fmt.Errorf("repository already exists (id=%d, name=%s)", existing.ID, existing.Name)
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check existing: %w", err)
		}

		name := filepath.Base(absPath)

		var remoteURL sql.NullString
		if remote := detectGitRemote(absPath); remote != "" {
			remoteURL = sql.NullString{String: remote, Valid: true}
		}

		repo, err := q.CreateRepo(context.Background(), db.CreateRepoParams{
			Path:      absPath,
			Name:      name,
			RemoteUrl: remoteURL,
		})
		if err != nil {
			return fmt.Errorf("create repo: %w", err)
		}

		fmt.Printf("Added repository: id=%d  name=%s  path=%s\n", repo.ID, repo.Name, repo.Path)
		return nil
	},
}

func init() {
	repoCmd.AddCommand(repoAddCmd)
}

func detectGitRemote(path string) string {
	data, err := os.ReadFile(filepath.Join(path, ".git", "config"))
	if err != nil {
		return ""
	}
	return extractRemoteURL(string(data))
}

func extractRemoteURL(configContent string) string {
	var inOrigin bool
	for _, line := range strings.Split(configContent, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[remote") {
			inOrigin = strings.Contains(trimmed, `"origin"`)
			continue
		}

		if inOrigin && strings.HasPrefix(trimmed, "url = ") {
			url := strings.TrimSpace(trimmed[5:])
			url = strings.Trim(url, `"`)
			return url
		}
	}
	return ""
}
