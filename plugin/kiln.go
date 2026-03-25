package plugin

import (
	"fmt"
	"io"
	"os"

	"github.com/fisayoafolayan/kiln/internal/bobadapter"
	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator"
	"github.com/stephenafamo/bob/gen/drivers"
)

// Options configures the kiln plugin.
type Options struct {
	// ConfigPath is the path to kiln.yaml. If empty, uses "kiln.yaml".
	ConfigPath string

	// ModulePath overrides the Go module path. If empty, read from go.mod.
	ModulePath string

	// Force overwrites user-modified files.
	Force bool

	// Output is where status messages are written. Defaults to os.Stdout.
	Output io.Writer
}

// Plugin implements bob's DBInfoPlugin[T, C, I] interface.
type Plugin[T, C, I any] struct {
	opts Options
}

// New creates a new kiln plugin with the given options.
func New[T, C, I any](opts Options) *Plugin[T, C, I] {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = "kiln.yaml"
	}
	return &Plugin[T, C, I]{opts: opts}
}

// Name satisfies the bob Plugin interface.
func (p *Plugin[T, C, I]) Name() string {
	return "kiln"
}

// PlugDBInfo is called by bob after database introspection.
func (p *Plugin[T, C, I]) PlugDBInfo(info *drivers.DBInfo[T, C, I]) error {
	cfg, err := config.LoadForPlugin(p.opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("kiln: loading config: %w", err)
	}

	driver := bobadapter.DriverFromString(cfg.Database.Driver)

	exclude := make(map[string]bool)
	for _, name := range cfg.Tables.Exclude {
		exclude[name] = true
	}

	schema := bobadapter.ConvertDBInfo(info, driver, exclude)

	fmt.Fprintf(p.opts.Output, "  [kiln] found %d tables\n", len(schema.Tables))

	// Config enum overrides take precedence.
	for _, t := range schema.Tables {
		override := cfg.OverrideFor(t.Name)
		for _, c := range t.Columns {
			if vals := override.EnumValuesFor(c.Name); len(vals) > 0 {
				c.EnumValues = vals
			}
		}
	}

	var g *generator.Generator
	if p.opts.ModulePath != "" {
		g = generator.NewWithModulePath(cfg, schema, p.opts.ModulePath)
	} else {
		g = generator.New(cfg, schema)
	}
	g.SetForce(p.opts.Force)
	return g.Run(p.opts.Output)
}
