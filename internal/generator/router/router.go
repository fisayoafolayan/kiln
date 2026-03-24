// Package router generates the route registration file.
package router

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/fisayoafolayan/kiln/internal/generator/genopt"
	"github.com/fisayoafolayan/kiln/internal/ir"
)

// Generator writes generated/router.go — a single file that registers
// all routes for all tables onto an http.ServeMux.
type Generator struct {
	opts genopt.Options
	tmpl *template.Template
}

// New returns a Generator ready to run.
func New(opts genopt.Options) (*Generator, error) {
	tmpl, err := template.New("router").Funcs(funcMap()).Parse(routerTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing router template: %w", err)
	}
	return &Generator{opts: opts, tmpl: tmpl}, nil
}

// Run generates the router file.
func (g *Generator) Run() ([]string, error) {
	outDir := g.opts.Config.Output.Dir
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output dir %q: %w", outDir, err)
	}

	path, err := g.writeRouter(outDir)
	if err != nil {
		return nil, err
	}
	return []string{path}, nil
}

// Diff returns the list of files that would be written without writing them.
func (g *Generator) Diff() []string {
	return []string{filepath.Join(g.opts.Config.Output.Dir, "router.go")}
}

func (g *Generator) writeRouter(outDir string) (string, error) {
	data := g.templateData()
	path := filepath.Join(outDir, "router.go")
	skipped, err := genopt.ExecuteAndWrite(g.tmpl, data, path, g.opts.Force)
	if err != nil {
		return "", err
	}
	if skipped {
		return "", nil
	}
	return path, nil
}

// route represents a single HTTP route registration.
type route struct {
	Method  string
	Path    string
	Handler string // e.g. "users.List"
}

// tableRoutes holds all routes for a single table.
type tableRoutes struct {
	Table   *ir.Table
	Handler string // handler var name e.g. "users"
	Routes  []route
}

// templateData is the data passed to the router template.
type templateData struct {
	OutputPkg  string
	ModulePath string
	ImportPath string
	BasePath   string
	Framework  string
	Tables     []tableRoutes
}

func (g *Generator) templateData() templateData {
	base := g.opts.Config.API.BasePath
	data := templateData{
		OutputPkg:  g.opts.Config.Output.Package,
		ModulePath: g.opts.ModulePath,
		ImportPath: g.opts.ImportPath,
		BasePath:   base,
		Framework:  g.opts.Config.API.Framework,
	}

	for _, t := range g.opts.Schema.Tables {
		if !g.opts.Config.ShouldGenerateTable(t.Name) {
			continue
		}
		if t.PrimaryKey == nil {
			continue
		}

		override := g.opts.Config.OverrideFor(t.Name)
		endpoint := t.Endpoint()
		if override.Endpoint != "" {
			endpoint = override.Endpoint
		}

		handlerVar := t.Name // e.g. "users"
		tr := tableRoutes{
			Table:   t,
			Handler: handlerVar,
		}

		// Standard CRUD routes
		if !override.IsOperationDisabled("list") {
			tr.Routes = append(tr.Routes, route{
				Method:  "GET",
				Path:    fmt.Sprintf("%s/%s", base, endpoint),
				Handler: fmt.Sprintf("%s.List", handlerVar),
			})
		}
		if !override.IsOperationDisabled("create") {
			tr.Routes = append(tr.Routes, route{
				Method:  "POST",
				Path:    fmt.Sprintf("%s/%s", base, endpoint),
				Handler: fmt.Sprintf("%s.Create", handlerVar),
			})
		}
		if !override.IsOperationDisabled("get") {
			tr.Routes = append(tr.Routes, route{
				Method:  "GET",
				Path:    fmt.Sprintf("%s/%s/{id}", base, endpoint),
				Handler: fmt.Sprintf("%s.Get", handlerVar),
			})
		}
		if !override.IsOperationDisabled("update") {
			tr.Routes = append(tr.Routes, route{
				Method:  "PATCH",
				Path:    fmt.Sprintf("%s/%s/{id}", base, endpoint),
				Handler: fmt.Sprintf("%s.Update", handlerVar),
			})
		}
		if !override.IsOperationDisabled("delete") {
			tr.Routes = append(tr.Routes, route{
				Method:  "DELETE",
				Path:    fmt.Sprintf("%s/%s/{id}", base, endpoint),
				Handler: fmt.Sprintf("%s.Delete", handlerVar),
			})
		}

		// Nested routes from FK relationships
		// e.g. posts.user_id → users.id generates GET /users/{id}/posts
		for _, fk := range t.ForeignKeys {
			parentTable := fk.TargetTable
			parentOverride := g.opts.Config.OverrideFor(parentTable.Name)
			parentEndpoint := parentTable.Endpoint()
			if parentOverride.Endpoint != "" {
				parentEndpoint = parentOverride.Endpoint
			}
			tr.Routes = append(tr.Routes, route{
				Method:  "GET",
				Path:    fmt.Sprintf("%s/%s/{id}/%s", base, parentEndpoint, endpoint),
				Handler: fmt.Sprintf("%s.ListBy%s", handlerVar, parentTable.GoName()),
			})
		}

		data.Tables = append(data.Tables, tr)
	}

	return data
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"isStdlib": func(framework string) bool {
			return framework == "stdlib" || framework == ""
		},
		"isChi": func(framework string) bool {
			return framework == "chi"
		},
		"isGin": func(framework string) bool {
			return framework == "gin"
		},
		// chiMethod converts "GET" to "Get", "POST" to "Post", etc.
		"chiMethod": func(method string) string {
			if len(method) == 0 {
				return method
			}
			return strings.ToUpper(method[:1]) + strings.ToLower(method[1:])
		},
	}
}

// ---------------------------------------------------------------------------
// Router template
// ---------------------------------------------------------------------------

const routerTemplate = `// Code generated by kiln. DO NOT EDIT.
// Re-generated on each run. Use endpoint/disable overrides in kiln.yaml to customise.
// kiln:checksum=__CHECKSUM__

package {{.OutputPkg}}

import (
{{if isStdlib .Framework}}	"net/http"
{{end}}{{if isChi .Framework}}	"github.com/go-chi/chi/v5"
{{end}}{{if isGin .Framework}}	"github.com/gin-gonic/gin"
{{end}}
	"{{.ImportPath}}/handlers"
)

{{range .Tables}}// {{.Table.GoName}}Handler is the handler type for {{.Table.Name}}.
// Declared here so callers can inject their own implementation.
type {{.Table.GoName}}HandlerIface interface {
{{range .Routes}}	// {{.Method}} {{.Path}}
{{end}}}
{{end}}

{{if isStdlib .Framework}}
// RegisterRoutes registers all generated routes onto mux.
// Call this once during application startup.
//
// Example:
//
//	mux := http.NewServeMux()
//	RegisterRoutes(mux,
{{range .Tables}}//	    handlers.New{{.Table.GoName}}Handler(store.New{{.Table.GoName}}Store(db)),
{{end}}//	)
func RegisterRoutes(
	mux *http.ServeMux,
{{range .Tables}}	{{.Handler}} *handlers.{{.Table.GoName}}Handler,
{{end}}) {
{{range .Tables}}	// {{.Table.Name}}
{{range .Routes}}	mux.HandleFunc("{{.Method}} {{.Path}}", {{.Handler}})
{{end}}
{{end}}}
{{end}}

{{if isChi .Framework}}
// RegisterRoutes registers all generated routes onto a chi router.
func RegisterRoutes(
	r chi.Router,
{{range .Tables}}	{{.Handler}} *handlers.{{.Table.GoName}}Handler,
{{end}}) {
{{range .Tables}}	// {{.Table.Name}}
{{range .Routes}}	r.{{chiMethod .Method}}("{{.Path}}", {{.Handler}})
{{end}}
{{end}}}
{{end}}

{{if isGin .Framework}}
// RegisterRoutes registers all generated routes onto a gin engine.
func RegisterRoutes(
	r *gin.Engine,
{{range .Tables}}	{{.Handler}} *handlers.{{.Table.GoName}}Handler,
{{end}}) {
{{range .Tables}}	// {{.Table.Name}}
{{range .Routes}}	r.{{.Method}}("{{.Path}}", gin.WrapF({{.Handler}}))
{{end}}
{{end}}}
{{end}}
`
