package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/cryskram/relith/internal/db"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show indexing status",
	Long:  `Show the current status of all repositories and the daemon.`,
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

		fmt.Printf("Data directory: %s\n", app.cfg.Core.DataDir)
		fmt.Printf("Repositories:   %d\n\n", len(repos))

		if len(repos) == 0 {
			fmt.Println("No repositories configured. Use 'relith repo add <path>' to add one.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tName\tStatus\tFiles\tChunks\tLast Indexed")
		fmt.Fprintln(w, "--\t----\t------\t-----\t------\t------------")

		var totalFiles, totalChunks int64
		for _, r := range repos {
			lastIndexed := "-"
			if r.LastIndexedAt.Valid {
				lastIndexed = r.LastIndexedAt.Time.Format("2006-01-02 15:04")
			}

			var chunkCount int64
			if r.Status == "ready" {
				rows, err := q.GetChunkCountsByRepo(context.Background(), r.ID)
				if err == nil {
					for _, row := range rows {
						chunkCount += row.ChunkCount
					}
				}
			}

			fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%d\t%s\n", r.ID, r.Name, r.Status, r.FileCount, chunkCount, lastIndexed)
			totalFiles += r.FileCount
			totalChunks += chunkCount
		}
		w.Flush()

		fmt.Printf("\nTotals: %d files, %d chunks across %d repositories\n", totalFiles, totalChunks, len(repos))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
