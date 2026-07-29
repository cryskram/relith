package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/tui"
)

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all indexed repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := openDB()
		if err != nil {
			return err
		}
		defer app.close()

		q := db.New(app.db)

		repos, err := q.ListRepos(context.Background())
		if err != nil {
			return fmt.Errorf("list repos: %w", err)
		}

		if len(repos) == 0 {
			fmt.Println("No repositories indexed. Use 'relith repo add <path>' to add one.")
			return nil
		}

		fmt.Printf("\n%s  %s\n\n",
			tui.TitleStyle.Render("Repositories"),
			tui.MutedStyle.Render(fmt.Sprintf("(%d)", len(repos))))

		for _, r := range repos {
			lastIndexed := "-"
			if r.LastIndexedAt.Valid {
				lastIndexed = r.LastIndexedAt.Time.Format("01-02 15:04")
			}

			statusIcon := tui.MutedStyle.Render("○")
			switch r.Status {
			case "ready":
				statusIcon = tui.SuccessStyle.Render("✓")
			case "indexing":
				statusIcon = tui.HighlightStyle.Render("◐")
			case "pending":
				statusIcon = tui.MutedStyle.Render("○")
			}

			path := r.Path
			if len(path) > 50 {
				path = "..." + path[len(path)-47:]
			}

			fmt.Printf("  %s %s\n", statusIcon, tui.TitleStyle.Render(r.Name))
			fmt.Printf("    %s  %s %s  %s\n",
				tui.MutedStyle.Render(path),
				tui.InfoStyle.Render(fmt.Sprintf("%d files", r.FileCount)),
				tui.MutedStyle.Render("·"),
				tui.MutedStyle.Render(lastIndexed))
		}
		fmt.Println()
		return nil
	},
}

func init() {
	repoCmd.AddCommand(repoListCmd)
}
