package cmd

import (
	"fmt"
	"os"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator"
	"github.com/fisayoafolayan/kiln/internal/parser/bob"
	"github.com/spf13/cobra"
)

func diffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show what would be generated without writing any files",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			p := bob.New(cfg.Bob.ModelsDir, toIRDriver(cfg.Database.Driver))
			p.Exclude = cfg.Tables.Exclude
			schema, err := p.Parse()
			if err != nil {
				return fmt.Errorf("parsing schema: %w", err)
			}
			g := generator.New(cfg, schema)
			return g.Diff(os.Stdout)
		},
	}
}
