// Package bob implements the kiln parser by reading bob's generated
// models directory and converting them into kiln's IR.
package bob

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fisayoafolayan/kiln/internal/ir"
	"github.com/gobuffalo/flect"
)

// minBobVersion is the minimum bob generator version kiln is tested against.
// The parser relies on the R struct and db tag format introduced in this version.
var minBobVersion = [3]int{0, 42, 0}

// Parser reads bob's generated models directory and produces an ir.Schema.
type Parser struct {
	ModelsDir  string
	Driver     ir.Driver
	Exclude    []string // table names to skip
	excludeSet map[string]bool
}

// New returns a Parser configured for the given models directory and driver.
func New(modelsDir string, driver ir.Driver) *Parser {
	return &Parser{ModelsDir: modelsDir, Driver: driver}
}

// Parse reads bob's generated models and returns a fully resolved ir.Schema.
func (p *Parser) Parse() (*ir.Schema, error) {
	files, err := filepath.Glob(filepath.Join(p.ModelsDir, "*.bob.go"))
	if err != nil {
		return nil, fmt.Errorf("reading models dir %q: %w", p.ModelsDir, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf(
			"no .bob.go files found in %q — did you run kiln generate after setting up the database?",
			p.ModelsDir,
		)
	}

	// Build exclude set for O(1) lookups.
	p.excludeSet = make(map[string]bool, len(p.Exclude))
	for _, e := range p.Exclude {
		p.excludeSet[e] = true
	}

	// Check bob version from the first model file's header comment.
	if err := p.checkBobVersion(files[0]); err != nil {
		return nil, err
	}

	schema := &ir.Schema{
		Driver:   p.Driver,
		TableMap: map[string]*ir.Table{},
	}

	for _, f := range files {
		if isBoilerplateFile(f) {
			continue
		}
		t, err := p.parseModelFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [parser] WARNING: skipping %s: %v\n", filepath.Base(f), err)
			continue
		}
		if t == nil {
			continue
		}
		// Skip tables with no primary key (e.g. composite PK join tables)
		if t.PrimaryKey == nil {
			continue
		}
		// Skip explicitly excluded tables
		if p.isExcluded(t.Name) {
			continue
		}
		schema.Tables = append(schema.Tables, t)
		schema.TableMap[t.Name] = t
	}

	if err := p.resolveRelationships(schema); err != nil {
		return nil, fmt.Errorf("resolving relationships: %w", err)
	}

	return schema, nil
}

// parseModelFile parses a single bob-generated .bob.go file and returns
// an *ir.Table. Returns nil, nil if the file doesn't represent a table.
func (p *Parser) parseModelFile(path string) (*ir.Table, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", path, err)
	}

	// Find the primary model struct — exported, not ending in Slice/Setter/Query/R
	var modelStruct *ast.StructType
	var modelName string

	ast.Inspect(f, func(n ast.Node) bool {
		if modelStruct != nil {
			return false
		}
		ts, ok := n.(*ast.TypeSpec)
		if !ok || !ts.Name.IsExported() {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		name := ts.Name.Name
		for _, suffix := range []string{"Slice", "Setter", "Query", "Columns"} {
			if strings.HasSuffix(name, suffix) {
				return true
			}
		}
		modelStruct = st
		modelName = name
		return false
	})

	if modelStruct == nil {
		return nil, nil
	}

	table := &ir.Table{
		Name:      toTableName(modelName),
		ColumnMap: map[string]*ir.Column{},
	}

	for i, field := range modelStruct.Fields.List {
		if len(field.Names) == 0 {
			continue // embedded field (e.g. R userR) — skip
		}

		name := field.Names[0].Name
		if !ast.IsExported(name) {
			continue
		}

		// Skip the R (relations) field
		if name == "R" {
			continue
		}

		// Read column name and pk from the db struct tag
		// e.g. `db:"id,pk"` or `db:"email"`
		colName, isPK := parseDBTag(field.Tag)
		if colName == "" {
			colName = toSnakeCase(name) // fallback
		}
		if colName == "-" {
			continue // explicitly excluded
		}

		col := &ir.Column{
			Name:         colName,
			GoType:       p.resolveGoType(field.Type),
			IsPrimaryKey: isPK,
			Nullable:     isNullableType(field.Type),
			Ordinal:      i,
		}

		table.Columns = append(table.Columns, col)
		table.ColumnMap[col.Name] = col

		if col.IsPrimaryKey && table.PrimaryKey == nil {
			table.PrimaryKey = col
		}
	}

	if len(table.Columns) == 0 {
		return nil, nil
	}

	// Parse the R struct to extract relationship hints.
	// e.g. type commentR struct { Post *Post; User *User }
	// tells us comments belongs-to posts (via post_id) and users (via user_id).
	table.Meta = parseRelationHints(f, modelName)

	return table, nil
}

// resolveRelationships wires up ForeignKey relationships between tables.
//
// Primary strategy: use bob's R struct hints (parsed from the model file).
// Each belongs-to field in the R struct (e.g. `Author *User`) tells us:
//   - The FK column name: snake_case(FieldName) + "_id" (e.g. "author_id")
//   - The target table: pluralize(snake_case(TargetModel)) (e.g. "users")
//
// Fallback: if no R struct hints are available (e.g. hand-written models),
// fall back to the _id column naming convention.
func (p *Parser) resolveRelationships(schema *ir.Schema) error {
	// Build a lookup from model name → table for R struct resolution.
	modelToTable := make(map[string]*ir.Table, len(schema.Tables))
	for _, t := range schema.Tables {
		// Reverse toTableName: "users" came from model "User".
		// We singularize and PascalCase the table name.
		model := flect.Pascalize(flect.Singularize(t.Name))
		modelToTable[model] = t
	}

	for _, t := range schema.Tables {
		if len(t.Meta) > 0 {
			// Use R struct hints — accurate FK resolution.
			p.resolveFromHints(schema, t, modelToTable)
		} else {
			// Fallback: _id column heuristic.
			p.resolveFromColumns(schema, t)
		}
	}
	return nil
}

// resolveFromHints uses the R struct belongs-to fields to wire up FKs.
func (p *Parser) resolveFromHints(schema *ir.Schema, t *ir.Table, modelToTable map[string]*ir.Table) {
	for _, hint := range t.Meta {
		// FK column: snake_case(FieldName) + "_id"
		// e.g. "Author" → "author_id", "Post" → "post_id"
		fkColName := toSnakeCase(hint.FieldName) + "_id"
		fkCol, ok := t.ColumnMap[fkColName]
		if !ok {
			continue
		}

		// Target table: look up by model name.
		targetTable, ok := modelToTable[hint.TargetModel]
		if !ok || targetTable.PrimaryKey == nil {
			continue
		}

		fk := &ir.ForeignKey{
			SourceTable:  t,
			SourceColumn: fkCol,
			TargetTable:  targetTable,
			TargetColumn: targetTable.PrimaryKey,
		}
		t.ForeignKeys = append(t.ForeignKeys, fk)
		targetTable.ReferencedBy = append(targetTable.ReferencedBy, fk)
	}
}

// resolveFromColumns is the fallback heuristic: columns ending in _id.
func (p *Parser) resolveFromColumns(schema *ir.Schema, t *ir.Table) {
	for _, c := range t.Columns {
		if !strings.HasSuffix(c.Name, "_id") {
			continue
		}
		targetName := flect.Pluralize(strings.TrimSuffix(c.Name, "_id"))
		targetTable, ok := schema.TableMap[targetName]
		if !ok || targetTable.PrimaryKey == nil {
			continue
		}
		fk := &ir.ForeignKey{
			SourceTable:  t,
			SourceColumn: c,
			TargetTable:  targetTable,
			TargetColumn: targetTable.PrimaryKey,
		}
		t.ForeignKeys = append(t.ForeignKeys, fk)
		targetTable.ReferencedBy = append(targetTable.ReferencedBy, fk)
	}
}

// parseRelationHints finds the R struct (e.g. `commentR`) in the AST
// and extracts belongs-to relationships from pointer fields.
//
// Bob generates: type commentR struct { Post *Post; User *User }
// Pointer fields (*Post) are belongs-to; slice fields (PostSlice) are has-many.
// We only care about belongs-to — has-many is the inverse, derived later.
func parseRelationHints(f *ast.File, modelName string) []ir.RelationHint {
	// The R struct is named lowercase(modelName) + "R", e.g. "commentR".
	rStructName := strings.ToLower(modelName[:1]) + modelName[1:] + "R"

	var hints []ir.RelationHint
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != rStructName {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return false
		}
		for _, field := range st.Fields.List {
			if len(field.Names) == 0 {
				continue
			}
			// Only pointer fields are belongs-to relationships.
			star, ok := field.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			// Extract the target model name from *ModelName.
			ident, ok := star.X.(*ast.Ident)
			if !ok {
				continue
			}
			hints = append(hints, ir.RelationHint{
				FieldName:   field.Names[0].Name,
				TargetModel: ident.Name,
			})
		}
		return false
	})
	return hints
}

// resolveGoType converts an ast.Expr from bob's generated model into
// a kiln ir.GoType. Handles bob v0.42.0 null.Val[T] generic nullable types.
func (p *Parser) resolveGoType(expr ast.Expr) ir.GoType {
	switch t := expr.(type) {

	case *ast.Ident:
		return identToGoType(t.Name)

	case *ast.StarExpr:
		inner := p.resolveGoType(t.X)
		return ir.NullableOf(inner)

	case *ast.SelectorExpr:
		// e.g. time.Time, uuid.UUID, pgtypes.JSONB
		pkg := ""
		if id, ok := t.X.(*ast.Ident); ok {
			pkg = id.Name
		}
		return selectorToGoType(pkg, t.Sel.Name)

	case *ast.IndexExpr:
		// Generic types: null.Val[string], omit.Val[string], omitnull.Val[string]
		// These are bob's nullable wrapper types — extract the inner type
		// and mark it as nullable.
		if sel, ok := t.X.(*ast.SelectorExpr); ok {
			pkg := ""
			if id, ok := sel.X.(*ast.Ident); ok {
				pkg = id.Name
			}
			// null.Val[T] and omitnull.Val[T] → nullable
			// omit.Val[T] → not nullable (field is optional on input but not null in DB)
			inner := p.resolveGoType(t.Index)
			if pkg == "null" || pkg == "omitnull" {
				return ir.NullableOf(inner)
			}
			return inner
		}

	case *ast.ArrayType:
		if t.Len == nil {
			inner := p.resolveGoType(t.Elt)
			if inner.Name == "byte" {
				return ir.GoTypeByteSlice
			}
		}
	}

	return ir.GoTypeString // fallback
}

// parseDBTag reads the `db` struct tag and returns the column name and
// whether the field is a primary key.
// e.g. `db:"id,pk"` → ("id", true)
//
//	`db:"email"`  → ("email", false)
//	`db:"-"`      → ("-", false)
func parseDBTag(tag *ast.BasicLit) (colName string, isPK bool) {
	if tag == nil {
		return "", false
	}
	raw := strings.Trim(tag.Value, "`")
	// Extract the db:"..." value
	const prefix = `db:"`
	idx := strings.Index(raw, prefix)
	if idx == -1 {
		return "", false
	}
	rest := raw[idx+len(prefix):]
	end := strings.Index(rest, `"`)
	if end == -1 {
		return "", false
	}
	val := rest[:end]
	parts := strings.Split(val, ",")
	colName = parts[0]
	for _, p := range parts[1:] {
		if strings.TrimSpace(p) == "pk" {
			isPK = true
		}
	}
	return colName, isPK
}

// isNullableType returns true if the type is a bob nullable wrapper.
func isNullableType(expr ast.Expr) bool {
	// *T → nullable
	if _, ok := expr.(*ast.StarExpr); ok {
		return true
	}
	// null.Val[T] or omitnull.Val[T] → nullable
	if idx, ok := expr.(*ast.IndexExpr); ok {
		if sel, ok := idx.X.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				return id.Name == "null" || id.Name == "omitnull"
			}
		}
	}
	return false
}

// checkBobVersion reads the first line of a bob-generated file and verifies
// the generator version meets the minimum requirement.
// Header format: "// Code generated by BobGen psql v0.42.0. DO NOT EDIT."
func (p *Parser) checkBobVersion(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // version check is best-effort
	}
	firstLine, _, _ := strings.Cut(string(data), "\n")

	// Extract version: find "vX.Y.Z" in the header.
	idx := strings.Index(firstLine, " v")
	if idx == -1 {
		return nil // can't determine version, skip check
	}
	verStr := firstLine[idx+2:]
	if dot := strings.IndexByte(verStr, '.'); dot != -1 {
		// Trim anything after the version (e.g. ". DO NOT EDIT.")
		if end := strings.Index(verStr, ". "); end != -1 {
			verStr = verStr[:end]
		}
	}

	ver, ok := parseSemver(verStr)
	if !ok {
		return nil // unparseable, skip
	}

	if compareSemver(ver, minBobVersion) < 0 {
		return fmt.Errorf(
			"bob generator version v%d.%d.%d is older than the minimum required v%d.%d.%d\n\n"+
				"  Upgrade with: go install github.com/stephenafamo/bob/gen/bobgen-psql@latest",
			ver[0], ver[1], ver[2],
			minBobVersion[0], minBobVersion[1], minBobVersion[2],
		)
	}
	return nil
}

// parseSemver parses "X.Y.Z" into [3]int. Returns false if unparseable.
func parseSemver(s string) ([3]int, bool) {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var v [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		v[i] = n
	}
	return v, true
}

// compareSemver returns -1, 0, or 1.
func compareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func (p *Parser) isExcluded(name string) bool {
	return p.excludeSet[name]
}

func isBoilerplateFile(path string) bool {
	base := filepath.Base(path)
	for _, b := range []string{
		"bob_joins.bob.go",
		"bob_loaders.bob.go",
		"bob_where.bob.go",
		"bob_types.bob.go",
	} {
		if base == b {
			return true
		}
	}
	return strings.HasSuffix(base, "_test.go")
}

// toTableName converts a PascalCase bob model name to snake_case plural table name.
// "UserProfile" → "user_profiles", "Category" → "categories"
func toTableName(modelName string) string {
	return flect.Underscore(flect.Pluralize(modelName))
}

func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' && i > 0 {
			b.WriteByte('_')
		}
		b.WriteRune(r | 0x20)
	}
	return b.String()
}

func identToGoType(name string) ir.GoType {
	switch name {
	case "string":
		return ir.GoTypeString
	case "int", "int32":
		return ir.GoTypeInt32
	case "int64":
		return ir.GoTypeInt64
	case "float32", "float64":
		return ir.GoTypeFloat64
	case "bool":
		return ir.GoTypeBool
	case "byte":
		return ir.GoType{Name: "byte"}
	default:
		return ir.GoType{Name: name}
	}
}

func selectorToGoType(pkg, name string) ir.GoType {
	switch pkg + "." + name {
	case "time.Time":
		return ir.GoTypeTime
	case "uuid.UUID":
		return ir.GoTypeUUID
	case "pgtypes.JSONB", "pgtypes.JSON":
		return ir.GoTypeJSON
	default:
		return ir.GoType{Name: name, Package: pkg}
	}
}
