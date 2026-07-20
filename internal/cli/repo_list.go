package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/cryskram/relith/internal/db"
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

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tName\tPath\tStatus\tFiles\tLast Indexed")
		fmt.Fprintln(w, "--\t----\t----\t------\t-----\t------------")
		for _, r := range repos {
			lastIndexed := "-"
			if r.LastIndexedAt.Valid {
				lastIndexed = r.LastIndexedAt.Time.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s\n", r.ID, r.Name, r.Path, r.Status, r.FileCount, lastIndexed)
		}
		w.Flush()
		return nil
	},
}

func init() {
	repoCmd.AddCommand(repoListCmd)
}
