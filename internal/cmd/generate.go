package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/fisayoafolayan/kiln/internal/bobadapter"
	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator"
	"github.com/spf13/cobra"
	"github.com/stephenafamo/bob/gen"
	"github.com/stephenafamo/bob/gen/drivers"
	"github.com/stephenafamo/bob/gen/plugins"

	mysqldriver "github.com/stephenafamo/bob/gen/bobgen-mysql/driver"
	psqldriver "github.com/stephenafamo/bob/gen/bobgen-psql/driver"
	sqlitedriver "github.com/stephenafamo/bob/gen/bobgen-sqlite/driver"
)

func generateCmd() *cobra.Command {
	var (
		table string
		noBob bool
		force bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Go code from your database schema",
		Long: `Generates your full API layer from the database schema.

kiln reads your database schema then generates models, store,
handlers, router, and OpenAPI spec.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			if cfg.Bob.IsEnabled() && !noBob {
				// Normal path: bob reads schema and generates models,
				// kiln plugin generates the API layer - all in one pass.
				fmt.Println("  Reading database schema...")
				if err := runBobWithKiln(cfg, table, force); err != nil {
					return fmt.Errorf("failed: %w\n\n"+
						"  Make sure your database is running and the DSN in kiln.yaml is correct.", err)
				}
			} else {
				// --no-bob: parse existing bob models, run kiln generators only.
				fmt.Println("  Parsing existing models...")
				schema, err := parseBobModels(cfg)
				if err != nil {
					return err
				}
				if table != "" {
					schema, err = schema.FilterTable(table)
					if err != nil {
						return err
					}
				}
				g := generator.New(cfg, schema)
				g.SetForce(force)
				return g.Run(os.Stdout)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&table, "table", "", "only regenerate a specific table")
	cmd.Flags().BoolVar(&noBob, "no-bob", false, "skip schema reading, use existing models")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite files even if they have been manually edited")

	return cmd
}

// kilnPlugin generates the kiln API layer when bob calls PlugDBInfo.
// Bob reads the schema and generates models; this plugin receives the
// parsed schema and generates types, store, handlers, router, and OpenAPI.
type kilnPlugin[T, C, I any] struct {
	cfg   *config.Config
	table string
	force bool
}

func (p *kilnPlugin[T, C, I]) Name() string { return "kiln" }

func (p *kilnPlugin[T, C, I]) PlugDBInfo(info *drivers.DBInfo[T, C, I]) error {
	driver := bobadapter.DriverFromString(p.cfg.Database.Driver)

	exclude := make(map[string]bool)
	for _, name := range p.cfg.Tables.Exclude {
		exclude[name] = true
	}

	schema := bobadapter.ConvertDBInfo(info, driver, exclude)
	fmt.Printf("  ✓ Found %d tables\n", len(schema.Tables))

	// Config enum overrides take precedence over auto-detected.
	for _, t := range schema.Tables {
		override := p.cfg.OverrideFor(t.Name)
		for _, c := range t.Columns {
			if vals := override.EnumValuesFor(c.Name); len(vals) > 0 {
				c.EnumValues = vals
			}
		}
	}

	if p.table != "" {
		var err error
		schema, err = schema.FilterTable(p.table)
		if err != nil {
			return err
		}
	}

	g := generator.New(p.cfg, schema)
	g.SetForce(p.force)
	return g.Run(os.Stdout)
}

// runBobWithKiln runs bob's pipeline with kiln as a plugin.
// Bob reads the schema, generates models via its own plugins,
// and kiln generates the API layer via PlugDBInfo.
func runBobWithKiln(cfg *config.Config, table string, force bool) error {
	dsn, err := cfg.Database.ResolvedDSN()
	if err != nil {
		return fmt.Errorf("resolving database DSN: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	modelsDir := cfg.Bob.ModelsDir
	if modelsDir == "" {
		modelsDir = "./models"
	}

	bobConfig := gen.Config[any]{}
	bobState := &gen.State[any]{Config: bobConfig}

	// Configure bob plugins to output to the models directory.
	pluginsCfg := plugins.Config{}
	pluginsCfg.Models.Destination = modelsDir
	pluginsCfg.Models.Pkgname = "models"
	pluginsCfg.DBInfo.Destination = "dbinfo"
	pluginsCfg.DBInfo.Pkgname = "dbinfo"

	switch cfg.Database.Driver {
	case "postgres":
		driverCfg := psqldriver.Config{}
		driverCfg.Dsn = dsn
		bobPlugins := plugins.Setup[any, any, psqldriver.IndexExtra](pluginsCfg, gen.PSQLTemplates)
		kiln := &kilnPlugin[any, any, psqldriver.IndexExtra]{cfg: cfg, table: table, force: force}
		return gen.Run(ctx, bobState, psqldriver.New(driverCfg), append(bobPlugins, kiln)...)

	case "mysql":
		driverCfg := mysqldriver.Config{}
		driverCfg.Dsn = dsn
		bobPlugins := plugins.Setup[any, any, any](pluginsCfg, gen.MySQLTemplates)
		kiln := &kilnPlugin[any, any, any]{cfg: cfg, table: table, force: force}
		return gen.Run(ctx, bobState, mysqldriver.New(driverCfg), append(bobPlugins, kiln)...)

	case "sqlite":
		driverCfg := sqlitedriver.Config{}
		driverCfg.Dsn = dsn
		bobPlugins := plugins.Setup[any, any, any](pluginsCfg, gen.SQLiteTemplates)
		kiln := &kilnPlugin[any, any, any]{cfg: cfg, table: table, force: force}
		return gen.Run(ctx, bobState, sqlitedriver.New(driverCfg), append(bobPlugins, kiln)...)

	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}
}
