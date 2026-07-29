package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/tui"
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

		fmt.Printf("\n%s  %s\n",
			tui.TitleStyle.Render("Status"),
			tui.MutedStyle.Render(fmt.Sprintf("(%d repos)", len(repos))))
		fmt.Printf("  %s %s\n",
			tui.MutedStyle.Render("Data:"),
			tui.InfoStyle.Render(app.cfg.Core.DataDir))

		if len(repos) == 0 {
			fmt.Println("  No repositories configured. Use 'relith repo add <path>' to add one.")
			return nil
		}

		fmt.Println()

		var totalFiles, totalChunks int64
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

			var chunkCount int64
			if r.Status == "ready" {
				rows, err := q.GetChunkCountsByRepo(context.Background(), r.ID)
				if err == nil {
					for _, row := range rows {
						chunkCount += row.ChunkCount
					}
				}
			}

			path := r.Path
			if len(path) > 50 {
				path = "..." + path[len(path)-47:]
			}

			fmt.Printf("  %s %s\n", statusIcon, tui.TitleStyle.Render(r.Name))
			fmt.Printf("    %s  %s  %s\n",
				tui.MutedStyle.Render(path),
				tui.InfoStyle.Render(fmt.Sprintf("%d files, %d chunks", r.FileCount, chunkCount)),
				tui.MutedStyle.Render(lastIndexed))

			totalFiles += r.FileCount
			totalChunks += chunkCount
		}

		fmt.Printf("\n  %s %s\n",
			tui.SubtitleStyle.Render("Totals:"),
			tui.InfoStyle.Render(fmt.Sprintf("%d files, %d chunks across %d repositories", totalFiles, totalChunks, len(repos))))

		stats, err := q.GetStats(context.Background())
		if err == nil {
			rawBytes := toInt64(stats.TotalRawBytes)
			chunkBytes := toInt64(stats.TotalChunkBytes)
			if rawBytes > 0 {
				rawMB := float64(rawBytes) / (1024 * 1024)
				chunkMB := float64(chunkBytes) / (1024 * 1024)
				savings := (1 - float64(chunkBytes)/float64(rawBytes)) * 100
				sep := strings.Repeat("─", 40)
				fmt.Println(tui.MutedStyle.Render(sep))
				fmt.Printf("  %s → %s  %s\n",
					tui.InfoStyle.Render(fmt.Sprintf("%d files (%.1f MB)", stats.DocCount, rawMB)),
					tui.InfoStyle.Render(fmt.Sprintf("%d chunks (%.1f MB)", stats.ChunkCount, chunkMB)),
					tui.SuccessStyle.Render(fmt.Sprintf("%.1f%% less", savings)))
			}
		}
		fmt.Println()
		return nil
	},
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
