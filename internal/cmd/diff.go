package cmd

import (
	"os"

	"github.com/fisayoafolayan/kiln/internal/generator"
	"github.com/spf13/cobra"
)

func diffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show what would be generated without writing any files",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, schema, err := parseSchema()
			if err != nil {
				return err
			}
			g := generator.New(cfg, schema)
			return g.Diff(os.Stdout)
		},
	}
}
