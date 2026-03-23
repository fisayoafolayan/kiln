package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fisayoafolayan/kiln/internal/config"
)

// writeConfig writes a kiln.yaml to a temp dir and returns the path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kiln.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

// ─────────────────────────────────────────────────────────────────────────────
// Load - valid configs
// ─────────────────────────────────────────────────────────────────────────────

func TestLoad_MinimalValid(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: postgres
  dsn: "postgres://localhost/test"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Defaults applied
	if cfg.Output.Dir != "./generated" {
		t.Errorf("Output.Dir = %q, want ./generated", cfg.Output.Dir)
	}
	if cfg.Output.Package != "generated" {
		t.Errorf("Output.Package = %q, want generated", cfg.Output.Package)
	}
	if cfg.API.BasePath != "/api/v1" {
		t.Errorf("API.BasePath = %q, want /api/v1", cfg.API.BasePath)
	}
	if cfg.API.Framework != "stdlib" {
		t.Errorf("API.Framework = %q, want stdlib", cfg.API.Framework)
	}
	if !cfg.Bob.IsEnabled() {
		t.Error("Bob.IsEnabled() = false, want true")
	}
}

func TestLoad_FullConfig(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: mysql
  dsn: "user:pass@tcp(localhost:3306)/blog"
output:
  dir: ./out
  package: api
api:
  base_path: /v2
  framework: chi
bob:
  enabled: true
  models_dir: ./out/models
tables:
  exclude:
    - schema_migrations
    - audit_logs
overrides:
  users:
    endpoint: members
    hidden_fields:
      - password_hash
    disable:
      - delete
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.Driver != "mysql" {
		t.Errorf("Driver = %q, want mysql", cfg.Database.Driver)
	}
	if cfg.Output.Dir != "./out" {
		t.Errorf("Output.Dir = %q, want ./out", cfg.Output.Dir)
	}
	if cfg.API.Framework != "chi" {
		t.Errorf("Framework = %q, want chi", cfg.API.Framework)
	}
	if len(cfg.Tables.Exclude) != 2 {
		t.Errorf("len(Tables.Exclude) = %d, want 2", len(cfg.Tables.Exclude))
	}

	override := cfg.OverrideFor("users")
	if override.Endpoint != "members" {
		t.Errorf("override.Endpoint = %q, want members", override.Endpoint)
	}
	if !override.IsOperationDisabled("delete") {
		t.Error("delete should be disabled for users")
	}
	if !override.IsFieldHidden("password_hash") {
		t.Error("password_hash should be hidden")
	}
}

func TestLoad_DSNEnv(t *testing.T) {
	t.Setenv("TEST_DSN", "postgres://localhost/envtest")

	path := writeConfig(t, `
version: 1
database:
  driver: postgres
  dsn_env: TEST_DSN
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dsn, err := cfg.Database.ResolvedDSN()
	if err != nil {
		t.Fatalf("ResolvedDSN: %v", err)
	}
	if dsn != "postgres://localhost/envtest" {
		t.Errorf("DSN = %q, want postgres://localhost/envtest", dsn)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Load — invalid configs
// ─────────────────────────────────────────────────────────────────────────────

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/kiln.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidDriver(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: oracle
  dsn: "something"
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid driver, got nil")
	}
}

func TestLoad_InvalidFramework(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: postgres
  dsn: "postgres://localhost/test"
api:
  framework: express
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid framework, got nil")
	}
}

func TestLoad_IncludeAndExcludeMutuallyExclusive(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: postgres
  dsn: "postgres://localhost/test"
tables:
  include:
    - users
  exclude:
    - posts
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error when both include and exclude are set, got nil")
	}
}

func TestLoad_MissingDSN(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: postgres
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for missing DSN, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ShouldGenerateTable
// ─────────────────────────────────────────────────────────────────────────────

func TestShouldGenerateTable_NoFilter(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: postgres
  dsn: "postgres://localhost/test"
`)
	cfg, _ := config.Load(path)
	if !cfg.ShouldGenerateTable("users") {
		t.Error("users should be generated with no filter")
	}
}

func TestShouldGenerateTable_Include(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: postgres
  dsn: "postgres://localhost/test"
tables:
  include:
    - users
    - posts
`)
	cfg, _ := config.Load(path)

	if !cfg.ShouldGenerateTable("users") {
		t.Error("users should be generated")
	}
	if !cfg.ShouldGenerateTable("posts") {
		t.Error("posts should be generated")
	}
	if cfg.ShouldGenerateTable("tags") {
		t.Error("tags should NOT be generated")
	}
}

func TestShouldGenerateTable_Exclude(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: postgres
  dsn: "postgres://localhost/test"
tables:
  exclude:
    - schema_migrations
    - post_tags
`)
	cfg, _ := config.Load(path)

	if !cfg.ShouldGenerateTable("users") {
		t.Error("users should be generated")
	}
	if cfg.ShouldGenerateTable("schema_migrations") {
		t.Error("schema_migrations should NOT be generated")
	}
	if cfg.ShouldGenerateTable("post_tags") {
		t.Error("post_tags should NOT be generated")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GenerateConfig.IsEnabled
// ─────────────────────────────────────────────────────────────────────────────

func TestGenerateConfig_DefaultsAllEnabled(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: postgres
  dsn: "postgres://localhost/test"
`)
	cfg, _ := config.Load(path)

	for _, layer := range []string{"types", "store", "handlers", "router", "openapi"} {
		if !cfg.Generate.IsEnabled(layer) {
			t.Errorf("layer %q should be enabled by default", layer)
		}
	}
}

func TestGenerateConfig_PartialDisable(t *testing.T) {
	path := writeConfig(t, `
version: 1
database:
  driver: postgres
  dsn: "postgres://localhost/test"
generate:
  types: true
  store: true
  handlers: false
  router: false
  openapi: true
`)
	cfg, _ := config.Load(path)

	if !cfg.Generate.IsEnabled("types") {
		t.Error("types should be enabled")
	}
	if cfg.Generate.IsEnabled("handlers") {
		t.Error("handlers should be disabled")
	}
	if cfg.Generate.IsEnabled("router") {
		t.Error("router should be disabled")
	}
}
