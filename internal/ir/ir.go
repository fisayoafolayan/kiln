// Package ir defines the Internal Representation of a database schema.
// It is the single source of truth that all generators consume.
// Parsers (bob, postgres, mysql, sqlite) produce IR. Generators consume IR.
// IR is database-agnostic — no dialect-specific types leak through.
package ir

import (
	"fmt"
	"sort"
	"strings"
)

// Schema is the top-level IR produced by the parser.
// It represents the full picture of a database schema.
type Schema struct {
	Driver Driver // postgres | mysql | sqlite
	Tables []*Table
	// TableMap provides O(1) lookup by table name.
	TableMap map[string]*Table
}

// Driver identifies the database dialect.
type Driver string

const (
	DriverPostgres Driver = "postgres"
	DriverMySQL    Driver = "mysql"
	DriverSQLite   Driver = "sqlite"
)

// RelationHint is a belongs-to relationship extracted from bob's R struct.
// FieldName is the R struct field name (e.g. "Post", "Author"),
// TargetModel is the Go model name (e.g. "Post", "User").
type RelationHint struct {
	FieldName   string // R struct field name, e.g. "Post", "Author"
	TargetModel string // pointed-to model name, e.g. "Post", "User"
}

// Table represents a single database table.
type Table struct {
	Name    string // raw table name as it appears in the DB
	Columns []*Column
	Indexes []*Index

	// Derived fields — populated by the parser after all tables are loaded.
	PrimaryKey  *Column // nil if composite PK (unsupported in v1)
	ForeignKeys []*ForeignKey
	// ReferencedBy holds FKs from other tables pointing to this table.
	// Used to generate nested routes e.g. GET /users/{id}/posts
	ReferencedBy []*ForeignKey

	// ColumnMap provides O(1) lookup by column name.
	ColumnMap map[string]*Column

	// Meta holds parser-specific data not used by generators directly.
	// Used during relationship resolution to carry bob R struct hints.
	Meta []RelationHint `json:"-"`
}

// GoName returns the PascalCase singular Go type name for this table.
// e.g. "user_profiles" → "UserProfile", "categories" → "Category"
func (t *Table) GoName() string {
	return toPascalCase(t.Name)
}

// GoNamePlural returns the PascalCase plural Go type name for this table.
// e.g. "user_profiles" → "UserProfiles", "categories" → "Categories"
func (t *Table) GoNamePlural() string {
	parts := splitWords(t.Name)
	result := make([]string, len(parts))
	for i, p := range parts {
		if i == len(parts)-1 {
			p = pluralize(p)
		}
		if len(p) == 0 {
			continue
		}
		result[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(result, "")
}

// Endpoint returns the kebab-case HTTP endpoint name for this table.
// e.g. "user_profiles" → "user-profiles"
func (t *Table) Endpoint() string {
	return toKebabCase(t.Name)
}

// Column represents a single column in a table.
type Column struct {
	Name         string
	GoType       GoType // resolved, database-agnostic Go type
	RawType      string // original DB type string e.g. "varchar(255)", "timestamptz"
	Nullable     bool
	IsPrimaryKey bool
	IsUnique     bool
	HasDefault   bool   // true if DB has a default value (affects Create request structs)
	DefaultValue string // raw default expression e.g. "gen_random_uuid()", "now()"
	MaxLength    *int   // populated for string types from varchar(n)
	Ordinal      int    // column position in the table
}

// GoName returns the idiomatic Go field name for this column,
// respecting common initialisms like ID, URL, API etc.
// e.g. "user_id" → "UserID", "api_key" → "APIKey"
func (c *Column) GoName() string {
	return toGoFieldName(c.Name)
}

// JSONName returns the snake_case JSON tag name for this column.
// e.g. "CreatedAt" → "created_at"
func (c *Column) JSONName() string {
	return c.Name
}

// IsReadOnly returns true if this column should be excluded from
// Create and Update request structs (PKs and DB-managed fields).
func (c *Column) IsReadOnly() bool {
	return c.IsPrimaryKey || c.Name == "created_at" || c.Name == "updated_at"
}

// GoType is a database-agnostic Go type resolved from the raw DB type.
// All dialect-specific mappings (postgres uuid, mysql tinyint(1), etc.)
// are normalised here so generators never need to think about dialects.
type GoType struct {
	Name    string // e.g. "string", "int64", "bool", "time.Time", "uuid.UUID"
	Package string // import path if not a builtin e.g. "time", "github.com/google/uuid"
	IsPtr   bool   // true when the column is nullable → *string, *time.Time etc.
}

// String returns the Go source representation of this type.
// e.g. "string", "*time.Time", "uuid.UUID"
func (g GoType) String() string {
	if g.IsPtr {
		return "*" + g.Name
	}
	return g.Name
}

// Common GoTypes — use these constants in the parser rather than
// constructing GoType literals to keep mappings consistent.
var (
	GoTypeString    = GoType{Name: "string"}
	GoTypeInt32     = GoType{Name: "int32"}
	GoTypeInt64     = GoType{Name: "int64"}
	GoTypeFloat64   = GoType{Name: "float64"}
	GoTypeBool      = GoType{Name: "bool"}
	GoTypeTime      = GoType{Name: "time.Time", Package: "time"}
	GoTypeUUID      = GoType{Name: "uuid.UUID", Package: "github.com/gofrs/uuid/v5"}
	GoTypeByteSlice = GoType{Name: "[]byte"}
	GoTypeJSON      = GoType{Name: "json.RawMessage", Package: "encoding/json"}
)

// NullableOf returns a pointer variant of the given GoType.
func NullableOf(t GoType) GoType {
	t.IsPtr = true
	return t
}

// ForeignKey represents a relationship between two tables.
type ForeignKey struct {
	// The table and column that holds the FK value.
	SourceTable  *Table
	SourceColumn *Column
	// The table and column being referenced.
	TargetTable  *Table
	TargetColumn *Column

	// OnDelete / OnUpdate actions e.g. "CASCADE", "SET NULL", "RESTRICT"
	OnDelete string
	OnUpdate string
}

// GoName returns a descriptive name for this FK relationship.
// Used in generated code comments and nested route registration.
// e.g. "PostsByUserID"
func (fk *ForeignKey) GoName() string {
	return toPascalCase(fk.SourceTable.Name) + "By" + toPascalCase(fk.SourceColumn.Name)
}

// Index represents a database index.
type Index struct {
	Name    string
	Columns []*Column
	Unique  bool
}

// ValidationTag returns the `validate` struct tag value for a column.
// Used by the types generator to produce validate annotations.
func (c *Column) ValidationTag() string {
	tags := []string{}

	if !c.Nullable && !c.HasDefault && !c.IsPrimaryKey {
		tags = append(tags, "required")
	}
	if c.GoType.Name == "string" {
		if c.MaxLength != nil {
			tags = append(tags, fmt.Sprintf("max=%d", *c.MaxLength))
		}
	}
	if len(tags) == 0 {
		return ""
	}
	return strings.Join(tags, ",")
}

// FilterTable returns a copy of the schema containing only the named table.
// Returns an error if the table is not found.
func (s *Schema) FilterTable(name string) (*Schema, error) {
	t, ok := s.TableMap[name]
	if !ok {
		return nil, fmt.Errorf("table %q not found in schema", name)
	}
	return &Schema{
		Driver:   s.Driver,
		Tables:   []*Table{t},
		TableMap: map[string]*Table{name: t},
	}, nil
}

// Imports returns the set of import paths required for this schema
// across all tables and columns. Used by generators to build import blocks.
func (s *Schema) Imports() []string {
	seen := map[string]bool{}
	var imports []string
	for _, t := range s.Tables {
		for _, c := range t.Columns {
			if c.GoType.Package != "" && !seen[c.GoType.Package] {
				seen[c.GoType.Package] = true
				imports = append(imports, c.GoType.Package)
			}
		}
	}
	sort.Strings(imports)
	return imports
}
