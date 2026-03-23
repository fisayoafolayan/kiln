// Package genopt defines the Options type shared across all generators.
package genopt

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"strings"
	"text/template"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/ir"
)

// Options holds resolved values shared across all generators.
type Options struct {
	ModulePath string
	ImportPath string
	Dialect    Dialect // database dialect — drives template choices
	Config     *config.Config
	Schema     *ir.Schema
}

// Dialect represents the database dialect for template generation.
type Dialect struct {
	// BobPkg is the bob dialect package e.g. "psql", "mysql", "sqlite"
	BobPkg string
	// BobImport is the full import path for bob's dialect sm package
	// e.g. "github.com/stephenafamo/bob/dialect/psql/sm"
	SMImport string
	// DialectImport is the full import path for bob's dialect package
	// e.g. "github.com/stephenafamo/bob/dialect/psql"
	DialectImport string
	// DriverImport is the Go database driver import path
	DriverImport string
	// DriverName is the name passed to sql.Open
	DriverName string
}

// NewOptions builds an Options with dialect info derived from the config driver.
func NewOptions(modulePath string, cfg *config.Config, schema *ir.Schema) Options {
	outDir := strings.TrimPrefix(cfg.Output.Dir, "./")
	importPath := modulePath + "/" + outDir

	return Options{
		ModulePath: modulePath,
		ImportPath: importPath,
		Dialect:    dialectFor(cfg.Database.Driver),
		Config:     cfg,
		Schema:     schema,
	}
}

// ExecuteAndWrite executes a Go template, formats the output with gofmt,
// and writes the result to path. Returns an error if formatting fails
// (the unformatted output is still written for debugging).
func ExecuteAndWrite(tmpl *template.Template, data any, path string) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Write unformatted output so the user can debug the template,
		// but still return the formatting error.
		_ = os.WriteFile(path, buf.Bytes(), 0644)
		return fmt.Errorf("formatting %q: %w (unformatted output written for debugging)", path, err)
	}
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}

// dialectFor returns the Dialect for the given driver string.
func dialectFor(driver string) Dialect {
	switch driver {
	case "mysql":
		return Dialect{
			BobPkg:        "mysql",
			SMImport:      "github.com/stephenafamo/bob/dialect/mysql/sm",
			DialectImport: "github.com/stephenafamo/bob/dialect/mysql",
			DriverImport:  "github.com/go-sql-driver/mysql",
			DriverName:    "mysql",
		}
	case "sqlite":
		return Dialect{
			BobPkg:        "sqlite",
			SMImport:      "github.com/stephenafamo/bob/dialect/sqlite/sm",
			DialectImport: "github.com/stephenafamo/bob/dialect/sqlite",
			DriverImport:  "github.com/mattn/go-sqlite3",
			DriverName:    "sqlite3",
		}
	default: // postgres
		return Dialect{
			BobPkg:        "psql",
			SMImport:      "github.com/stephenafamo/bob/dialect/psql/sm",
			DialectImport: "github.com/stephenafamo/bob/dialect/psql",
			DriverImport:  "github.com/jackc/pgx/v5/stdlib",
			DriverName:    "pgx",
		}
	}
}
