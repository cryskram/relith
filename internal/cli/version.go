package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.2.1"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the current version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}
