package cmd

import (
	"fmt"
	"os"

	"github.com/fisayoafolayan/kiln/internal/bobadapter"
	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator"
	"github.com/fisayoafolayan/kiln/internal/ir"
	bob "github.com/fisayoafolayan/kiln/internal/parser/bob"
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
			schema, err := parseBobModels(cfg)
			if err != nil {
				return err
			}
			g := generator.New(cfg, schema)
			return g.Diff(os.Stdout)
		},
	}
}

// parseBobModels reads existing bob-generated files via AST parser.
// Used by diff and --no-bob, where no database connection is needed.
func parseBobModels(cfg *config.Config) (*ir.Schema, error) {
	p := bob.New(cfg.Bob.ModelsDir, bobadapter.DriverFromString(cfg.Database.Driver))
	p.Exclude = cfg.Tables.Exclude
	schema, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}
	return schema, nil
}
