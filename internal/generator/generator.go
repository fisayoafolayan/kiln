// Package generator orchestrates all code generators.
package generator

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator/auth"
	"github.com/fisayoafolayan/kiln/internal/generator/genopt"
	"github.com/fisayoafolayan/kiln/internal/generator/handlers"
	"github.com/fisayoafolayan/kiln/internal/generator/mainfile"
	"github.com/fisayoafolayan/kiln/internal/generator/openapi"
	"github.com/fisayoafolayan/kiln/internal/generator/router"
	"github.com/fisayoafolayan/kiln/internal/generator/store"
	"github.com/fisayoafolayan/kiln/internal/generator/types"
	"github.com/fisayoafolayan/kiln/internal/ir"
)

// Generator orchestrates all code generators.
type Generator struct {
	opts genopt.Options
}

// New returns a Generator configured with the given config and schema.
// Reads go.mod from the current working directory to resolve the module path.
func New(cfg *config.Config, schema *ir.Schema) *Generator {
	return NewWithModulePath(cfg, schema, resolveModulePath())
}

// NewWithModulePath returns a Generator with an explicit module path.
// Used in tests where there is no go.mod in the working directory.
func NewWithModulePath(cfg *config.Config, schema *ir.Schema, modulePath string) *Generator {
	if modulePath == "" {
		fmt.Fprintln(os.Stderr, "  [generator] WARNING: module path is empty - imports will be broken. Is go.mod present?")
	} else {
		fmt.Fprintf(os.Stderr, "  [generator] module path: %s\n", modulePath)
	}
	// Apply config enum values to IR columns.
	for _, t := range schema.Tables {
		override := cfg.OverrideFor(t.Name)
		for _, c := range t.Columns {
			if vals := override.EnumValuesFor(c.Name); len(vals) > 0 {
				c.EnumValues = vals
			}
		}
	}

	return &Generator{
		opts: genopt.NewOptions(modulePath, cfg, schema),
	}
}

// SetForce enables overwriting user-modified files.
func (g *Generator) SetForce(force bool) { g.opts.Force = force }

// runnable is the interface shared by all sub-generators.
type runnable interface {
	Run() ([]string, error)
	Diff() []string
}

// newFunc is a constructor for a sub-generator.
type newFunc func(genopt.Options) (runnable, error)

// Run executes all enabled generators and writes output files.
func (g *Generator) Run(w io.Writer) error {
	// Warn about skipped tables and report M2M relationships.
	for _, t := range g.opts.Schema.Tables {
		if !g.opts.Config.ShouldGenerateTable(t.Name) {
			continue
		}
		if !t.HasPK() {
			fmt.Fprintf(w, "  ⚠ SKIPPED  %s (no primary key detected)\n", t.Name)
		} else if t.HasCompositePK() {
			fmt.Fprintf(w, "  ⚠ SKIPPED  %s (composite primary key — consider using a single auto-generated PK with a UNIQUE constraint)\n", t.Name)
		}
		for _, m2m := range t.ManyToMany {
			fmt.Fprintf(w, "  [kiln] %s ↔ %s (via %s)\n", t.Name, m2m.TargetTable.Name, m2m.JunctionTable)
		}
	}

	for _, s := range g.steps() {
		gen, err := s.newGen(g.opts)
		if err != nil {
			return err
		}
		written, err := gen.Run()
		if err != nil {
			return err
		}
		for _, f := range written {
			if _, err := fmt.Fprintf(w, "  ✓ %s\n", f); err != nil {
				return err
			}
		}
	}
	return nil
}

// Diff prints what would be generated without writing any files.
func (g *Generator) Diff(w io.Writer) error {
	for _, s := range g.steps() {
		gen, err := s.newGen(g.opts)
		if err != nil {
			return err
		}
		for _, f := range gen.Diff() {
			if _, err := fmt.Fprintf(w, "  + %s\n", f); err != nil {
				return err
			}
		}
	}
	return nil
}

// step pairs an enablement check with a generator constructor.
type step struct {
	enabled bool
	newGen  newFunc
}

// steps returns the ordered list of generators to run, filtered by config.
func (g *Generator) steps() []step {
	cfg := g.opts.Config
	all := []step{
		{cfg.Generate.IsEnabled("models"), asRunnable(types.New)},
		{cfg.Generate.IsEnabled("store"), asRunnable(store.New)},
		{cfg.Generate.IsEnabled("handlers"), asRunnable(handlers.New)},
		{cfg.Generate.IsEnabled("router"), asRunnable(router.New)},
		{cfg.Generate.IsEnabled("openapi") && cfg.OpenAPI.Enabled, asRunnable(openapi.New)},
		{cfg.Auth.Strategy != "none", asRunnable(auth.New)},
		{true, asRunnable(mainfile.New)}, // always run, write-once
	}
	var enabled []step
	for _, s := range all {
		if s.enabled {
			enabled = append(enabled, s)
		}
	}
	return enabled
}

// asRunnable wraps a typed sub-generator constructor into a newFunc.
func asRunnable[T runnable](fn func(genopt.Options) (T, error)) newFunc {
	return func(opts genopt.Options) (runnable, error) {
		return fn(opts)
	}
}

// resolveModulePath reads the module path from go.mod in the current
// working directory. Falls back to an empty string if not found.
func resolveModulePath() string {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	for _, line := range strings.SplitN(string(data), "\n", 10) {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "\r")
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
