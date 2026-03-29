package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the knet version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("knet version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
