// Package bobadapter converts bob's drivers.DBInfo into kiln's ir.Schema.
// Used by both the kiln CLI and the bob plugin.
package bobadapter

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/fisayoafolayan/kiln/internal/ir"
	"github.com/stephenafamo/bob/gen/drivers"
)

var varcharLenRe = regexp.MustCompile(`(?i)(?:var)?char\((\d+)\)`)

// ConvertDBInfo converts bob's DBInfo into kiln's ir.Schema.
func ConvertDBInfo[T, C, I any](info *drivers.DBInfo[T, C, I], driver ir.Driver, exclude map[string]bool) *ir.Schema {
	schema := &ir.Schema{
		Driver:   driver,
		TableMap: map[string]*ir.Table{},
	}

	for _, dt := range info.Tables {
		if exclude[dt.Name] {
			continue
		}
		t := convertTable(dt)
		if t == nil {
			continue
		}
		schema.Tables = append(schema.Tables, t)
		schema.TableMap[t.Name] = t
	}

	// Wire up foreign keys.
	for _, dt := range info.Tables {
		t, ok := schema.TableMap[dt.Name]
		if !ok {
			continue
		}
		for _, fk := range dt.Constraints.Foreign {
			if len(fk.Columns) == 0 || len(fk.ForeignColumns) == 0 {
				continue
			}
			targetTable, ok := schema.TableMap[fk.ForeignTable]
			if !ok || !targetTable.HasPK() {
				continue
			}
			sourceCol := t.ColumnMap[fk.Columns[0]]
			if sourceCol == nil {
				continue
			}
			fkIR := &ir.ForeignKey{
				SourceTable:  t,
				SourceColumn: sourceCol,
				TargetTable:  targetTable,
				TargetColumn: targetTable.PrimaryKeys[0],
			}
			t.ForeignKeys = append(t.ForeignKeys, fkIR)
			targetTable.ReferencedBy = append(targetTable.ReferencedBy, fkIR)
		}
	}

	// Apply enum values from CHECK constraints.
	for _, dt := range info.Tables {
		t, ok := schema.TableMap[dt.Name]
		if !ok {
			continue
		}
		for _, chk := range dt.Constraints.Checks {
			if chk.Expression == "" || len(chk.Columns) == 0 {
				continue
			}
			vals := ExtractINValues(chk.Expression)
			if len(vals) == 0 {
				continue
			}
			for _, colName := range chk.Columns {
				if c, ok := t.ColumnMap[colName]; ok && len(c.EnumValues) == 0 {
					c.EnumValues = vals
				}
			}
		}
	}

	// Apply enum values from bob's native enum detection.
	if len(info.Enums) > 0 {
		enumMap := make(map[string][]string, len(info.Enums))
		for _, e := range info.Enums {
			enumMap[e.Type] = e.Values
		}
		for _, t := range schema.Tables {
			for _, c := range t.Columns {
				if c.RawType != "" && len(c.EnumValues) == 0 {
					if vals, ok := enumMap[c.RawType]; ok {
						c.EnumValues = vals
					}
				}
			}
		}
	}

	// Classify junction tables: composite PK tables that are pure M2M junctions
	// get removed from the schema and converted to ManyToMany relationships.
	classifyJunctions(schema)

	return schema
}

// classifyJunctions identifies junction tables among composite-PK tables in the
// schema, removes them from Tables/TableMap, and wires up ManyToMany on both sides.
// A table is a junction if: exactly 2 PK columns, exactly 2 FKs, each FK column
// is a PK column, and both FK targets are in the schema.
func classifyJunctions(schema *ir.Schema) {
	var junctions []*ir.Table
	for _, t := range schema.Tables {
		if !t.HasCompositePK() || len(t.PrimaryKeys) != 2 {
			continue
		}
		if len(t.ForeignKeys) != 2 {
			continue
		}
		pkSet := map[string]bool{}
		for _, pk := range t.PrimaryKeys {
			pkSet[pk.Name] = true
		}
		// Each FK's source column must be a PK column.
		valid := true
		for _, fk := range t.ForeignKeys {
			if !pkSet[fk.SourceColumn.Name] {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		junctions = append(junctions, t)
	}

	// Remove junction tables from the schema.
	for _, jt := range junctions {
		delete(schema.TableMap, jt.Name)
	}
	filtered := make([]*ir.Table, 0, len(schema.Tables)-len(junctions))
	junctionSet := map[string]bool{}
	for _, jt := range junctions {
		junctionSet[jt.Name] = true
	}
	for _, t := range schema.Tables {
		if !junctionSet[t.Name] {
			filtered = append(filtered, t)
		}
	}
	schema.Tables = filtered

	// Wire up M2M on both sides.
	for _, jt := range junctions {
		fk0 := jt.ForeignKeys[0]
		fk1 := jt.ForeignKeys[1]

		// Collect extra columns (non-PK).
		pkSet := map[string]bool{}
		for _, pk := range jt.PrimaryKeys {
			pkSet[pk.Name] = true
		}
		var extraCols []*ir.Column
		for _, c := range jt.Columns {
			if !pkSet[c.Name] {
				extraCols = append(extraCols, c)
			}
		}

		fk0.TargetTable.ManyToMany = append(fk0.TargetTable.ManyToMany, &ir.ManyToMany{
			JunctionTable:     jt.Name,
			JunctionSourceCol: fk0.SourceColumn.Name,
			JunctionTargetCol: fk1.SourceColumn.Name,
			TargetTable:       fk1.TargetTable,
			ExtraColumns:      extraCols,
		})
		fk1.TargetTable.ManyToMany = append(fk1.TargetTable.ManyToMany, &ir.ManyToMany{
			JunctionTable:     jt.Name,
			JunctionSourceCol: fk1.SourceColumn.Name,
			JunctionTargetCol: fk0.SourceColumn.Name,
			TargetTable:       fk0.TargetTable,
			ExtraColumns:      extraCols,
		})

		// Clean up: remove junction FKs from target tables' ReferencedBy.
		for _, target := range []*ir.Table{fk0.TargetTable, fk1.TargetTable} {
			var kept []*ir.ForeignKey
			for _, ref := range target.ReferencedBy {
				if ref.SourceTable.Name != jt.Name {
					kept = append(kept, ref)
				}
			}
			target.ReferencedBy = kept
		}
	}
}

func convertTable[C, I any](dt drivers.Table[C, I]) *ir.Table {
	t := &ir.Table{
		Name:      dt.Name,
		ColumnMap: map[string]*ir.Column{},
	}

	pkCols := map[string]bool{}
	if dt.Constraints.Primary != nil {
		for _, col := range dt.Constraints.Primary.Columns {
			pkCols[col] = true
		}
	}
	uniqueCols := map[string]bool{}
	for _, u := range dt.Constraints.Uniques {
		if len(u.Columns) == 1 {
			uniqueCols[u.Columns[0]] = true
		}
	}

	for i, dc := range dt.Columns {
		col := extractColumn(dc)
		if col == nil {
			continue
		}
		goType := GoTypeFromString(col.Type)
		// Ensure nullable columns use pointer types even if bob's
		// Type field doesn't include the null.Val wrapper.
		if col.Nullable && !goType.IsPtr {
			goType = ir.NullableOf(goType)
		}
		c := &ir.Column{
			Name:         col.Name,
			GoType:       goType,
			RawType:      col.DBType,
			Nullable:     col.Nullable,
			IsPrimaryKey: pkCols[col.Name],
			IsUnique:     uniqueCols[col.Name],
			HasDefault:   col.Default != "",
			DefaultValue: col.Default,
			Ordinal:      i,
		}
		if m := varcharLenRe.FindStringSubmatch(col.DBType); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				c.MaxLength = &n
			}
		}
		t.Columns = append(t.Columns, c)
		t.ColumnMap[c.Name] = c
		if c.IsPrimaryKey {
			t.PrimaryKeys = append(t.PrimaryKeys, c)
		}
	}

	if !t.HasPK() {
		return nil
	}
	return t
}

type columnData struct {
	Name, DBType, Default, Type string
	Nullable                    bool
}

func extractColumn(dc any) *columnData {
	if c, ok := dc.(drivers.Column); ok {
		return &columnData{
			Name: c.Name, DBType: c.DBType, Default: c.Default,
			Type: c.Type, Nullable: c.Nullable,
		}
	}
	return nil
}

// GoTypeFromString maps a Go type string to an ir.GoType.
func GoTypeFromString(s string) ir.GoType {
	if strings.HasPrefix(s, "null.Val[") || strings.HasPrefix(s, "omitnull.Val[") {
		inner := s[strings.Index(s, "[")+1 : len(s)-1]
		return ir.NullableOf(GoTypeFromString(inner))
	}
	if strings.HasPrefix(s, "[]") {
		switch s[2:] {
		case "byte":
			return ir.GoTypeByteSlice
		case "string":
			return ir.GoTypeStringArr
		case "int32":
			return ir.GoTypeInt32Arr
		case "int64":
			return ir.GoTypeInt64Arr
		case "float64":
			return ir.GoTypeFloat64Arr
		case "bool":
			return ir.GoTypeBoolArr
		case "uuid.UUID":
			return ir.GoTypeUUIDArr
		}
	}
	switch s {
	case "string":
		return ir.GoTypeString
	case "int32":
		return ir.GoTypeInt32
	case "int64", "int":
		return ir.GoTypeInt64
	case "float64", "float32":
		return ir.GoTypeFloat64
	case "bool":
		return ir.GoTypeBool
	case "time.Time":
		return ir.GoTypeTime
	case "uuid.UUID":
		return ir.GoTypeUUID
	case "json.RawMessage":
		return ir.GoTypeJSON
	case "[]byte":
		return ir.GoTypeByteSlice
	default:
		return ir.GoTypeString
	}
}

// ExtractINValues parses enum values from a CHECK expression.
func ExtractINValues(expr string) []string {
	re := regexp.MustCompile(`'([^']+?)'`)
	matches := re.FindAllStringSubmatch(expr, -1)
	if len(matches) == 0 {
		return nil
	}
	var vals []string
	for _, m := range matches {
		v := m[1]
		if idx := strings.Index(v, "::"); idx > 0 {
			v = v[:idx]
		}
		vals = append(vals, v)
	}
	return vals
}

// DriverFromString maps a config driver string to ir.Driver.
func DriverFromString(driver string) ir.Driver {
	switch driver {
	case "mysql":
		return ir.DriverMySQL
	case "sqlite":
		return ir.DriverSQLite
	default:
		return ir.DriverPostgres
	}
}
