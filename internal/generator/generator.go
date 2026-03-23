// Package generator orchestrates all code generators.
package generator

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fisayoafolayan/kiln/internal/config"
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
		fmt.Fprintln(os.Stderr, "  [generator] WARNING: module path is empty — imports will be broken. Is go.mod present?")
	} else {
		fmt.Fprintf(os.Stderr, "  [generator] module path: %s\n", modulePath)
	}
	return &Generator{
		opts: genopt.NewOptions(modulePath, cfg, schema),
	}
}

// Run executes all enabled generators and writes output files.
func (g *Generator) Run(w io.Writer) error {
	// Types
	if g.opts.Config.Generate.IsEnabled("types") {
		gen, err := types.New(g.opts)
		if err != nil {
			return err
		}
		written, err := gen.Run()
		if err != nil {
			return err
		}
		for _, f := range written {
			fmt.Fprintf(w, "  ✓ %s\n", f)
		}
	}

	// Store
	if g.opts.Config.Generate.IsEnabled("store") {
		gen, err := store.New(g.opts)
		if err != nil {
			return err
		}
		written, err := gen.Run()
		if err != nil {
			return err
		}
		for _, f := range written {
			fmt.Fprintf(w, "  ✓ %s\n", f)
		}
	}

	// Handlers
	if g.opts.Config.Generate.IsEnabled("handlers") {
		gen, err := handlers.New(g.opts)
		if err != nil {
			return err
		}
		written, err := gen.Run()
		if err != nil {
			return err
		}
		for _, f := range written {
			fmt.Fprintf(w, "  ✓ %s\n", f)
		}
	}

	// Router
	if g.opts.Config.Generate.IsEnabled("router") {
		gen, err := router.New(g.opts)
		if err != nil {
			return err
		}
		written, err := gen.Run()
		if err != nil {
			return err
		}
		for _, f := range written {
			fmt.Fprintf(w, "  ✓ %s\n", f)
		}
	}

	// OpenAPI
	if g.opts.Config.Generate.IsEnabled("openapi") && g.opts.Config.OpenAPI.Enabled {
		gen, err := openapi.New(g.opts)
		if err != nil {
			return err
		}
		written, err := gen.Run()
		if err != nil {
			return err
		}
		for _, f := range written {
			fmt.Fprintf(w, "  ✓ %s\n", f)
		}
	}

	// main.go — write-once wiring file
	{
		gen, err := mainfile.New(g.opts)
		if err != nil {
			return err
		}
		written, err := gen.Run()
		if err != nil {
			return err
		}
		for _, f := range written {
			fmt.Fprintf(w, "  ✓ %s\n", f)
		}
	}

	return nil
}

// Diff prints what would be generated without writing any files.
func (g *Generator) Diff(w io.Writer) error {
	if g.opts.Config.Generate.IsEnabled("types") {
		gen, err := types.New(g.opts)
		if err != nil {
			return err
		}
		for _, f := range gen.Diff() {
			fmt.Fprintf(w, "  + %s\n", f)
		}
	}

	if g.opts.Config.Generate.IsEnabled("store") {
		gen, err := store.New(g.opts)
		if err != nil {
			return err
		}
		for _, f := range gen.Diff() {
			fmt.Fprintf(w, "  + %s\n", f)
		}
	}

	if g.opts.Config.Generate.IsEnabled("handlers") {
		gen, err := handlers.New(g.opts)
		if err != nil {
			return err
		}
		for _, f := range gen.Diff() {
			fmt.Fprintf(w, "  + %s\n", f)
		}
	}

	if g.opts.Config.Generate.IsEnabled("router") {
		gen, err := router.New(g.opts)
		if err != nil {
			return err
		}
		for _, f := range gen.Diff() {
			fmt.Fprintf(w, "  + %s\n", f)
		}
	}

	if g.opts.Config.Generate.IsEnabled("openapi") && g.opts.Config.OpenAPI.Enabled {
		gen, err := openapi.New(g.opts)
		if err != nil {
			return err
		}
		for _, f := range gen.Diff() {
			fmt.Fprintf(w, "  + %s\n", f)
		}
	}

	// main.go diff
	{
		gen, err := mainfile.New(g.opts)
		if err != nil {
			return err
		}
		for _, f := range gen.Diff() {
			fmt.Fprintf(w, "  + %s\n", f)
		}
	}

	return nil
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
