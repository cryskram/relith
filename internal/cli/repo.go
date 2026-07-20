package cli

import (
	"github.com/spf13/cobra"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage indexed repositories",
}

func init() {
	rootCmd.AddCommand(repoCmd)
}
