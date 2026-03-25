package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a kiln.yaml config file interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(cfgFile); err == nil {
				if !confirm(fmt.Sprintf("%q already exists. Overwrite?", cfgFile)) {
					fmt.Println("Aborted.")
					return nil
				}
			}

			r := bufio.NewReader(os.Stdin)

			fmt.Println(`
┌─────────────────────────────────────────┐
│  kiln - schema-driven Go API generator  │
└─────────────────────────────────────────┘

kiln reads your database schema and generates a complete,
idiomatic Go API - models, store, handlers, router, and OpenAPI spec.

Under the hood, kiln uses bob (github.com/stephenafamo/bob) to
introspect your database. You don't need to know how bob works -
kiln handles it for you.

Let's get started.`)

			driver := prompt(r, "Database driver (postgres/mysql/sqlite)", "postgres")
			dsn := prompt(r, "Database DSN (or leave blank to use an env var)", "")
			dsnEnv := ""
			if dsn == "" {
				dsnEnv = prompt(r, "Environment variable name for DSN", "DATABASE_URL")
			}
			outDir := prompt(r, "Output directory", "./generated")
			basePath := prompt(r, "API base path", "/api/v1")

			modelsDir := "./models"

			// --- Write kiln.yaml ---
			dsnField := ""
			if dsn != "" {
				dsnField = fmt.Sprintf("  dsn: %q", dsn)
			} else {
				dsnField = fmt.Sprintf("  dsn_env: %q", dsnEnv)
			}

			kilnYAML := fmt.Sprintf(`version: 1

database:
  driver: %q
%s

output:
  dir: %q
  package: generated

api:
  base_path: %q
  framework: stdlib

bob:
  enabled: true
  models_dir: %q

generate:
  models: true
  store: true
  handlers: true
  router: true
  openapi: true

openapi:
  enabled: true
  output: ./docs/openapi.yaml
  title: My API
  version: 1.0.0

tables:
  exclude:
    - schema_migrations
`, driver, dsnField, outDir, basePath, modelsDir)

			if err := os.WriteFile(cfgFile, []byte(kilnYAML), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", cfgFile, err)
			}
			fmt.Printf("  ✓ Created %s\n", cfgFile)

			fmt.Println(`
Setup complete. Run:

  kiln generate`)
			return nil
		},
	}
}
