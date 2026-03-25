// Package genopt defines the Options type shared across all generators.
package genopt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/format"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/ir"
)

const checksumPlaceholder = "__CHECKSUM__"

var checksumRe = regexp.MustCompile(`kiln:checksum=([a-f0-9]{64})`)
var tableTagRe = regexp.MustCompile(`kiln:table=(\S+)`)

// Options holds resolved values shared across all generators.
type Options struct {
	ModulePath string
	ImportPath string
	Dialect    Dialect // database dialect - drives template choices
	Config     *config.Config
	Schema     *ir.Schema
	Force      bool // overwrite user-modified files
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

// computeChecksum returns the SHA-256 hex digest of content.
func computeChecksum(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// embedChecksum computes a SHA-256 over content (which must contain the
// literal __CHECKSUM__ placeholder), then replaces the placeholder with
// the hex digest.
func embedChecksum(content []byte) []byte {
	sum := computeChecksum(content)
	return bytes.Replace(content, []byte(checksumPlaceholder), []byte(sum), 1)
}

// FileIsUserModified reports whether the file at path has been edited
// since kiln last generated it. It returns false (not modified) when
// the file does not exist or has no embedded checksum.
func FileIsUserModified(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	m := checksumRe.FindSubmatch(data)
	if m == nil {
		// No valid checksum marker - treat as unmanaged (allow overwrite).
		return false, nil
	}

	storedSum := string(m[1])
	// Replace the stored checksum back with the placeholder and re-hash.
	withPlaceholder := checksumRe.ReplaceAll(data, []byte("kiln:checksum="+checksumPlaceholder))
	actual := computeChecksum(withPlaceholder)
	return storedSum != actual, nil
}

// ExecuteAndWrite executes a Go template, formats the output with gofmt,
// embeds a content checksum, and writes the result to path.
// If the file was previously generated and the user has modified it,
// the write is skipped (returns skipped=true) unless force is true.
func ExecuteAndWrite(tmpl *template.Template, data any, path string, force bool) (skipped bool, err error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return false, fmt.Errorf("executing template: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Write unformatted output so the user can debug the template,
		// but still return the formatting error.
		_ = os.WriteFile(path, buf.Bytes(), 0644)
		return false, fmt.Errorf("formatting %q: %w (unformatted output written for debugging)", path, err)
	}

	if !force {
		modified, err := FileIsUserModified(path)
		if err != nil {
			return false, fmt.Errorf("checking %q: %w", path, err)
		}
		if modified {
			fmt.Fprintf(os.Stderr, "  ⚠ SKIPPED  %s (user-modified; use --force to overwrite)\n", path)
			return true, nil
		}
	}

	final := embedChecksum(formatted)
	if err := os.WriteFile(path, final, 0644); err != nil {
		return false, fmt.Errorf("writing %q: %w", path, err)
	}
	return false, nil
}

// WriteRawWithChecksum embeds a checksum and writes raw (non-Go) content
// to path, with the same user-modification guard as ExecuteAndWrite.
func WriteRawWithChecksum(content []byte, path string, force bool) (skipped bool, err error) {
	if !force {
		modified, err := FileIsUserModified(path)
		if err != nil {
			return false, fmt.Errorf("checking %q: %w", path, err)
		}
		if modified {
			fmt.Fprintf(os.Stderr, "  ⚠ SKIPPED  %s (user-modified; use --force to overwrite)\n", path)
			return true, nil
		}
	}

	final := embedChecksum(content)
	if err := os.WriteFile(path, final, 0644); err != nil {
		return false, fmt.Errorf("writing %q: %w", path, err)
	}
	return false, nil
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

// ExtractTableName reads the kiln:table=<name> tag from a generated file's
// header comment. Returns empty string if the file has no table tag.
func ExtractTableName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Only check the first few lines (header comments).
	lines := strings.SplitN(string(data), "\n", 5)
	for _, line := range lines {
		if m := tableTagRe.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}
