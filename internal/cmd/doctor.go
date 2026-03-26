package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator/genopt"
	"github.com/spf13/cobra"
)

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check project health and report issues",
		Long:  "Validates your kiln setup: config, schema, generated files, and overrides.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var warnings, errors int
			var pluginMode bool

			// ── Config ──────────────────────────────────────────────
			fmt.Println("\n  Config")
			cfg, err := config.Load(cfgFile)
			if err != nil {
				// If standalone load fails, try plugin mode (no DSN required).
				cfg, err = config.LoadForPlugin(cfgFile)
				if err != nil {
					fmt.Printf("    ✗ %v\n", err)
					// Can't continue without config.
					fmt.Printf("\n  1 error\n\n")
					return nil
				}
				pluginMode = true
			}
			fmt.Printf("    ✓ kiln.yaml loaded\n")
			if pluginMode {
				fmt.Printf("    ✓ plugin mode (no database DSN required)\n")
			}
			fmt.Printf("    ✓ driver: %s\n", cfg.Database.Driver)
			fmt.Printf("    ✓ output: %s\n", cfg.Output.Dir)

			// ── Project ─────────────────────────────────────────────
			fmt.Println("\n  Project")
			modPath := resolveModulePath()
			if modPath == "" {
				fmt.Println("    ✗ go.mod not found or missing module path")
				errors++
			} else {
				fmt.Printf("    ✓ go.mod found (module: %s)\n", modPath)
			}

			// ── Schema ──────────────────────────────────────────────
			fmt.Println("\n  Schema")
			schema, err := parseBobModels(cfg)
			if err != nil {
				fmt.Printf("    ✗ %v\n", err)
				errors++
				fmt.Printf("\n  %s\n\n", summary(warnings, errors))
				return nil
			}

			tableNames := make([]string, 0, len(schema.Tables))
			for _, t := range schema.Tables {
				tableNames = append(tableNames, t.Name)
			}
			fmt.Printf("    ✓ %d tables found (%s)\n", len(schema.Tables), strings.Join(tableNames, ", "))

			// Report M2M relationships.
			m2mCount := 0
			for _, t := range schema.Tables {
				for _, m2m := range t.ManyToMany {
					// Only count each junction once (from the first side).
					if t.Name < m2m.TargetTable.Name {
						m2mCount++
						fmt.Printf("    ✓ junction: %s (%s ↔ %s)\n", m2m.JunctionTable, t.Name, m2m.TargetTable.Name)
					}
				}
			}
			if m2mCount == 0 {
				fmt.Println("    - no junction tables detected")
			}

			// Report tables with no PK.
			noPK := 0
			for _, t := range schema.Tables {
				if !t.HasPK() {
					noPK++
					fmt.Printf("    ⚠ %s has no primary key (skipped during generation)\n", t.Name)
					warnings++
				} else if t.HasCompositePK() {
					noPK++
					fmt.Printf("    ⚠ %s has a composite primary key (skipped during generation — consider using a single auto-generated PK with a UNIQUE constraint)\n", t.Name)
					warnings++
				}
			}

			// ── Generated Files ─────────────────────────────────────
			fmt.Println("\n  Generated Files")
			schemaTableSet := make(map[string]bool, len(schema.Tables))
			for _, t := range schema.Tables {
				schemaTableSet[t.Name] = true
			}

			genFiles := scanGeneratedFiles(cfg.Output.Dir)
			if len(genFiles) == 0 {
				fmt.Println("    - no generated files found (run kiln generate first)")
			}
			for _, path := range genFiles {
				tableName := genopt.ExtractTableName(path)
				modified, err := genopt.FileIsUserModified(path)
				if err != nil {
					fmt.Printf("    ✗ %s: %v\n", relPath(path), err)
					errors++
					continue
				}

				if tableName != "" && !schemaTableSet[tableName] {
					fmt.Printf("    ✗ %s (stale - table %q not in schema)\n", relPath(path), tableName)
					errors++
				} else if modified {
					fmt.Printf("    ⚠ %s (user-modified)\n", relPath(path))
					warnings++
				} else {
					fmt.Printf("    ✓ %s\n", relPath(path))
				}
			}

			// ── Overrides ───────────────────────────────────────────
			if len(cfg.Overrides) > 0 {
				fmt.Println("\n  Overrides")
				for name, override := range cfg.Overrides {
					if !schemaTableSet[name] {
						fmt.Printf("    ⚠ %s: table not found in schema\n", name)
						warnings++
					} else {
						parts := describeOverride(override)
						if len(parts) > 0 {
							fmt.Printf("    ✓ %s: %s\n", name, strings.Join(parts, ", "))
						} else {
							fmt.Printf("    ✓ %s\n", name)
						}
					}
				}
			}

			fmt.Printf("\n  %s\n\n", summary(warnings, errors))
			return nil
		},
	}
}

// resolveModulePath reads the module path from go.mod.
func resolveModulePath() string {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	for _, line := range strings.SplitN(string(data), "\n", 10) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// scanGeneratedFiles returns all .go files in the output directory
// that have a kiln checksum header (auto-generated files).
func scanGeneratedFiles(outDir string) []string {
	var files []string
	// Scan models, store, handlers dirs + router.go at root.
	dirs := []string{
		filepath.Join(outDir, "models"),
		filepath.Join(outDir, "store"),
		filepath.Join(outDir, "handlers"),
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			// Include all .go files — doctor checks them for checksums.
			files = append(files, path)
		}
	}
	// Check router.go at output root.
	routerPath := filepath.Join(outDir, "router.go")
	if _, err := os.Stat(routerPath); err == nil {
		files = append(files, routerPath)
	}
	return files
}

// relPath returns the path relative to cwd, or the original path on error.
func relPath(path string) string {
	if rel, err := filepath.Rel(".", path); err == nil {
		return rel
	}
	return path
}

// describeOverride returns a human-readable summary of a table override.
func describeOverride(o config.TableOverride) []string {
	var parts []string
	if o.Endpoint != "" {
		parts = append(parts, "endpoint="+o.Endpoint)
	}
	if len(o.HiddenFields) > 0 {
		parts = append(parts, fmt.Sprintf("hidden_fields=%v", o.HiddenFields))
	}
	if len(o.ReadonlyFields) > 0 {
		parts = append(parts, fmt.Sprintf("readonly_fields=%v", o.ReadonlyFields))
	}
	if len(o.Disable) > 0 {
		parts = append(parts, fmt.Sprintf("disable=%v", o.Disable))
	}
	return parts
}

func summary(warnings, errors int) string {
	if errors == 0 && warnings == 0 {
		return "No issues found"
	}
	parts := []string{}
	if errors > 0 {
		parts = append(parts, fmt.Sprintf("%d error(s)", errors))
	}
	if warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warnings))
	}
	return strings.Join(parts, ", ")
}
