// Package config loads and validates kiln.yaml.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "kiln.yaml"

// Config is the top-level structure of kiln.yaml.
type Config struct {
	Version  int            `yaml:"version"`
	Database DatabaseConfig `yaml:"database"`
	Output   OutputConfig   `yaml:"output"`
	API      APIConfig      `yaml:"api"`
	Bob      BobConfig      `yaml:"bob"`
	Tables   TablesConfig   `yaml:"tables"`
	Generate GenerateConfig `yaml:"generate"`
	Auth     AuthConfig     `yaml:"auth"`
	OpenAPI  OpenAPIConfig  `yaml:"openapi"`
	Tests    TestsConfig    `yaml:"tests"`
	// Per-table overrides keyed by table name.
	Overrides map[string]TableOverride `yaml:"overrides"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"` // postgres | mysql | sqlite
	DSN    string `yaml:"dsn"`
	// DSNEnv is an environment variable name that holds the DSN.
	// If set, takes precedence over DSN.
	DSNEnv string `yaml:"dsn_env"`
}

// ResolvedDSN returns the DSN, resolving from environment if DSNEnv is set.
func (d DatabaseConfig) ResolvedDSN() (string, error) {
	if d.DSNEnv != "" {
		val := os.Getenv(d.DSNEnv)
		if val == "" {
			return "", fmt.Errorf("environment variable %q is not set", d.DSNEnv)
		}
		return val, nil
	}
	if d.DSN == "" {
		return "", errors.New("database.dsn or database.dsn_env must be set")
	}
	return d.DSN, nil
}

type OutputConfig struct {
	Dir     string `yaml:"dir"`     // default: ./generated
	Package string `yaml:"package"` // default: generated
}

type APIConfig struct {
	BasePath  string `yaml:"base_path"` // default: /api/v1
	Framework string `yaml:"framework"` // stdlib | chi | gin (default: stdlib)
}

type BobConfig struct {
	Enabled    bool   `yaml:"enabled"`    // default: true
	ModelsDir  string `yaml:"models_dir"` // default: ./generated/models
	MinVersion string `yaml:"min_version"`
}

type TablesConfig struct {
	Include []string `yaml:"include"` // if set, only generate these tables
	Exclude []string `yaml:"exclude"` // always skip these tables
}

// GenerateConfig controls which layers are generated.
// Defaults to all true — opt individual layers out for brownfield adoption.
type GenerateConfig struct {
	Types    *bool `yaml:"types"`
	Store    *bool `yaml:"store"`
	Handlers *bool `yaml:"handlers"`
	Router   *bool `yaml:"router"`
	OpenAPI  *bool `yaml:"openapi"`
}

// IsEnabled returns whether a given layer should be generated.
// Defaults to true if the field is nil (not set in yaml).
func (g GenerateConfig) IsEnabled(layer string) bool {
	var b *bool
	switch layer {
	case "types":
		b = g.Types
	case "store":
		b = g.Store
	case "handlers":
		b = g.Handlers
	case "router":
		b = g.Router
	case "openapi":
		b = g.OpenAPI
	}
	if b == nil {
		return true // default on
	}
	return *b
}

type AuthConfig struct {
	Strategy string `yaml:"strategy"` // jwt | api_key | none (default: none)
	Header   string `yaml:"header"`   // default: Authorization
}

type OpenAPIConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Output      string `yaml:"output"`
	Title       string `yaml:"title"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

type TestsConfig struct {
	Enabled bool   `yaml:"enabled"`
	DSNEnv  string `yaml:"db_dsn_env"`
}

// TableOverride holds per-table customisations.
type TableOverride struct {
	Endpoint       string   `yaml:"endpoint"`        // override the URL path segment
	ReadonlyFields []string `yaml:"readonly_fields"` // excluded from Create/Update
	HiddenFields   []string `yaml:"hidden_fields"`   // excluded from all responses
	Disable        []string `yaml:"disable"`         // operations to disable: create|update|delete|list|get
	Filters        []string `yaml:"filters"`         // fields allowed as query filters
}

// IsOperationDisabled returns true if the given operation is disabled
// for this table. op should be one of: create, update, delete, list, get.
func (o TableOverride) IsOperationDisabled(op string) bool {
	for _, d := range o.Disable {
		if strings.EqualFold(d, op) {
			return true
		}
	}
	return false
}

// IsFieldHidden returns true if the field should be excluded from responses.
func (o TableOverride) IsFieldHidden(field string) bool {
	for _, f := range o.HiddenFields {
		if f == field {
			return true
		}
	}
	return false
}

// IsFieldReadonly returns true if the field should be excluded from
// Create and Update request structs.
func (o TableOverride) IsFieldReadonly(field string) bool {
	for _, f := range o.ReadonlyFields {
		if f == field {
			return true
		}
	}
	return false
}

// Load reads and validates a kiln.yaml file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"config file not found at %q — run `kiln init` to create one",
				path,
			)
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	cfg.applyDefaults()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// LoadFromDir looks for kiln.yaml in the given directory.
func LoadFromDir(dir string) (*Config, error) {
	return Load(filepath.Join(dir, DefaultConfigFile))
}

// applyDefaults fills in zero values with sensible defaults.
func (c *Config) applyDefaults() {
	if c.Version == 0 {
		c.Version = 1
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "postgres"
	}
	if c.Output.Dir == "" {
		c.Output.Dir = "./generated"
	}
	if c.Output.Package == "" {
		c.Output.Package = "generated"
	}
	if c.API.BasePath == "" {
		c.API.BasePath = "/api/v1"
	}
	if c.API.Framework == "" {
		c.API.Framework = "stdlib"
	}
	if c.Bob.ModelsDir == "" {
		c.Bob.ModelsDir = "./generated/models"
	}
	// Bob is enabled by default. Only disable if explicitly set to false.
	// We use pointer-less bool here so zero value = enabled.
	// The yaml `enabled: false` will correctly set this to false.
	if !c.Bob.Enabled && c.Bob.ModelsDir == "./generated/models" {
		// If not explicitly disabled, enable bob.
		c.Bob.Enabled = true
	}
	if c.Auth.Header == "" {
		c.Auth.Header = "Authorization"
	}
	if c.Auth.Strategy == "" {
		c.Auth.Strategy = "none"
	}
	if c.OpenAPI.Output == "" {
		c.OpenAPI.Output = "./docs/openapi.yaml"
	}
	if c.OpenAPI.Version == "" {
		c.OpenAPI.Version = "1.0.0"
	}
}

// validate checks for required fields and invalid combinations.
func (c *Config) validate() error {
	var errs []string

	// Database driver
	validDrivers := map[string]bool{
		"postgres": true,
		"mysql":    true,
		"sqlite":   true,
	}
	if !validDrivers[c.Database.Driver] {
		errs = append(errs, fmt.Sprintf(
			"database.driver %q is invalid — must be one of: postgres, mysql, sqlite",
			c.Database.Driver,
		))
	}

	// DSN presence check (not format — that's validated at connection time)
	if c.Database.DSN == "" && c.Database.DSNEnv == "" {
		errs = append(errs, "database.dsn or database.dsn_env is required")
	}

	// Framework
	validFrameworks := map[string]bool{
		"stdlib": true,
		"chi":    true,
		"gin":    true,
	}
	if !validFrameworks[c.API.Framework] {
		errs = append(errs, fmt.Sprintf(
			"api.framework %q is invalid — must be one of: stdlib, chi, gin",
			c.API.Framework,
		))
	}

	// Auth strategy
	validStrategies := map[string]bool{
		"none":    true,
		"jwt":     true,
		"api_key": true,
	}
	if !validStrategies[c.Auth.Strategy] {
		errs = append(errs, fmt.Sprintf(
			"auth.strategy %q is invalid — must be one of: none, jwt, api_key",
			c.Auth.Strategy,
		))
	}

	// Include and Exclude are mutually exclusive
	if len(c.Tables.Include) > 0 && len(c.Tables.Exclude) > 0 {
		errs = append(errs, "tables.include and tables.exclude cannot both be set")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n  "))
	}
	return nil
}

// ShouldGenerateTable returns true if the given table name should be
// processed, taking into account tables.include and tables.exclude.
func (c *Config) ShouldGenerateTable(name string) bool {
	if len(c.Tables.Include) > 0 {
		for _, t := range c.Tables.Include {
			if t == name {
				return true
			}
		}
		return false
	}
	for _, t := range c.Tables.Exclude {
		if t == name {
			return false
		}
	}
	return true
}

// OverrideFor returns the TableOverride for the given table name.
// Returns an empty TableOverride if none is configured.
func (c *Config) OverrideFor(table string) TableOverride {
	if c.Overrides == nil {
		return TableOverride{}
	}
	return c.Overrides[table]
}
