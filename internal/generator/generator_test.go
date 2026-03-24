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
		"models/users.go",
		"models/posts.go",
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

	content, err := os.ReadFile(filepath.Join(outDir, "models/users.go"))
	if err != nil {
		t.Fatalf("reading models/users.go: %v", err)
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
			t.Errorf("%s: expected %q in models/users.go but not found", c.desc, c.substr)
		}
		if !c.present && found {
			t.Errorf("%s: expected %q to be absent from models/users.go but was found", c.desc, c.substr)
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

func TestChiRouterGeneratesValidGo(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)
	cfg.API.Framework = "chi"

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
		{"chi import", "go-chi/chi/v5"},
		{"chi router param", "chi.Router"},
		{"chi Get method", ".Get("},
		{"chi Post method", ".Post("},
		{"chi Patch method", ".Patch("},
		{"chi Delete method", ".Delete("},
	}

	for _, c := range checks {
		if !strings.Contains(src, c.substr) {
			t.Errorf("%s: expected %q in router.go but not found", c.desc, c.substr)
		}
	}

	// Should NOT contain stdlib mux references.
	if strings.Contains(src, "http.ServeMux") {
		t.Error("chi router should not contain http.ServeMux")
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
// Level 2: Checksum tests
// ─────────────────────────────────────────────────────────────────────────────

func TestChecksumIsEmbedded(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)

	g := generator.New(cfg, testSchema())
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("generator.Run() failed: %v", err)
	}

	// Check a generated Go file has a real checksum, not the placeholder.
	content, err := os.ReadFile(filepath.Join(outDir, "models/users.go"))
	if err != nil {
		t.Fatalf("reading models/users.go: %v", err)
	}
	if strings.Contains(string(content), "__CHECKSUM__") {
		t.Error("models/users.go still contains __CHECKSUM__ placeholder")
	}
	if !strings.Contains(string(content), "kiln:checksum=") {
		t.Error("models/users.go missing checksum marker")
	}
}

func TestUserEditedFileIsSkipped(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)
	schema := testSchema()

	// First run — generates files with checksums.
	g := generator.New(cfg, schema)
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("first Run() failed: %v", err)
	}

	typesPath := filepath.Join(outDir, "models/users.go")
	original, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("reading models/users.go: %v", err)
	}

	// Simulate a user edit.
	edited := append(original, []byte("\n// user customisation\n")...)
	if err := os.WriteFile(typesPath, edited, 0644); err != nil {
		t.Fatalf("writing edited file: %v", err)
	}

	// Second run — should skip the edited file.
	g2 := generator.New(cfg, schema)
	if err := g2.Run(os.Stdout); err != nil {
		t.Fatalf("second Run() failed: %v", err)
	}

	after, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("reading after second run: %v", err)
	}
	if string(after) != string(edited) {
		t.Error("user-edited file was overwritten — checksum guard failed")
	}
}

func TestForceOverwritesEditedFile(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)
	schema := testSchema()

	// First run.
	g := generator.New(cfg, schema)
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("first Run() failed: %v", err)
	}

	typesPath := filepath.Join(outDir, "models/users.go")
	original, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("reading models/users.go: %v", err)
	}

	// Simulate a user edit.
	edited := append(original, []byte("\n// user customisation\n")...)
	if err := os.WriteFile(typesPath, edited, 0644); err != nil {
		t.Fatalf("writing edited file: %v", err)
	}

	// Second run with force — should overwrite.
	g2 := generator.New(cfg, schema)
	g2.SetForce(true)
	if err := g2.Run(os.Stdout); err != nil {
		t.Fatalf("second Run() with force failed: %v", err)
	}

	after, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("reading after force run: %v", err)
	}
	if strings.Contains(string(after), "user customisation") {
		t.Error("force run did not overwrite user-edited file")
	}
}

func TestUnmodifiedFileIsRegenerated(t *testing.T) {
	outDir := t.TempDir()
	cfg := testConfig(t, outDir)
	schema := testSchema()

	// First run.
	g := generator.New(cfg, schema)
	if err := g.Run(os.Stdout); err != nil {
		t.Fatalf("first Run() failed: %v", err)
	}

	typesPath := filepath.Join(outDir, "models/users.go")
	original, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("reading models/users.go: %v", err)
	}

	// Second run without edits — file should be regenerated (content unchanged).
	g2 := generator.New(cfg, schema)
	if err := g2.Run(os.Stdout); err != nil {
		t.Fatalf("second Run() failed: %v", err)
	}

	after, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("reading after second run: %v", err)
	}
	if string(after) != string(original) {
		t.Error("unmodified file content changed after regeneration")
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
