package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/ir"
	"github.com/fisayoafolayan/kiln/internal/parser/bob"
)

// prompt prints a label with an optional default and reads a line from stdin.
func prompt(r *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "  WARNING: error reading input: %v\n", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// confirm prints a yes/no prompt and returns true if the user answers y.
func confirm(label string) bool {
	r := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", label)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "  WARNING: error reading input: %v\n", err)
	}
	return strings.ToLower(strings.TrimSpace(line)) == "y"
}

// toIRDriver maps the config driver string to an ir.Driver constant.
func toIRDriver(driver string) ir.Driver {
	switch driver {
	case "postgres":
		return ir.DriverPostgres
	case "mysql":
		return ir.DriverMySQL
	case "sqlite":
		return ir.DriverSQLite
	default:
		return ir.DriverPostgres
	}
}

// parseSchema loads the config and parses the bob models into an ir.Schema.
func parseSchema() (*config.Config, *ir.Schema, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, nil, err
	}
	schema, err := parseSchemaWithConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cfg, schema, nil
}

// parseSchemaWithConfig parses the bob models using an already-loaded config.
func parseSchemaWithConfig(cfg *config.Config) (*ir.Schema, error) {
	p := bob.New(cfg.Bob.ModelsDir, toIRDriver(cfg.Database.Driver))
	p.Exclude = cfg.Tables.Exclude
	schema, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}
	return schema, nil
}
