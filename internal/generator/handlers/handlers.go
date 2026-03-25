// Package handlers generates HTTP handlers for each table.
package handlers

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator/genopt"
	"github.com/fisayoafolayan/kiln/internal/ir"
)

// Generator writes generated/handlers/<table>.go for each table.
type Generator struct {
	opts genopt.Options
	tmpl *template.Template
}

// New returns a Generator ready to run.
func New(opts genopt.Options) (*Generator, error) {
	tmpl, err := template.New("handlers").Funcs(funcMap()).Parse(handlerTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing handler template: %w", err)
	}
	return &Generator{opts: opts, tmpl: tmpl}, nil
}

// Run generates a handler file for every table in the schema.
func (g *Generator) Run() ([]string, error) {
	outDir := filepath.Join(g.opts.Config.Output.Dir, "handlers")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output dir %q: %w", outDir, err)
	}

	var written []string
	for _, t := range g.opts.Schema.Tables {
		if !g.opts.Config.ShouldGenerateTable(t.Name) {
			continue
		}
		if t.PrimaryKey == nil {
			continue
		}
		path, err := g.writeTable(t, outDir)
		if err != nil {
			return nil, fmt.Errorf("generating handler for %q: %w", t.Name, err)
		}
		written = append(written, path)
	}

	// Write shared helper file — write-once
	helpersPath, skipped, err := g.writeHelpers(outDir)
	if err != nil {
		return nil, err
	}
	if !skipped {
		written = append(written, helpersPath)
	}

	return written, nil
}

// Diff returns the list of files that would be written without writing them.
func (g *Generator) Diff() []string {
	outDir := filepath.Join(g.opts.Config.Output.Dir, "handlers")
	var files []string
	for _, t := range g.opts.Schema.Tables {
		if !g.opts.Config.ShouldGenerateTable(t.Name) {
			continue
		}
		files = append(files, filepath.Join(outDir, t.Name+".go"))
	}
	helpersPath := filepath.Join(outDir, "helpers.go")
	if _, err := os.Stat(helpersPath); os.IsNotExist(err) {
		files = append(files, helpersPath+" (write-once)")
	}
	return files
}

func (g *Generator) writeTable(t *ir.Table, outDir string) (string, error) {
	data := g.templateData(t)
	path := filepath.Join(outDir, t.Name+".go")
	skipped, err := genopt.ExecuteAndWrite(g.tmpl, data, path, g.opts.Force)
	if err != nil {
		return "", err
	}
	if skipped {
		return "", nil
	}
	return path, nil
}

func (g *Generator) writeHelpers(outDir string) (string, bool, error) {
	path := filepath.Join(outDir, "helpers.go")
	if _, err := os.Stat(path); err == nil {
		return path, true, nil // write-once
	}
	formatted, err := format.Source([]byte(helpersTemplate))
	if err != nil {
		formatted = []byte(helpersTemplate)
	}
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return "", false, fmt.Errorf("writing helpers: %w", err)
	}
	return path, false, nil
}

// templateData is the data passed to the handler template.
type templateData struct {
	ModulePath     string
	ImportPath     string
	OutputPkg      string
	Table          *ir.Table
	Override       config.TableOverride
	ForeignKeys    []*ir.ForeignKey // all FK relationships for this table
	FilterableCols []*ir.Column
	SortableCols   []*ir.Column
}

func (g *Generator) templateData(t *ir.Table) templateData {
	override := g.opts.Config.OverrideFor(t.Name)

	var filterable, sortable []*ir.Column
	for _, c := range t.Columns {
		if override.IsFieldFilterable(c.Name) && c.GoType.IsFilterable() && !c.IsSoftDeleteColumn() {
			filterable = append(filterable, c)
		}
		if override.IsFieldSortable(c.Name) && c.GoType.IsFilterable() && !c.IsSoftDeleteColumn() {
			sortable = append(sortable, c)
		}
	}

	return templateData{
		ModulePath:     g.opts.ModulePath,
		ImportPath:     g.opts.ImportPath,
		OutputPkg:      g.opts.Config.Output.Package,
		Table:          t,
		Override:       override,
		ForeignKeys:    t.ForeignKeys,
		FilterableCols: filterable,
		SortableCols:   sortable,
	}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"isOperationEnabled": func(op string, o config.TableOverride) bool {
			return !o.IsOperationDisabled(op)
		},
		"pkIsStringLike": func(t *ir.Table) bool {
			return t.PKIsStringLike()
		},
		"needsRangeOps": func(gt ir.GoType) bool {
			return gt.SupportsRangeOps()
		},
		"filterNeedsStrconv": func(cols []*ir.Column) bool {
			for _, c := range cols {
				switch c.GoType.Name {
				case "int32", "int64", "float64", "bool":
					return true
				}
			}
			return false
		},
		"filterNeedsTime": func(cols []*ir.Column) bool {
			for _, c := range cols {
				if c.GoType.Name == "time.Time" {
					return true
				}
			}
			return false
		},
		"filterNeedsUUID": func(cols []*ir.Column) bool {
			for _, c := range cols {
				if c.GoType.Name == "uuid.UUID" {
					return true
				}
			}
			return false
		},
		"pkIsUUID": func(t *ir.Table) bool {
			return t.PKTypeName() == "uuid.UUID"
		},
		"handlerNeedsUUID": func(t *ir.Table, filterCols []*ir.Column, fks []*ir.ForeignKey) bool {
			if t.PKTypeName() == "uuid.UUID" {
				return true
			}
			for _, c := range filterCols {
				if c.GoType.Name == "uuid.UUID" {
					return true
				}
			}
			for _, fk := range fks {
				if fk.TargetTable.PKTypeName() == "uuid.UUID" {
					return true
				}
			}
			return false
		},
		// filterParseSnippet returns the Go code to parse a query param value
		// into a pointer of the column's Go type and assign it to a target variable.
		"filterParseSnippet": func(c *ir.Column, target string) string {
			switch c.GoType.Name {
			case "string":
				return "{ val := v; " + target + " = &val }"
			case "uuid.UUID":
				return "if parsed, err := uuid.FromString(v); err == nil { " + target + " = &parsed }"
			case "int32":
				return "if n, err := strconv.ParseInt(v, 10, 32); err == nil { val := int32(n); " + target + " = &val }"
			case "int64":
				return "if n, err := strconv.ParseInt(v, 10, 64); err == nil { " + target + " = &n }"
			case "float64":
				return "if n, err := strconv.ParseFloat(v, 64); err == nil { " + target + " = &n }"
			case "bool":
				return "if b, err := strconv.ParseBool(v); err == nil { " + target + " = &b }"
			case "time.Time":
				return "if t, err := time.Parse(time.RFC3339, v); err == nil { " + target + " = &t }"
			default:
				return "// unsupported filter type: " + c.GoType.Name
			}
		},
	}
}

// ---------------------------------------------------------------------------
// Handler template
// ---------------------------------------------------------------------------

const handlerTemplate = `// Code generated by kiln. DO NOT EDIT.
// Re-generated on each run. Customise helpers.go or disable operations in kiln.yaml.
// kiln:table={{.Table.Name}} kiln:checksum=__CHECKSUM__

package handlers

import (
	"context"
	"net/http"
{{if or (filterNeedsStrconv .FilterableCols) (not (pkIsStringLike .Table))}}	"strconv"
{{end}}{{if .SortableCols}}	"strings"
{{end}}{{if filterNeedsTime .FilterableCols}}	"time"
{{end}}
{{if handlerNeedsUUID .Table .FilterableCols .ForeignKeys}}	"github.com/gofrs/uuid/v5"
{{end}}	"{{.ImportPath}}/store"
	"{{.ImportPath}}/models"
)

// {{.Table.GoName}}Store is the interface the handler depends on.
type {{.Table.GoName}}Store interface {
{{if isOperationEnabled "get" .Override}}	Get(ctx context.Context, id {{.Table.PKTypeName}}) (*models.{{.Table.GoName}}, error)
{{end}}{{if isOperationEnabled "list" .Override}}	List(ctx context.Context, page, pageSize int, filter store.{{.Table.GoName}}ListFilter) ([]models.{{.Table.GoName}}, int, error)
{{end}}{{if isOperationEnabled "create" .Override}}	Create(ctx context.Context, req models.Create{{.Table.GoName}}Request) (*models.{{.Table.GoName}}, error)
{{end}}{{if isOperationEnabled "update" .Override}}	Update(ctx context.Context, id {{.Table.PKTypeName}}, req models.Update{{.Table.GoName}}Request) (*models.{{.Table.GoName}}, error)
{{end}}{{if isOperationEnabled "delete" .Override}}	Delete(ctx context.Context, id {{.Table.PKTypeName}}) error
{{end}}{{range .ForeignKeys}}	ListBy{{.TargetTable.GoName}}(ctx context.Context, parentID {{.TargetTable.PKTypeName}}, page, pageSize int) ([]models.{{$.Table.GoName}}, int, error)
{{end}}}

// {{.Table.GoName}}Handler handles HTTP requests for {{.Table.Name}}.
type {{.Table.GoName}}Handler struct {
	store {{.Table.GoName}}Store
}

// New{{.Table.GoName}}Handler returns a new {{.Table.GoName}}Handler.
func New{{.Table.GoName}}Handler(store {{.Table.GoName}}Store) *{{.Table.GoName}}Handler {
	return &{{.Table.GoName}}Handler{store: store}
}

{{if isOperationEnabled "get" .Override}}
// Get handles GET /{{.Table.Endpoint}}/{id}
func (h *{{.Table.GoName}}Handler) Get(w http.ResponseWriter, r *http.Request) {
	{{if pkIsUUID .Table}}idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	id, err := uuid.FromString(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}{{else if pkIsStringLike .Table}}id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}{{else}}idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}{{end}}
	row, err := h.store.Get(r.Context(), {{if pkIsUUID .Table}}id{{else if not (pkIsStringLike .Table)}}{{.Table.PKTypeName}}(id){{else}}id{{end}})
	if err != nil {
		handleStoreError(w, err, "{{.Table.Name}}", "get")
		return
	}
	writeJSON(w, http.StatusOK, row)
}
{{end}}

{{if isOperationEnabled "list" .Override}}
// List handles GET /{{.Table.Endpoint}}
func (h *{{.Table.GoName}}Handler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	q := r.URL.Query()
	filter := store.{{.Table.GoName}}ListFilter{}

{{range .FilterableCols}}	if v := q.Get("{{.Name}}"); v != "" {
		{{filterParseSnippet . (printf "filter.%s" .GoName)}}
	}
	if v := q.Get("{{.Name}}[neq]"); v != "" {
		{{filterParseSnippet . (printf "filter.%sNeq" .GoName)}}
	}
{{if needsRangeOps .GoType}}	if v := q.Get("{{.Name}}[gt]"); v != "" {
		{{filterParseSnippet . (printf "filter.%sGt" .GoName)}}
	}
	if v := q.Get("{{.Name}}[gte]"); v != "" {
		{{filterParseSnippet . (printf "filter.%sGte" .GoName)}}
	}
	if v := q.Get("{{.Name}}[lt]"); v != "" {
		{{filterParseSnippet . (printf "filter.%sLt" .GoName)}}
	}
	if v := q.Get("{{.Name}}[lte]"); v != "" {
		{{filterParseSnippet . (printf "filter.%sLte" .GoName)}}
	}
{{end}}{{end}}
{{if .SortableCols}}	if sortParam := q.Get("sort"); sortParam != "" {
		if strings.HasPrefix(sortParam, "-") {
			filter.SortBy = sortParam[1:]
			filter.SortDesc = true
		} else {
			filter.SortBy = sortParam
		}
	}
{{end}}
	rows, total, err := h.store.List(r.Context(), page, pageSize, filter)
	if err != nil {
		handleStoreError(w, err, "{{.Table.Name}}", "list")
		return
	}
	writeJSON(w, http.StatusOK, models.List{{.Table.GoNamePlural}}Response{
		Data:     rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
{{end}}

{{if isOperationEnabled "create" .Override}}
// Create handles POST /{{.Table.Endpoint}}
func (h *{{.Table.GoName}}Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.Create{{.Table.GoName}}Request
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validateRequest(w, req) {
		return
	}
	row, err := h.store.Create(r.Context(), req)
	if err != nil {
		handleStoreError(w, err, "{{.Table.Name}}", "create")
		return
	}
	writeJSON(w, http.StatusCreated, row)
}
{{end}}

{{if isOperationEnabled "update" .Override}}
// Update handles PATCH /{{.Table.Endpoint}}/{id}
func (h *{{.Table.GoName}}Handler) Update(w http.ResponseWriter, r *http.Request) {
	{{if pkIsUUID .Table}}idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	id, err := uuid.FromString(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}{{else if pkIsStringLike .Table}}id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}{{else}}idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}{{end}}
	var req models.Update{{.Table.GoName}}Request
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validateRequest(w, req) {
		return
	}
	row, err {{if pkIsStringLike .Table}}:{{end}}= h.store.Update(r.Context(), {{if pkIsUUID .Table}}id{{else if not (pkIsStringLike .Table)}}{{.Table.PKTypeName}}(id){{else}}id{{end}}, req)
	if err != nil {
		handleStoreError(w, err, "{{.Table.Name}}", "update")
		return
	}
	writeJSON(w, http.StatusOK, row)
}
{{end}}

{{if isOperationEnabled "delete" .Override}}
// Delete handles DELETE /{{.Table.Endpoint}}/{id}
func (h *{{.Table.GoName}}Handler) Delete(w http.ResponseWriter, r *http.Request) {
	{{if pkIsUUID .Table}}idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	id, err := uuid.FromString(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}{{else if pkIsStringLike .Table}}id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}{{else}}idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}{{end}}
	if err := h.store.Delete(r.Context(), {{if pkIsUUID .Table}}id{{else if not (pkIsStringLike .Table)}}{{.Table.PKTypeName}}(id){{else}}id{{end}}); err != nil {
		handleStoreError(w, err, "{{.Table.Name}}", "delete")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
{{end}}

{{range .ForeignKeys}}
// ListBy{{.TargetTable.GoName}} handles GET /{{.TargetTable.Endpoint}}/{id}/{{$.Table.Endpoint}}
func (h *{{$.Table.GoName}}Handler) ListBy{{.TargetTable.GoName}}(w http.ResponseWriter, r *http.Request) {
	{{if .TargetTable.PKIsUUID}}parentIDStr := r.PathValue("id")
	if parentIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	parentID, err := uuid.FromString(parentIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}{{else if .TargetTable.PKIsStringLike}}parentID := r.PathValue("id")
	if parentID == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}{{else}}parentIDStr := r.PathValue("id")
	if parentIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	parentIDVal, err := strconv.ParseInt(parentIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	parentID := {{.TargetTable.PKTypeName}}(parentIDVal){{end}}
	page, pageSize := parsePagination(r)
	rows, total, err := h.store.ListBy{{.TargetTable.GoName}}(r.Context(), parentID, page, pageSize)
	if err != nil {
		handleStoreError(w, err, "{{$.Table.Name}}", "list")
		return
	}
	writeJSON(w, http.StatusOK, models.List{{$.Table.GoNamePlural}}Response{
		Data:     rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
{{end}}
`

// ---------------------------------------------------------------------------
// Helpers template — written once, yours to customise
// ---------------------------------------------------------------------------

const helpersTemplate = `// Generated by kiln on first run. THIS FILE IS YOURS.
// Customise error formats, add auth helpers, adjust pagination defaults etc.

package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// validate is a shared validator instance. It is safe for concurrent use.
var (
	validate     *validator.Validate
	validateOnce sync.Once
)

func getValidator() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New()
		// Use JSON tag names in validation error messages
		// so errors reference "email" not "Email"
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	})
	return validate
}

type errorResponse struct {
	Error  string            ` + "`" + `json:"error"` + "`" + `
	Fields map[string]string ` + "`" + `json:"fields,omitempty"` + "`" + `
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// maxRequestBodySize is the maximum size of a request body (1MB).
// Adjust this to your needs.
const maxRequestBodySize = 1 << 20

// decodeJSON reads and decodes a JSON request body with a size limit.
// Returns false and writes an error response if decoding fails.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		if err.Error() == "http: request body too large" {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return false
	}
	return true
}

// handleStoreError maps common store/database errors to HTTP status codes.
// Customise this function to add application-specific error handling.
func handleStoreError(w http.ResponseWriter, err error, entity, operation string) {
	if err == nil {
		return
	}

	// Not found — bob/sql returns sql.ErrNoRows from .One()
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, entity+" not found")
		return
	}

	// Constraint violations - detect via error codes (locale-independent).
	// Drivers expose codes through interfaces; we check those first,
	// then fall back to string matching for compatibility.
	code := dbErrorCode(err)
	switch {
	case code == "23505" || code == "1062": // Postgres / MySQL unique violation
		writeError(w, http.StatusConflict, entity+" already exists")
		return
	case code == "23503" || code == "1452": // Postgres / MySQL FK violation
		writeError(w, http.StatusUnprocessableEntity, "referenced "+entity+" does not exist")
		return
	}

	// Fallback: string matching for SQLite and untyped errors.
	errMsg := err.Error()
	if strings.Contains(errMsg, "UNIQUE constraint") {
		writeError(w, http.StatusConflict, entity+" already exists")
		return
	}
	if strings.Contains(errMsg, "FOREIGN KEY constraint") {
		writeError(w, http.StatusUnprocessableEntity, "referenced "+entity+" does not exist")
		return
	}

	// Default: internal server error. Log the actual error for debugging.
	log.Printf("ERROR %s.%s: %v", entity, operation, err)
	writeError(w, http.StatusInternalServerError, "failed to "+operation+" "+entity)
}

// dbErrorCode extracts a database error code without importing driver packages.
// Postgres (pgx): implements SQLState() string → "23505", "23503", etc.
// MySQL (go-sql-driver): implements Number() uint16 → 1062, 1452, etc.
func dbErrorCode(err error) string {
	// Postgres: pgconn.PgError has Code/SQLState method.
	type pgErr interface{ SQLState() string }
	var pg pgErr
	if errors.As(err, &pg) {
		return pg.SQLState()
	}
	// MySQL: *mysql.MySQLError has Number field.
	type myErr interface{ Error() string; Is(error) bool }
	// MySQL driver doesn't have a clean code accessor, so extract from Error() format.
	// Format: "Error 1062 (23000): Duplicate entry ..."
	s := err.Error()
	if len(s) > 6 && s[:6] == "Error " {
		if idx := strings.IndexByte(s[6:], ' '); idx > 0 {
			return s[6 : 6+idx]
		}
	}
	return ""
}

func parsePagination(r *http.Request) (page, pageSize int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// validateStruct runs struct validation and returns a structured error
// with per-field messages if validation fails.
func validateRequest(w http.ResponseWriter, v any) bool {
	if err := getValidator().Struct(v); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			fields := make(map[string]string, len(ve))
			for _, fe := range ve {
				fields[fe.Field()] = validationMessage(fe)
			}
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
				Error:  "validation failed",
				Fields: fields,
			})
			return false
		}
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return false
	}
	return true
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	default:
		return fmt.Sprintf("failed %s validation", fe.Tag())
	}
}

func extractID(r *http.Request, key string) (string, error) {
	v := r.PathValue(key)
	if v == "" {
		return "", fmt.Errorf("missing path parameter %q", key)
	}
	return v, nil
}
`
