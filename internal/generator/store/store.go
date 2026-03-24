// Package store generates the bob-powered Store implementation for each table.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator/genopt"
	"github.com/fisayoafolayan/kiln/internal/ir"
)

// Generator writes:
//
//	generated/store/<table>.go         — bob-powered Store impl (always regenerated)
//	generated/store/mappers/<table>.go — type mapper (write-once, never overwritten)
type Generator struct {
	opts       genopt.Options
	storeTmpl  *template.Template
	mapperTmpl *template.Template
}

// New returns a Generator ready to run.
func New(opts genopt.Options) (*Generator, error) {
	storeTmpl, err := template.New("store").Funcs(funcMap()).Parse(storeTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing store template: %w", err)
	}
	mapperTmpl, err := template.New("mapper").Funcs(funcMap()).Parse(mapperTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing mapper template: %w", err)
	}
	return &Generator{
		opts:       opts,
		storeTmpl:  storeTmpl,
		mapperTmpl: mapperTmpl,
	}, nil
}

// Run generates store and mapper files for every table.
func (g *Generator) Run() ([]string, error) {
	storeDir := filepath.Join(g.opts.Config.Output.Dir, "store")
	mapperDir := filepath.Join(storeDir, "mappers")

	for _, dir := range []string{storeDir, mapperDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating dir %q: %w", dir, err)
		}
	}

	var written []string
	for _, t := range g.opts.Schema.Tables {
		if !g.opts.Config.ShouldGenerateTable(t.Name) {
			continue
		}
		if t.PrimaryKey == nil {
			continue // skip composite PK tables
		}
		storePath, err := g.writeStore(t, storeDir)
		if err != nil {
			return nil, fmt.Errorf("generating store for %q: %w", t.Name, err)
		}
		written = append(written, storePath)

		mapperPath, skipped, err := g.writeMapper(t, mapperDir)
		if err != nil {
			return nil, fmt.Errorf("generating mapper for %q: %w", t.Name, err)
		}
		if !skipped {
			written = append(written, mapperPath)
		}
	}
	return written, nil
}

// Diff returns the list of files that would be written without writing them.
func (g *Generator) Diff() []string {
	storeDir := filepath.Join(g.opts.Config.Output.Dir, "store")
	mapperDir := filepath.Join(storeDir, "mappers")

	var files []string
	for _, t := range g.opts.Schema.Tables {
		if !g.opts.Config.ShouldGenerateTable(t.Name) {
			continue
		}
		files = append(files, filepath.Join(storeDir, t.Name+".go"))
		mapperPath := filepath.Join(mapperDir, t.Name+".go")
		if _, err := os.Stat(mapperPath); os.IsNotExist(err) {
			files = append(files, mapperPath+" (write-once)")
		}
	}
	return files
}

func (g *Generator) writeStore(t *ir.Table, outDir string) (string, error) {
	data := g.templateData(t)
	return g.writeFile(g.storeTmpl, data, filepath.Join(outDir, t.Name+".go"))
}

func (g *Generator) writeMapper(t *ir.Table, outDir string) (string, bool, error) {
	path := filepath.Join(outDir, t.Name+".go")
	if _, err := os.Stat(path); err == nil {
		return path, true, nil
	}
	data := g.templateData(t)
	written, err := g.writeFile(g.mapperTmpl, data, path)
	return written, false, err
}

func (g *Generator) writeFile(tmpl *template.Template, data templateData, path string) (string, error) {
	skipped, err := genopt.ExecuteAndWrite(tmpl, data, path, g.opts.Force)
	if err != nil {
		return "", err
	}
	if skipped {
		return "", nil
	}
	return path, nil
}

// templateData is the data passed to the store and mapper templates.
type templateData struct {
	ModulePath     string
	ImportPath     string
	ModelsPath     string
	OutputPkg      string
	SMImport       string
	BobPkg         string
	DialectImport  string
	NeedsClientID  bool // true for MySQL/SQLite which lack RETURNING
	Table          *ir.Table
	Override       config.TableOverride
	WritableCols   []*ir.Column
	VisibleCols    []*ir.Column
	FilterableCols []*ir.Column
	SortableCols   []*ir.Column
}

func (g *Generator) templateData(t *ir.Table) templateData {
	override := g.opts.Config.OverrideFor(t.Name)

	modelsDir := strings.TrimPrefix(filepath.ToSlash(g.opts.Config.Bob.ModelsDir), "./")
	modelsPath := g.opts.ModulePath + "/" + modelsDir

	var writable, visible, filterable, sortable []*ir.Column
	for _, c := range t.Columns {
		if !override.IsFieldHidden(c.Name) {
			visible = append(visible, c)
		}
		if !c.IsReadOnly() && !override.IsFieldHidden(c.Name) && !override.IsFieldReadonly(c.Name) {
			writable = append(writable, c)
		}
		if override.IsFieldFilterable(c.Name) && c.GoType.IsFilterable() {
			filterable = append(filterable, c)
		}
		if override.IsFieldSortable(c.Name) && c.GoType.IsFilterable() {
			sortable = append(sortable, c)
		}
	}

	return templateData{
		ModulePath:     g.opts.ModulePath,
		ImportPath:     g.opts.ImportPath,
		ModelsPath:     modelsPath,
		OutputPkg:      g.opts.Config.Output.Package,
		SMImport:       g.opts.Dialect.SMImport,
		BobPkg:         g.opts.Dialect.BobPkg,
		DialectImport:  g.opts.Dialect.DialectImport,
		NeedsClientID:  g.opts.Config.Database.Driver != "postgres",
		Table:          t,
		Override:       override,
		WritableCols:   writable,
		VisibleCols:    visible,
		FilterableCols: filterable,
		SortableCols:   sortable,
	}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"isOperationEnabled": func(op string, o config.TableOverride) bool {
			return !o.IsOperationDisabled(op)
		},
		"needsRangeOps": func(gt ir.GoType) bool {
			return gt.SupportsRangeOps()
		},
		"hasNullableWritable": func(cols []*ir.Column) bool {
			for _, c := range cols {
				if c.Nullable {
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
	}
}

// ---------------------------------------------------------------------------
// Store template
// ---------------------------------------------------------------------------

const storeTemplate = `// Code generated by kiln. DO NOT EDIT.
// Re-generated on each run. Customise the mapper in store/mappers/ instead.
// kiln:table={{.Table.Name}} kiln:checksum=__CHECKSUM__

package store

import (
	"context"
	"fmt"
{{if filterNeedsTime .FilterableCols}}	"time"
{{end}}
	"github.com/aarondl/opt/omit"
	{{if hasNullableWritable .WritableCols}}"github.com/aarondl/opt/omitnull"
	{{end}}{{if filterNeedsUUID .FilterableCols}}"github.com/gofrs/uuid/v5"
	{{end}}{{if .NeedsClientID}}"github.com/google/uuid"
	{{end}}"github.com/stephenafamo/bob"
	"{{.DialectImport}}"
	"{{.DialectImport}}/dialect"
	"{{.SMImport}}"
	dbmodels "{{.ModelsPath}}"
	"{{.ImportPath}}/models"
	"{{.ImportPath}}/store/mappers"
)

// {{.Table.GoName}}Store handles database operations for {{.Table.Name}}.
type {{.Table.GoName}}Store struct {
	db bob.DB
}

// New{{.Table.GoName}}Store returns a new {{.Table.GoName}}Store.
func New{{.Table.GoName}}Store(db bob.DB) *{{.Table.GoName}}Store {
	return &{{.Table.GoName}}Store{db: db}
}

{{if isOperationEnabled "get" .Override}}
// Get retrieves a single {{.Table.GoName}} by primary key.
func (s *{{.Table.GoName}}Store) Get(ctx context.Context, id {{.Table.PKTypeName}}) (*models.{{.Table.GoName}}, error) {
	row, err := dbmodels.{{.Table.GoNamePlural}}.Query(
		sm.Where(dbmodels.{{.Table.GoNamePlural}}.Columns.{{.Table.PrimaryKey.GoName}}.EQ({{.BobPkg}}.Arg(id))),
	).One(ctx, s.db)
	if err != nil {
		return nil, fmt.Errorf("{{.Table.Name}}.Get: %w", err)
	}
	return mappers.{{.Table.GoName}}ToType(row), nil
}
{{end}}

{{if isOperationEnabled "list" .Override}}
// {{.Table.GoName}}ListFilter holds optional filter and sort parameters.
type {{.Table.GoName}}ListFilter struct {
{{range .FilterableCols}}	{{.GoName}}    *{{.GoType.Name}}
	{{.GoName}}Neq *{{.GoType.Name}}
{{if needsRangeOps .GoType}}	{{.GoName}}Gt  *{{.GoType.Name}}
	{{.GoName}}Gte *{{.GoType.Name}}
	{{.GoName}}Lt  *{{.GoType.Name}}
	{{.GoName}}Lte *{{.GoType.Name}}
{{end}}{{end}}	SortBy   string
	SortDesc bool
}

// List retrieves a paginated, filtered list of {{.Table.GoNamePlural}}.
func (s *{{.Table.GoName}}Store) List(ctx context.Context, page, pageSize int, filter {{.Table.GoName}}ListFilter) ([]models.{{.Table.GoName}}, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }

	// Build WHERE clauses from filter.
	var whereMods []bob.Mod[*dialect.SelectQuery]
{{range .FilterableCols}}	if filter.{{.GoName}} != nil {
		whereMods = append(whereMods, sm.Where(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.GoName}}.EQ({{$.BobPkg}}.Arg(*filter.{{.GoName}}))))
	}
	if filter.{{.GoName}}Neq != nil {
		whereMods = append(whereMods, sm.Where(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.GoName}}.NE({{$.BobPkg}}.Arg(*filter.{{.GoName}}Neq))))
	}
{{if needsRangeOps .GoType}}	if filter.{{.GoName}}Gt != nil {
		whereMods = append(whereMods, sm.Where(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.GoName}}.GT({{$.BobPkg}}.Arg(*filter.{{.GoName}}Gt))))
	}
	if filter.{{.GoName}}Gte != nil {
		whereMods = append(whereMods, sm.Where(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.GoName}}.GTE({{$.BobPkg}}.Arg(*filter.{{.GoName}}Gte))))
	}
	if filter.{{.GoName}}Lt != nil {
		whereMods = append(whereMods, sm.Where(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.GoName}}.LT({{$.BobPkg}}.Arg(*filter.{{.GoName}}Lt))))
	}
	if filter.{{.GoName}}Lte != nil {
		whereMods = append(whereMods, sm.Where(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.GoName}}.LTE({{$.BobPkg}}.Arg(*filter.{{.GoName}}Lte))))
	}
{{end}}{{end}}
	// Build query mods: filters + pagination.
	queryMods := append(whereMods,
		sm.Limit(int64(pageSize)),
		sm.Offset(int64((page-1)*pageSize)),
	)

	// Sorting.
{{if .SortableCols}}	switch filter.SortBy {
{{range .SortableCols}}	case "{{.Name}}":
		if filter.SortDesc {
			queryMods = append(queryMods, sm.OrderBy(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.GoName}}).Desc())
		} else {
			queryMods = append(queryMods, sm.OrderBy(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.GoName}}).Asc())
		}
{{end}}	}
{{end}}
	rows, err := dbmodels.{{.Table.GoNamePlural}}.Query(queryMods...).All(ctx, s.db)
	if err != nil {
		return nil, 0, fmt.Errorf("{{.Table.Name}}.List: %w", err)
	}
	// Count uses only WHERE clauses, not pagination/sort.
	count, err := dbmodels.{{.Table.GoNamePlural}}.Query(whereMods...).Count(ctx, s.db)
	if err != nil {
		return nil, 0, fmt.Errorf("{{.Table.Name}}.List count: %w", err)
	}
	return mappers.{{.Table.GoNamePlural}}ToTypes(rows), int(count), nil
}
{{end}}

{{range .Table.ForeignKeys}}
// ListBy{{.TargetTable.GoName}} retrieves {{$.Table.GoNamePlural}} belonging to a {{.TargetTable.GoName}}.
func (s *{{$.Table.GoName}}Store) ListBy{{.TargetTable.GoName}}(ctx context.Context, parentID {{.TargetTable.PKTypeName}}, page, pageSize int) ([]models.{{$.Table.GoName}}, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }
	rows, err := dbmodels.{{$.Table.GoNamePlural}}.Query(
		sm.Where(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.SourceColumn.GoName}}.EQ({{$.BobPkg}}.Arg(parentID))),
		sm.Limit(int64(pageSize)),
		sm.Offset(int64((page-1)*pageSize)),
	).All(ctx, s.db)
	if err != nil {
		return nil, 0, fmt.Errorf("{{$.Table.Name}}.ListBy{{.TargetTable.GoName}}: %w", err)
	}
	count, err := dbmodels.{{$.Table.GoNamePlural}}.Query(
		sm.Where(dbmodels.{{$.Table.GoNamePlural}}.Columns.{{.SourceColumn.GoName}}.EQ({{$.BobPkg}}.Arg(parentID))),
	).Count(ctx, s.db)
	if err != nil {
		return nil, 0, fmt.Errorf("{{$.Table.Name}}.ListBy{{.TargetTable.GoName}} count: %w", err)
	}
	return mappers.{{$.Table.GoNamePlural}}ToTypes(rows), int(count), nil
}
{{end}}

{{if isOperationEnabled "create" .Override}}
// Create inserts a new {{.Table.GoName}} record.
func (s *{{.Table.GoName}}Store) Create(ctx context.Context, req types.Create{{.Table.GoName}}Request) (*models.{{.Table.GoName}}, error) {
	{{if .NeedsClientID}}// Generate ID in Go — required for MySQL/SQLite which lack RETURNING support.
	newID := uuid.New().String()
	setter := &dbmodels.{{.Table.GoName}}Setter{
		{{.Table.PrimaryKey.GoName}}: omit.From(newID),
		{{range .WritableCols}}{{if .Nullable}}{{.GoName}}: omitnull.FromPtr(req.{{.GoName}}),
		{{else}}{{.GoName}}: omit.From(req.{{.GoName}}),
		{{end}}{{end}}
	}
	_, err := dbmodels.{{.Table.GoNamePlural}}.Insert(setter).Exec(ctx, s.db)
	if err != nil {
		return nil, fmt.Errorf("{{.Table.Name}}.Create: %w", err)
	}
	row, err := dbmodels.{{.Table.GoNamePlural}}.Query(
		sm.Where(dbmodels.{{.Table.GoNamePlural}}.Columns.{{.Table.PrimaryKey.GoName}}.EQ({{.BobPkg}}.Arg(newID))),
	).One(ctx, s.db)
	if err != nil {
		return nil, fmt.Errorf("{{.Table.Name}}.Create fetch: %w", err)
	}
	return mappers.{{.Table.GoName}}ToType(row), nil
	{{else}}setter := &dbmodels.{{.Table.GoName}}Setter{
		{{range .WritableCols}}{{if .Nullable}}{{.GoName}}: omitnull.FromPtr(req.{{.GoName}}),
		{{else}}{{.GoName}}: omit.From(req.{{.GoName}}),
		{{end}}{{end}}
	}
	row, err := dbmodels.{{.Table.GoNamePlural}}.Insert(setter).One(ctx, s.db)
	if err != nil {
		return nil, fmt.Errorf("{{.Table.Name}}.Create: %w", err)
	}
	return mappers.{{.Table.GoName}}ToType(row), nil
	{{end}}}
{{end}}

{{if isOperationEnabled "update" .Override}}
// Update modifies an existing {{.Table.GoName}} record.
func (s *{{.Table.GoName}}Store) Update(ctx context.Context, id {{.Table.PKTypeName}}, req types.Update{{.Table.GoName}}Request) (*models.{{.Table.GoName}}, error) {
	row, err := dbmodels.{{.Table.GoNamePlural}}.Query(
		sm.Where(dbmodels.{{.Table.GoNamePlural}}.Columns.{{.Table.PrimaryKey.GoName}}.EQ({{.BobPkg}}.Arg(id))),
	).One(ctx, s.db)
	if err != nil {
		return nil, fmt.Errorf("{{.Table.Name}}.Update find: %w", err)
	}
	setter := &dbmodels.{{.Table.GoName}}Setter{}
	{{range .WritableCols}}if req.{{.GoName}} != nil {
		{{if .Nullable}}setter.{{.GoName}} = omitnull.FromPtr(req.{{.GoName}})
		{{else}}setter.{{.GoName}} = omit.From(*req.{{.GoName}})
		{{end}}
	}
	{{end}}
	err = row.Update(ctx, s.db, setter)
	if err != nil {
		return nil, fmt.Errorf("{{.Table.Name}}.Update: %w", err)
	}
	return mappers.{{.Table.GoName}}ToType(row), nil
}
{{end}}

{{if isOperationEnabled "delete" .Override}}
// Delete removes a {{.Table.GoName}} record by primary key.
func (s *{{.Table.GoName}}Store) Delete(ctx context.Context, id {{.Table.PKTypeName}}) error {
	row, err := dbmodels.{{.Table.GoNamePlural}}.Query(
		sm.Where(dbmodels.{{.Table.GoNamePlural}}.Columns.{{.Table.PrimaryKey.GoName}}.EQ({{.BobPkg}}.Arg(id))),
	).One(ctx, s.db)
	if err != nil {
		return fmt.Errorf("{{.Table.Name}}.Delete find: %w", err)
	}
	if err := row.Delete(ctx, s.db); err != nil {
		return fmt.Errorf("{{.Table.Name}}.Delete: %w", err)
	}
	return nil
}
{{end}}
`

// ---------------------------------------------------------------------------
// Mapper template — written once, never overwritten
// ---------------------------------------------------------------------------

const mapperTemplate = `// Generated by kiln on first run. THIS FILE IS YOURS.
// kiln will never overwrite it. Add computed fields,
// transformations, or custom logic here freely.

package mappers

import (
	dbmodels "{{.ModelsPath}}"
	"{{.ImportPath}}/models"
)

// {{.Table.GoName}}ToType maps a bob model to a models.{{.Table.GoName}}.
func {{.Table.GoName}}ToType(m *dbmodels.{{.Table.GoName}}) *models.{{.Table.GoName}} {
	if m == nil {
		return nil
	}
	t := &models.{{.Table.GoName}}{}
{{range .VisibleCols}}{{if .Nullable}}	if !m.{{.GoName}}.IsNull() {
		v := m.{{.GoName}}.MustGet()
		t.{{.GoName}} = &v
	}
{{else}}	t.{{.GoName}} = m.{{.GoName}}
{{end}}{{end}}	return t
}

// {{.Table.GoNamePlural}}ToTypes maps a slice of bob models to a slice of models.{{.Table.GoName}}.
func {{.Table.GoNamePlural}}ToTypes(rows []*dbmodels.{{.Table.GoName}}) []models.{{.Table.GoName}} {
	out := make([]models.{{.Table.GoName}}, 0, len(rows))
	for _, r := range rows {
		if mapped := {{.Table.GoName}}ToType(r); mapped != nil {
			out = append(out, *mapped)
		}
	}
	return out
}
`
