package router

import (
	"testing"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator/genopt"
	"github.com/fisayoafolayan/kiln/internal/ir"
)

// helper to build a Generator with the given schema and config overrides.
func newTestGenerator(t *testing.T, schema *ir.Schema, cfg *config.Config) *Generator {
	t.Helper()
	if cfg.API.BasePath == "" {
		cfg.API.BasePath = "/api/v1"
	}
	if cfg.Output.Package == "" {
		cfg.Output.Package = "generated"
	}
	opts := genopt.Options{
		ModulePath: "github.com/example/app",
		ImportPath: "github.com/example/app/generated",
		Config:     cfg,
		Schema:     schema,
	}
	gen, err := New(opts)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return gen
}

func usersTable() *ir.Table {
	return &ir.Table{
		Name: "users",
		Columns: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
			{Name: "name", GoType: ir.GoTypeString},
		},
		PrimaryKeys: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
	}
}

func postsTable(users *ir.Table) *ir.Table {
	return &ir.Table{
		Name: "posts",
		Columns: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
			{Name: "title", GoType: ir.GoTypeString},
			{Name: "user_id", GoType: ir.GoTypeInt64},
		},
		PrimaryKeys: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
		ForeignKeys: []*ir.ForeignKey{
			{
				SourceTable:  nil, // set below
				SourceColumn: &ir.Column{Name: "user_id", GoType: ir.GoTypeInt64},
				TargetTable:  users,
				TargetColumn: &ir.Column{Name: "id", GoType: ir.GoTypeInt64},
			},
		},
	}
}

func TestTemplateData_BasicCRUD(t *testing.T) {
	users := usersTable()
	schema := &ir.Schema{Tables: []*ir.Table{users}}
	cfg := &config.Config{}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	if len(data.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(data.Tables))
	}

	routes := data.Tables[0].Routes
	want := []route{
		{Method: "GET", Path: "/api/v1/users", Handler: "users.List"},
		{Method: "POST", Path: "/api/v1/users", Handler: "users.Create"},
		{Method: "GET", Path: "/api/v1/users/{id}", Handler: "users.Get"},
		{Method: "PATCH", Path: "/api/v1/users/{id}", Handler: "users.Update"},
		{Method: "DELETE", Path: "/api/v1/users/{id}", Handler: "users.Delete"},
	}

	assertRoutes(t, routes, want)
}

func TestTemplateData_FKNestedRoutes(t *testing.T) {
	users := usersTable()
	posts := postsTable(users)
	posts.ForeignKeys[0].SourceTable = posts
	schema := &ir.Schema{Tables: []*ir.Table{users, posts}}
	cfg := &config.Config{}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	if len(data.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(data.Tables))
	}

	// Posts should have 5 CRUD + 1 nested route
	postRoutes := data.Tables[1].Routes
	if len(postRoutes) != 6 {
		t.Fatalf("expected 6 post routes, got %d", len(postRoutes))
	}

	nested := postRoutes[5]
	if nested.Method != "GET" {
		t.Errorf("nested route method = %q, want GET", nested.Method)
	}
	if nested.Path != "/api/v1/users/{id}/posts" {
		t.Errorf("nested route path = %q, want /api/v1/users/{id}/posts", nested.Path)
	}
	if nested.Handler != "posts.ListByUser" {
		t.Errorf("nested route handler = %q, want posts.ListByUser", nested.Handler)
	}
}

func TestTemplateData_M2MRoutes(t *testing.T) {
	posts := &ir.Table{
		Name: "posts",
		Columns: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
		PrimaryKeys: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
	}
	tags := &ir.Table{
		Name: "tags",
		Columns: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
		PrimaryKeys: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
	}
	posts.ManyToMany = []*ir.ManyToMany{
		{
			JunctionTable:     "post_tags",
			JunctionSourceCol: "post_id",
			JunctionTargetCol: "tag_id",
			TargetTable:       tags,
		},
	}

	schema := &ir.Schema{Tables: []*ir.Table{posts}}
	cfg := &config.Config{}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	postRoutes := data.Tables[0].Routes
	// 5 CRUD + 3 M2M (link, unlink, list)
	if len(postRoutes) != 8 {
		t.Fatalf("expected 8 routes, got %d", len(postRoutes))
	}

	m2mRoutes := postRoutes[5:]
	wantM2M := []route{
		{Method: "POST", Path: "/api/v1/posts/{id}/tags", Handler: "posts.LinkTag"},
		{Method: "DELETE", Path: "/api/v1/posts/{id}/tags/{tagId}", Handler: "posts.UnlinkTag"},
		{Method: "GET", Path: "/api/v1/posts/{id}/tags", Handler: "posts.ListLinkedTags"},
	}

	assertRoutes(t, m2mRoutes, wantM2M)
}

func TestTemplateData_DisableOperations(t *testing.T) {
	users := usersTable()
	schema := &ir.Schema{Tables: []*ir.Table{users}}
	cfg := &config.Config{
		Overrides: map[string]config.TableOverride{
			"users": {Disable: []string{"create", "delete"}},
		},
	}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	routes := data.Tables[0].Routes
	want := []route{
		{Method: "GET", Path: "/api/v1/users", Handler: "users.List"},
		{Method: "GET", Path: "/api/v1/users/{id}", Handler: "users.Get"},
		{Method: "PATCH", Path: "/api/v1/users/{id}", Handler: "users.Update"},
	}

	assertRoutes(t, routes, want)
}

func TestTemplateData_DisableLinkUnlink(t *testing.T) {
	posts := &ir.Table{
		Name: "posts",
		Columns: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
		PrimaryKeys: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
	}
	tags := &ir.Table{
		Name: "tags",
		Columns: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
		PrimaryKeys: []*ir.Column{
			{Name: "id", GoType: ir.GoTypeInt64, IsPrimaryKey: true, HasDefault: true},
		},
	}
	posts.ManyToMany = []*ir.ManyToMany{
		{
			JunctionTable:     "post_tags",
			JunctionSourceCol: "post_id",
			JunctionTargetCol: "tag_id",
			TargetTable:       tags,
		},
	}

	schema := &ir.Schema{Tables: []*ir.Table{posts}}
	cfg := &config.Config{
		Overrides: map[string]config.TableOverride{
			"posts": {Disable: []string{"link", "unlink"}},
		},
	}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	postRoutes := data.Tables[0].Routes
	// 5 CRUD + 1 ListLinked (link and unlink disabled)
	if len(postRoutes) != 6 {
		t.Fatalf("expected 6 routes, got %d", len(postRoutes))
	}

	last := postRoutes[5]
	if last.Method != "GET" || last.Handler != "posts.ListLinkedTags" {
		t.Errorf("expected ListLinkedTags route, got %s %s", last.Method, last.Handler)
	}
}

func TestTemplateData_EndpointOverride(t *testing.T) {
	users := usersTable()
	schema := &ir.Schema{Tables: []*ir.Table{users}}
	cfg := &config.Config{
		Overrides: map[string]config.TableOverride{
			"users": {Endpoint: "people"},
		},
	}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	routes := data.Tables[0].Routes
	for _, r := range routes {
		if r.Path != "/api/v1/people" && r.Path != "/api/v1/people/{id}" {
			t.Errorf("unexpected path %q, expected /api/v1/people or /api/v1/people/{id}", r.Path)
		}
	}
}

func TestTemplateData_FKEndpointOverride(t *testing.T) {
	users := usersTable()
	posts := postsTable(users)
	posts.ForeignKeys[0].SourceTable = posts
	schema := &ir.Schema{Tables: []*ir.Table{users, posts}}
	cfg := &config.Config{
		Overrides: map[string]config.TableOverride{
			"users": {Endpoint: "people"},
		},
	}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	// Find the nested route on posts
	postRoutes := data.Tables[1].Routes
	nested := postRoutes[5]
	if nested.Path != "/api/v1/people/{id}/posts" {
		t.Errorf("nested route path = %q, want /api/v1/people/{id}/posts", nested.Path)
	}
}

func TestTemplateData_SkipsNoPK(t *testing.T) {
	noPK := &ir.Table{
		Name: "migrations",
		Columns: []*ir.Column{
			{Name: "version", GoType: ir.GoTypeString},
		},
	}
	schema := &ir.Schema{Tables: []*ir.Table{noPK}}
	cfg := &config.Config{}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	if len(data.Tables) != 0 {
		t.Errorf("expected 0 tables for no-PK table, got %d", len(data.Tables))
	}
}

func TestTemplateData_SkipsCompositePK(t *testing.T) {
	composite := &ir.Table{
		Name: "post_tags",
		Columns: []*ir.Column{
			{Name: "post_id", GoType: ir.GoTypeInt64, IsPrimaryKey: true},
			{Name: "tag_id", GoType: ir.GoTypeInt64, IsPrimaryKey: true},
		},
		PrimaryKeys: []*ir.Column{
			{Name: "post_id", GoType: ir.GoTypeInt64, IsPrimaryKey: true},
			{Name: "tag_id", GoType: ir.GoTypeInt64, IsPrimaryKey: true},
		},
	}
	schema := &ir.Schema{Tables: []*ir.Table{composite}}
	cfg := &config.Config{}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	if len(data.Tables) != 0 {
		t.Errorf("expected 0 tables for composite PK table, got %d", len(data.Tables))
	}
}

func TestTemplateData_ExcludedTable(t *testing.T) {
	users := usersTable()
	schema := &ir.Schema{Tables: []*ir.Table{users}}
	cfg := &config.Config{
		Tables: config.TablesConfig{Exclude: []string{"users"}},
	}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	if len(data.Tables) != 0 {
		t.Errorf("expected 0 tables for excluded table, got %d", len(data.Tables))
	}
}

func TestTemplateData_CustomBasePath(t *testing.T) {
	users := usersTable()
	schema := &ir.Schema{Tables: []*ir.Table{users}}
	cfg := &config.Config{
		API: config.APIConfig{BasePath: "/v2"},
	}

	gen := newTestGenerator(t, schema, cfg)
	data := gen.templateData()

	routes := data.Tables[0].Routes
	if routes[0].Path != "/v2/users" {
		t.Errorf("expected /v2/users, got %s", routes[0].Path)
	}
}

func assertRoutes(t *testing.T, got, want []route) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("route count = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i].Method != want[i].Method {
			t.Errorf("route[%d].Method = %q, want %q", i, got[i].Method, want[i].Method)
		}
		if got[i].Path != want[i].Path {
			t.Errorf("route[%d].Path = %q, want %q", i, got[i].Path, want[i].Path)
		}
		if got[i].Handler != want[i].Handler {
			t.Errorf("route[%d].Handler = %q, want %q", i, got[i].Handler, want[i].Handler)
		}
	}
}
