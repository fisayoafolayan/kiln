package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator"
	"github.com/spf13/cobra"
)

func generateCmd() *cobra.Command {
	var (
		table  string
		noBob  bool
		dryRun bool
		force  bool
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

			// Step 1 - ensure bob is available, write bobgen.yaml, run schema introspection
			if cfg.Bob.IsEnabled() && !noBob {
				if err := ensureBob(cfg.Database.Driver); err != nil {
					return err
				}
				// Write bobgen.yaml if it doesn't exist.
				if _, err := os.Stat("bobgen.yaml"); os.IsNotExist(err) {
					dsn, err := cfg.Database.ResolvedDSN()
					if err != nil {
						return fmt.Errorf("resolving database DSN: %w", err)
					}
					bobConfig := buildBobConfig(cfg.Database.Driver, dsn, cfg.Bob.ModelsDir)
					if err := os.WriteFile("bobgen.yaml", []byte(bobConfig), 0644); err != nil {
						return fmt.Errorf("writing bobgen.yaml: %w", err)
					}
				}
				fmt.Println("  Reading database schema...")
				if err := runBobGen(cfg.Database.Driver); err != nil {
					return fmt.Errorf("failed to read schema: %w\n\n"+
						"  Make sure your database is running and the DSN in kiln.yaml is correct.", err)
				}
				fmt.Println("  ✓ Schema read complete")
			}

			// Step 2 - parse bob's generated models into kiln IR
			fmt.Println("  Parsing schema...")
			schema, err := parseSchemaWithConfig(cfg)
			if err != nil {
				return err
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
			g.SetForce(force)
			if dryRun {
				return g.Diff(os.Stdout)
			}
			return g.Run(os.Stdout)
		},
	}

	cmd.Flags().StringVar(&table, "table", "", "only regenerate a specific table")
	cmd.Flags().BoolVar(&noBob, "no-bob", false, "skip schema reading, use existing models")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be generated without writing files")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite files even if they have been manually edited")

	return cmd
}

type bobGen struct {
	binary string
	module string
}

var bobGens = map[string]bobGen{
	"mysql":  {binary: "bobgen-mysql", module: "github.com/stephenafamo/bob/gen/bobgen-mysql@latest"},
	"sqlite": {binary: "bobgen-sqlite", module: "github.com/stephenafamo/bob/gen/bobgen-sqlite@latest"},
}

var defaultBobGen = bobGen{binary: "bobgen-psql", module: "github.com/stephenafamo/bob/gen/bobgen-psql@latest"}

func bobGenFor(driver string) bobGen {
	if bg, ok := bobGens[driver]; ok {
		return bg
	}
	return defaultBobGen
}

func bobGenBinary(driver string) string { return bobGenFor(driver).binary }
func bobGenModule(driver string) string { return bobGenFor(driver).module }

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
