package cli

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/indexer"
	"github.com/cryskram/relith/internal/tui"
)

var indexCmd = &cobra.Command{
	Use:   "index [repo-path]",
	Short: "Index a repository",
	Long: `Index a repository by its path. If no path is given, all pending repositories are indexed.

Examples:
  relith index /path/to/repo    Index a specific repository by path
  relith index                  Index all pending repositories`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := openDB()
		if err != nil {
			return err
		}
		defer app.close()

		q := db.New(app.db)

		if len(args) == 1 {
			repoPath := args[0]
			repo, err := q.GetRepoByPath(context.Background(), repoPath)
			if err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("repository not found: %s", repoPath)
				}
				return fmt.Errorf("get repo: %w", err)
			}
			return indexRepo(app, q, repo)
		}

		repos, err := q.ListRepos(context.Background())
		if err != nil {
			return fmt.Errorf("list repos: %w", err)
		}

		var indexed int
		for _, repo := range repos {
			if err := indexRepo(app, q, repo); err != nil {
				fmt.Fprintf(os.Stderr, "Index failed for repo %d: %v\n", repo.ID, err)
				continue
			}
			indexed++
		}

		if indexed == 0 {
			fmt.Println("No repositories to index. Use 'relith repo add <path>' first.")
		} else {
			fmt.Printf("Indexed %d repository(ies)\n", indexed)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}

func indexRepo(app *cliApp, q *db.Queries, repo db.Repository) error {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return indexRepoTUI(app, repo)
	}

	idx := indexer.New(app.db, app.logger, app.cfg.Indexer)
	fmt.Printf("Indexing: %s (%s)\n", repo.Name, repo.Path)
	result, err := idx.IndexRepo(context.Background(), repo.Path, repo.ID)
	if err != nil {
		return fmt.Errorf("index repo %s: %w", repo.Name, err)
	}
	fmt.Printf("  Indexed: %d files, %d chunks, %d skipped, %d errors [%s]\n",
		result.FilesIndexed, result.TotalChunks, result.FilesSkipped, result.FilesError, result.Elapsed)
	return nil
}

func indexRepoTUI(app *cliApp, repo db.Repository) error {
	silentLog := slog.New(slog.DiscardHandler)
	idx := indexer.New(app.db, silentLog, app.cfg.Indexer)

	eventCh := make(chan indexer.ProgressEvent, 100)

	go func() {
		defer close(eventCh)
		ctx := context.Background()
		idx.OnProgress(func(evt indexer.ProgressEvent) {
			eventCh <- evt
		})
		_, err := idx.IndexRepo(ctx, repo.Path, repo.ID)
		if err != nil {
			eventCh <- indexer.ProgressEvent{
				Phase: "complete",
				Error: err,
			}
		}
	}()

	p := tea.NewProgram(tui.NewProgress(eventCh, repo.Name))
	_, err := p.Run()
	return err
}
