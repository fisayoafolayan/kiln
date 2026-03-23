package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator"
	"github.com/fisayoafolayan/kiln/internal/parser/bob"
	"github.com/spf13/cobra"
)

func generateCmd() *cobra.Command {
	var (
		table  string
		noBob  bool
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Go code from your database schema",
		Long: `Generates your full API layer from the database schema.

kiln reads your database schema then generates types, store,
handlers, router, and OpenAPI spec.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			// Step 1 — ensure bob is available, offer to install if not
			if cfg.Bob.Enabled && !noBob {
				if err := ensureBob(cfg.Database.Driver); err != nil {
					return err
				}
				if _, err := os.Stat("bobgen.yaml"); os.IsNotExist(err) {
					return fmt.Errorf(
						"bobgen.yaml not found\n\n" +
							"  Run `kiln init` to set up your project first.",
					)
				}
				fmt.Println("  Reading database schema...")
				if err := runBobGen(cfg.Database.Driver); err != nil {
					return fmt.Errorf("failed to read schema: %w\n\n"+
						"  Make sure your database is running and the DSN in bob.toml is correct.", err)
				}
				fmt.Println("  ✓ Schema read complete")
			}

			// Step 2 — parse bob's generated models into kiln IR
			fmt.Println("  Parsing schema...")
			p := bob.New(cfg.Bob.ModelsDir, toIRDriver(cfg.Database.Driver))
			p.Exclude = cfg.Tables.Exclude
			schema, err := p.Parse()
			if err != nil {
				return fmt.Errorf("parsing schema: %w", err)
			}
			fmt.Printf("  ✓ Found %d tables\n", len(schema.Tables))

			// Step 3 — filter to a single table if --table is set
			if table != "" {
				schema, err = schema.FilterTable(table)
				if err != nil {
					return err
				}
			}

			// Step 4 — run generators
			g := generator.New(cfg, schema)
			if dryRun {
				return g.Diff(os.Stdout)
			}
			return g.Run(os.Stdout)
		},
	}

	cmd.Flags().StringVar(&table, "table", "", "only regenerate a specific table")
	cmd.Flags().BoolVar(&noBob, "no-bob", false, "skip schema reading, use existing models")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be generated without writing files")

	return cmd
}

// bobGenBinary returns the correct bob generator binary name for the given driver.
func bobGenBinary(driver string) string {
	switch driver {
	case "mysql":
		return "bobgen-mysql"
	case "sqlite":
		return "bobgen-sqlite"
	default:
		return "bobgen-psql"
	}
}

// bobGenModule returns the go install path for the given driver's bob generator.
func bobGenModule(driver string) string {
	switch driver {
	case "mysql":
		return "github.com/stephenafamo/bob/gen/bobgen-mysql@latest"
	case "sqlite":
		return "github.com/stephenafamo/bob/gen/bobgen-sqlite@latest"
	default:
		return "github.com/stephenafamo/bob/gen/bobgen-psql@latest"
	}
}

func ensureBob(driver string) error {
	bin := bobGenBinary(driver)
	if _, err := exec.LookPath(bin); err == nil {
		return nil
	}

	mod := bobGenModule(driver)
	fmt.Printf(`
  kiln uses %s to read your database schema.
  %s is not installed on your system.
`, bin, bin)

	if !confirm(fmt.Sprintf("  Install it now? (go install %s)", mod)) {
		return fmt.Errorf(
			"%s is required\n\n"+
				"  Install it manually:\n"+
				"  go install %s", bin, mod,
		)
	}

	fmt.Printf("  Installing %s...\n", bin)
	c := exec.Command("go", "install", mod)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to install %s: %w", bin, err)
	}
	fmt.Printf("  ✓ %s installed\n", bin)
	return nil
}

func runBobGen(driver string) error {
	bin := bobGenBinary(driver)
	c := exec.Command(bin)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
