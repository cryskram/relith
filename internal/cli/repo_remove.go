package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/indexer"
	"github.com/cryskram/relith/internal/tui"
)

var repoRemoveCmd = &cobra.Command{
	Use:   "remove <id-or-name>",
	Short: "Remove a repository and all its data",
	Long:  `Remove a repository from the index by ID or name. All documents, chunks, symbols, and references for this repository are deleted.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := openDB()
		if err != nil {
			return err
		}
		defer app.close()

		q := db.New(app.db)

		repo, err := lookupRepo(q, args[0])
		if err != nil {
			return err
		}

		if term.IsTerminal(int(os.Stdout.Fd())) {
			return removeRepoTUI(app, repo)
		}
		return removeRepoPlain(app, repo)
	},
}

func lookupRepo(q *db.Queries, arg string) (db.Repository, error) {
	ctx := context.Background()
	if id, parseErr := strconv.ParseInt(arg, 10, 64); parseErr == nil {
		repo, err := q.GetRepo(ctx, id)
		if err == nil {
			return repo, nil
		}
	}
	repos, err := q.ListRepos(ctx)
	if err != nil {
		return db.Repository{}, fmt.Errorf("list repos: %w", err)
	}
	for _, r := range repos {
		if r.Name == arg {
			return r, nil
		}
	}
	return db.Repository{}, fmt.Errorf("repository not found: %s", arg)
}

func removeRepoPlain(app *cliApp, repo db.Repository) error {
	if err := indexer.DeleteRepoWithData(context.Background(), app.db, repo.ID); err != nil {
		return fmt.Errorf("delete repo: %w", err)
	}
	fmt.Printf("Removed repository: id=%d  name=%s  path=%s\n", repo.ID, repo.Name, repo.Path)
	vacuumIfNeeded(app.db)
	return nil
}

func removeRepoTUI(app *cliApp, repo db.Repository) error {
	doneCh := make(chan error, 1)

	go func() {
		doneCh <- indexer.DeleteRepoWithData(context.Background(), app.db, repo.ID)
		close(doneCh)
	}()

	m := tui.NewSpinner(fmt.Sprintf("Removing %s (%s)...", repo.Name, repo.Path), doneCh)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}

	fmt.Printf("Removed repository: id=%d  name=%s  path=%s\n", repo.ID, repo.Name, repo.Path)
	vacuumIfNeeded(app.db)
	return nil
}

func vacuumIfNeeded(database *sql.DB) {
	var pageCount, freelistCount int
	ctx := context.Background()
	database.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
	database.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freelistCount)
	if pageCount == 0 {
		return
	}
	freeRatio := float64(freelistCount) / float64(pageCount)
	if freeRatio > 0.2 {
		fmt.Printf("Free pages: %.0f%% of database. Running VACUUM to reclaim space...\n", freeRatio*100)
		if _, err := database.ExecContext(ctx, "VACUUM"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: VACUUM failed: %v\n", err)
			return
		}
		fmt.Println("VACUUM complete.")
	}
}

func init() {
	repoCmd.AddCommand(repoRemoveCmd)
}
