package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print kiln version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kiln %s\n", Version)
		},
	}
}
