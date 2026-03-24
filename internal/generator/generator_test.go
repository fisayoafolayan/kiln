package generator_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator"
	"github.com/fisayoafolayan/kiln/internal/ir"
)

// testSchema returns a minimal hardcoded ir.Schema that mimics
// the blog schema - no database connection required.
func testSchema() *ir.Schema {
	usersTable := &ir.Table{
		Name:      "users",
		ColumnMap: map[string]*ir.Column{},
	}

	cols := []*ir.Column{
		{Name: "id", GoType: ir.GoTypeUUID, IsPrimaryKey: true, HasDefault: true, Ordinal: 0},
		{Name: "email", GoType: ir.GoTypeString, Ordinal: 1},
		{Name: "name", GoType: ir.GoTypeString, Ordinal: 2},
		{Name: "bio", GoType: ir.NullableOf(ir.GoTypeString), Nullable: true, Ordinal: 3},
		{Name: "role", GoType: ir.GoTypeString, HasDefault: true, Ordinal: 4},
		{Name: "created_at", GoType: ir.GoTypeTime, HasDefault: true, Ordinal: 5},
		{Name: "updated_at", GoType: ir.GoTypeTime, HasDefault: true, Ordinal: 6},
	}

	for _, c := range cols {
		usersTable.Columns = append(usersTable.Columns, c)
		usersTable.ColumnMap[c.Name] = c
		if c.IsPrimaryKey {
			usersTable.PrimaryKey = c
		}
	}

	postsTable := &ir.Table{
		Name:      "posts",
		ColumnMap: map[string]*ir.Column{},
	}

	postCols := []*ir.Column{
		{Name: "id", GoType: ir.GoTypeUUID, IsPrimaryKey: true, HasDefault: true, Ordinal: 0},
		{Name: "user_id", GoType: ir.GoTypeUUID, Ordinal: 1},
		{Name: "title", GoType: ir.GoTypeString, Ordinal: 2},
		{Name: "body", GoType: ir.GoTypeString, Ordinal: 3},
		{Name: "status", GoType: ir.GoTypeString, HasDefault: true, Ordinal: 4},
		{Name: "published_at", GoType: ir.NullableOf(ir.GoTypeTime), Nullable: true, Ordinal: 5},
		{Name: "created_at", GoType: ir.GoTypeTime, HasDefault: true, Ordinal: 6},
		{Name: "updated_at", GoType: ir.GoTypeTime, HasDefault: true, Ordinal: 7},
	}

	for _, c := range postCols {
		postsTable.Columns = append(postsTable.Columns, c)
		postsTable.ColumnMap[c.Name] = c
		if c.IsPrimaryKey {
			postsTable.PrimaryKey = c
		}
	}

	// Wire up FK: posts.user_id → users.id
	fk := &ir.ForeignKey{
		SourceTable:  postsTable,
		SourceColumn: postsTable.ColumnMap["user_id"],
		TargetTable:  usersTable,
		TargetColumn: usersTable.PrimaryKey,
	}
	postsTable.ForeignKeys = append(postsTable.ForeignKeys, fk)
	usersTable.ReferencedBy = append(usersTable.ReferencedBy, fk)

	schema := &ir.Schema{
		Driver: ir.DriverPostgres,
		Tables: []*ir.Table{usersTable, postsTable},
		TableMap: map[string]*ir.Table{
			"users": usersTable,
			"posts": postsTable,
		},
	}

	return schema
}

// testConfig returns a minimal config pointing output to a temp dir.
func testConfig(t *testing.T, outDir string) *config.Config {
	t.Helper()
	return &config.Config{
		Output: config.OutputConfig{
			Dir:     outDir,
			Package: "generated",
		},
		API: config.APIConfig{
			BasePath:  "/api/v1",
			Framework: "stdlib",
		},
		Auth: config.AuthConfig{
			Strategy: "none",
			Header:   "Authorization",
		},
		Bob: config.BobConfig{
			ModelsDir: filepath.Join(outDir, "models"),
		},
		OpenAPI: config.OpenAPIConfig{
			Enabled: true,
			Output:  filepath.Join(outDir, "openapi.yaml"),
			Title:   "Test API",
			Version: "1.0.0",
		},
		Overrides: map[string]config.TableOverride{},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Level 1: File existence tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGeneratorCreatesExpectedFiles(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)
	cfg.Generate = config.GenerateConfig{} // all layers enabled (nil = true)

	g := generator.New(cfg, testSchema())
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("generator.Run() failed: %v", err)
	}

	expected := []string{
		"types/users.go",
		"types/posts.go",
		"store/users.go",
		"store/posts.go",
		"store/mappers/users.go",
		"store/mappers/posts.go",
		"handlers/users.go",
		"handlers/posts.go",
		"handlers/helpers.go",
		"router.go",
		"openapi.yaml",
		// cmd/server/main.go is skipped in tests — requires a real module path
	}

	for _, rel := range expected {
		path := filepath.Join(outDir, rel)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file not found: %s", path)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Level 2: Content correctness tests
// ─────────────────────────────────────────────────────────────────────────────

func TestTypesFileContainsExpectedTypes(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)

	g := generator.New(cfg, testSchema())
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("generator.Run() failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "types/users.go"))
	if err != nil {
		t.Fatalf("reading types/users.go: %v", err)
	}
	src := string(content)

	checks := []struct {
		desc    string
		present bool
		substr  string
	}{
		{"User response struct", true, "type User struct"},
		{"CreateUserRequest struct", true, "type CreateUserRequest struct"},
		{"UpdateUserRequest struct", true, "type UpdateUserRequest struct"},
		{"ListUsersResponse struct", true, "type ListUsersResponse struct"},
		{"nullable bio field", true, "*string"},
		{"created_at present in User struct", true, "CreatedAt time.Time"},
		{"id excluded from CreateUserRequest", false, "type CreateUserRequest struct {\n\tID"},
	}

	for _, c := range checks {
		found := strings.Contains(src, c.substr)
		if c.present && !found {
			t.Errorf("%s: expected %q in types/users.go but not found", c.desc, c.substr)
		}
		if !c.present && found {
			t.Errorf("%s: expected %q to be absent from types/users.go but was found", c.desc, c.substr)
		}
	}
}

func TestRouterContainsNestedRoute(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)

	g := generator.New(cfg, testSchema())
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("generator.Run() failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "router.go"))
	if err != nil {
		t.Fatalf("reading router.go: %v", err)
	}
	src := string(content)

	checks := []struct {
		desc   string
		substr string
	}{
		{"users list route", "GET /api/v1/users"},
		{"users create route", "POST /api/v1/users"},
		{"users get route", "GET /api/v1/users/{id}"},
		{"users update route", "PATCH /api/v1/users/{id}"},
		{"users delete route", "DELETE /api/v1/users/{id}"},
		{"nested posts by user route", "GET /api/v1/users/{id}/posts"},
	}

	for _, c := range checks {
		if !strings.Contains(src, c.substr) {
			t.Errorf("%s: expected %q in router.go but not found", c.desc, c.substr)
		}
	}
}

func TestMapperIsWriteOnce(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)
	schema := testSchema()

	// First run — should create the mapper
	g := generator.New(cfg, schema)
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("first Run() failed: %v", err)
	}

	mapperPath := filepath.Join(outDir, "store/mappers/users.go")
	original, err := os.ReadFile(mapperPath)
	if err != nil {
		t.Fatalf("reading mapper: %v", err)
	}

	// Simulate user editing the mapper
	edited := string(original) + "\n// custom user edit"
	if err := os.WriteFile(mapperPath, []byte(edited), 0644); err != nil {
		t.Fatalf("editing mapper: %v", err)
	}

	// Second run — should NOT overwrite the mapper
	g2 := generator.New(cfg, schema)
	if err := g2.Run(os.Stdout); err != nil {
		t.Fatalf("second Run() failed: %v", err)
	}

	after, err := os.ReadFile(mapperPath)
	if err != nil {
		t.Fatalf("reading mapper after second run: %v", err)
	}

	if string(after) != edited {
		t.Error("mapper was overwritten on second run — write-once guarantee broken")
	}
}

func TestOpenAPIContainsAllTables(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)

	g := generator.New(cfg, testSchema())
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("generator.Run() failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "openapi.yaml"))
	if err != nil {
		t.Fatalf("reading openapi.yaml: %v", err)
	}
	src := string(content)

	checks := []string{
		"/api/v1/users",
		"/api/v1/posts",
		"/api/v1/users/{id}/posts", // nested route
		"components:",
		"schemas:",
		"User:",
		"Post:",
		"CreateUserRequest:",
		"UpdateUserRequest:",
	}

	for _, check := range checks {
		if !strings.Contains(src, check) {
			t.Errorf("expected %q in openapi.yaml but not found", check)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Level 2: Compilation test
// ─────────────────────────────────────────────────────────────────────────────

// TestGeneratedFilesParseAsValidGo verifies every generated .go file
// is syntactically valid Go — no compiler, no network, no go.sum needed.
func TestGeneratedFilesParseAsValidGo(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)

	g := generator.New(cfg, testSchema())
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("generator.Run() failed: %v", err)
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(outDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if _, err := parser.ParseFile(fset, path, src, parser.AllErrors); err != nil {
			return fmt.Errorf("parse error in %s:\n%w", path, err)
		}
		t.Logf("  ✓ valid Go syntax: %s", strings.TrimPrefix(path, outDir+"/"))
		return nil
	})
	if err != nil {
		t.Errorf("generated file has syntax errors: %v", err)
	}
}
