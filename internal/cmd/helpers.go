package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fisayoafolayan/kiln/internal/ir"
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
