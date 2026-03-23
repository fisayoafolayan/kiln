// Package cmd defines the kiln CLI commands.
package cmd

import (
	"github.com/spf13/cobra"
)

var cfgFile string

// Root returns the root cobra command with all subcommands registered.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "kiln",
		Short: "Generate a production-ready Go API from your database schema",
		Long: `kiln reads your database schema via bob and generates a complete,
idiomatic Go API - types, store, handlers, router, and OpenAPI spec.

Supports Postgres, MySQL/MariaDB, and SQLite.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	root.PersistentFlags().StringVarP(
		&cfgFile, "config", "c", "kiln.yaml",
		"path to config file",
	)

	// Register subcommands
	root.AddCommand(
		initCmd(),
		generateCmd(),
		diffCmd(),
		introspectCmd(),
		versionCmd(),
	)

	return root
}
