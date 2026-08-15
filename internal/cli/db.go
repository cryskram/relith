package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
}

var dbVacuumCmd = &cobra.Command{
	Use:   "vacuum",
	Short: "Reclaim unused database space",
	Long: `Run VACUUM to rebuild the database file and reclaim space from deleted rows and free pages.

This is useful after removing repositories or re-indexing, which can leave behind
dead space in the SQLite file. VACUUM rewrites the entire database file, so you
need free disk space equal to the current database size.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}

		dbPath := filepath.Join(cfg.Core.DataDir, "relith.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found at %s", dbPath)
		}

		fmt.Printf("Opening database: %s\n", dbPath)
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		// Check current free pages
		var pageCount, freelistCount int
		ctx := context.Background()
		_ = database.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
		_ = database.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freelistCount)
		pageSize := 4096
		_ = database.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize)
		freeMB := float64(freelistCount*pageSize) / (1024 * 1024)
		usedMB := float64((pageCount-freelistCount)*pageSize) / (1024 * 1024)

		if freelistCount == 0 {
			fmt.Println("Database has no free pages; nothing to reclaim.")
			return nil
		}

		fmt.Printf("Pages: %d total, %d free (%.1f MB free of %.1f MB)\n", pageCount, freelistCount, freeMB, float64(pageCount*pageSize)/(1024*1024))
		fmt.Printf("Actual data: %.1f MB\n", usedMB)
		fmt.Println("Running VACUUM...")

		if _, err := database.ExecContext(ctx, "VACUUM"); err != nil {
			return fmt.Errorf("vacuum: %w", err)
		}

		// Check free pages after vacuum
		_ = database.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freelistCount)
		_ = database.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
		fmt.Printf("Done. Database now uses %.1f MB (%.1f MB reclaimed)\n",
			float64(pageCount*pageSize)/(1024*1024), freeMB)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbVacuumCmd)
}
