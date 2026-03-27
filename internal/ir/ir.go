// Package ir defines the Internal Representation of a database schema.
// It is the single source of truth that all generators consume.
// Parsers (bob, postgres, mysql, sqlite) produce IR. Generators consume IR.
// IR is database-agnostic - no dialect-specific types leak through.
package ir

import (
	"fmt"
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

	// Derived fields - populated by the parser after all tables are loaded.
	PrimaryKeys []*Column // ordered list of PK columns; len==1 for single PK
	ForeignKeys []*ForeignKey
	// ReferencedBy holds FKs from other tables pointing to this table.
	// Used to generate nested routes e.g. GET /users/{id}/posts
	ReferencedBy []*ForeignKey

	// ManyToMany holds M2M relationships discovered via junction tables.
	// Used to generate link/unlink endpoints e.g. POST /posts/{id}/tags
	ManyToMany []*ManyToMany

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

// HasPK returns true if this table has at least one primary key column.
func (t *Table) HasPK() bool {
	return len(t.PrimaryKeys) > 0
}

// HasCompositePK returns true if this table has a multi-column primary key.
func (t *Table) HasCompositePK() bool {
	return len(t.PrimaryKeys) > 1
}

// PKTypeName returns the Go type name for a single primary key (e.g. "string", "int64", "uuid.UUID").
// For composite PKs, returns "string" (callers should use PrimaryKeys directly).
func (t *Table) PKTypeName() string {
	if len(t.PrimaryKeys) != 1 {
		return "string"
	}
	return t.PrimaryKeys[0].GoType.Name
}

// PKIsUUID returns true if the table has a single UUID primary key.
func (t *Table) PKIsUUID() bool {
	return len(t.PrimaryKeys) == 1 && t.PrimaryKeys[0].GoType.Name == "uuid.UUID"
}

// PKIsStringLike returns true if the table has a single string or UUID primary key.
func (t *Table) PKIsStringLike() bool {
	if len(t.PrimaryKeys) != 1 {
		return false
	}
	name := t.PrimaryKeys[0].GoType.Name
	return name == "string" || name == "uuid.UUID"
}

// SoftDeleteColumn returns the soft-delete column if this table has one, or nil.
func (t *Table) SoftDeleteColumn() *Column {
	for _, c := range t.Columns {
		if c.IsSoftDeleteColumn() {
			return c
		}
	}
	return nil
}

// HasSoftDelete returns true if this table uses soft delete.
func (t *Table) HasSoftDelete() bool {
	return t.SoftDeleteColumn() != nil
}

// Column represents a single column in a table.
type Column struct {
	Name         string
	GoType       GoType // resolved, database-agnostic Go type
	RawType      string // original DB type string e.g. "varchar(255)", "timestamptz"
	Nullable     bool
	IsPrimaryKey bool
	IsUnique     bool
	HasDefault   bool     // true if DB has a default value (affects Create request structs)
	DefaultValue string   // raw default expression e.g. "gen_random_uuid()", "now()"
	MaxLength    *int     // populated for string types from varchar(n)
	EnumValues   []string // allowed values from config; generates oneof= validation
	Ordinal      int      // column position in the table
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

// autoManagedTimestamps are column names that are typically set by the
// database (via DEFAULT now() or triggers) and should not appear in
// Create/Update request structs.
// Users with non-standard names should use overrides.readonly_fields.
var autoManagedTimestamps = map[string]bool{
	"created_at":  true,
	"updated_at":  true,
	"inserted_at": true,
	"modified_at": true,
	"deleted_at":  true,
	"created_on":  true,
	"updated_on":  true,
}

// IsReadOnly returns true if this column should be excluded from
// Create and Update request structs.
//
// A column is auto-readonly if it is:
//   - a primary key with a database default (e.g. auto-generated UUID), or
//   - a timestamp with HasDefault set (from schema introspection), or
//   - a timestamp matching a well-known auto-managed name pattern
//
// Composite PK columns without defaults are writable — the caller must supply them.
// For other cases, use overrides.readonly_fields in kiln.yaml.
func (c *Column) IsReadOnly() bool {
	if c.IsPrimaryKey && c.HasDefault {
		return true
	}
	if c.GoType.Name == "time.Time" {
		// If the parser set HasDefault, trust it.
		if c.HasDefault {
			return true
		}
		// Fallback: match common auto-managed timestamp names.
		// This handles parsers (like bob) that can't detect DB defaults.
		return autoManagedTimestamps[c.Name]
	}
	return false
}

// IsSoftDeleteColumn returns true if this column is a nullable deleted_at timestamp.
func (c *Column) IsSoftDeleteColumn() bool {
	return c.Name == "deleted_at" && c.Nullable && c.GoType.Name == "time.Time"
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

// Common GoTypes - use these constants in the parser rather than
// constructing GoType literals to keep mappings consistent.
var (
	GoTypeString     = GoType{Name: "string"}
	GoTypeInt32      = GoType{Name: "int32"}
	GoTypeInt64      = GoType{Name: "int64"}
	GoTypeFloat64    = GoType{Name: "float64"}
	GoTypeBool       = GoType{Name: "bool"}
	GoTypeTime       = GoType{Name: "time.Time", Package: "time"}
	GoTypeUUID       = GoType{Name: "uuid.UUID", Package: "github.com/gofrs/uuid/v5"}
	GoTypeByteSlice  = GoType{Name: "[]byte"}
	GoTypeJSON       = GoType{Name: "json.RawMessage", Package: "encoding/json"}
	GoTypeStringArr  = GoType{Name: "[]string"}
	GoTypeInt32Arr   = GoType{Name: "[]int32"}
	GoTypeInt64Arr   = GoType{Name: "[]int64"}
	GoTypeFloat64Arr = GoType{Name: "[]float64"}
	GoTypeBoolArr    = GoType{Name: "[]bool"}
	GoTypeUUIDArr    = GoType{Name: "[]uuid.UUID", Package: "github.com/gofrs/uuid/v5"}
)

// IsFilterable returns true if this type supports query filtering.
func (g GoType) IsFilterable() bool {
	switch g.Name {
	case "string", "uuid.UUID", "int32", "int64", "float64", "bool", "time.Time":
		return true
	}
	return false
}

// SupportsRangeOps returns true if this type supports gt/gte/lt/lte operators.
func (g GoType) SupportsRangeOps() bool {
	switch g.Name {
	case "int32", "int64", "float64", "time.Time":
		return true
	}
	return false
}

// NullableOf returns a pointer variant of the given GoType.
func NullableOf(t GoType) GoType {
	t.IsPtr = true
	return t
}

// ManyToMany represents a many-to-many relationship through a junction table.
type ManyToMany struct {
	// JunctionTable is the raw junction table name e.g. "post_tags".
	JunctionTable string
	// JunctionSourceCol is the junction column pointing to this table e.g. "post_id".
	JunctionSourceCol string
	// JunctionTargetCol is the junction column pointing to the related table e.g. "tag_id".
	JunctionTargetCol string
	// TargetTable is the table on the other side of the relationship.
	TargetTable *Table
	// ExtraColumns are non-FK, non-PK columns on the junction table (e.g. created_at).
	ExtraColumns []*Column
}

// ForeignKey represents a relationship between two tables.
type ForeignKey struct {
	// The table and column that holds the FK value.
	SourceTable  *Table
	SourceColumn *Column
	// The table and column being referenced.
	TargetTable  *Table
	TargetColumn *Column
}

// ValidationTag returns the `validate` struct tag value for a column.
// Used by the types generator for Create request annotations.
func (c *Column) ValidationTag() string {
	var tags []string

	if !c.Nullable && !c.HasDefault && !c.IsPrimaryKey {
		tags = append(tags, "required")
	}
	if len(c.EnumValues) > 0 {
		tags = append(tags, "oneof="+strings.Join(c.EnumValues, " "))
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

// UpdateValidationTag returns the `validate` struct tag for Update requests.
// All fields are optional (pointer + omitempty), so "required" is stripped
// and "omitempty" is prepended to any remaining rules.
func (c *Column) UpdateValidationTag() string {
	var tags []string

	// Never required on update - all fields are optional.
	if len(c.EnumValues) > 0 {
		tags = append(tags, "oneof="+strings.Join(c.EnumValues, " "))
	}
	if c.GoType.Name == "string" {
		if c.MaxLength != nil {
			tags = append(tags, fmt.Sprintf("max=%d", *c.MaxLength))
		}
	}
	if len(tags) == 0 {
		return ""
	}
	// omitempty tells the validator to skip nil pointers.
	return "omitempty," + strings.Join(tags, ",")
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
