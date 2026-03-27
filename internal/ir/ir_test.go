package ir

import "testing"

func TestColumnIsReadOnly(t *testing.T) {
	cases := []struct {
		name string
		col  Column
		want bool
	}{
		{
			"PK with default",
			Column{IsPrimaryKey: true, HasDefault: true, GoType: GoTypeInt64},
			true,
		},
		{
			"PK without default",
			Column{IsPrimaryKey: true, HasDefault: false, GoType: GoTypeInt64},
			false,
		},
		{
			"timestamp with default",
			Column{Name: "created_at", GoType: GoTypeTime, HasDefault: true},
			true,
		},
		{
			"well-known timestamp name",
			Column{Name: "updated_at", GoType: GoTypeTime},
			true,
		},
		{
			"deleted_at timestamp",
			Column{Name: "deleted_at", GoType: GoTypeTime},
			true,
		},
		{
			"custom timestamp not auto-managed",
			Column{Name: "published_at", GoType: GoTypeTime},
			false,
		},
		{
			"regular field",
			Column{Name: "title", GoType: GoTypeString},
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.col.IsReadOnly(); got != c.want {
				t.Errorf("IsReadOnly() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestValidationTag(t *testing.T) {
	maxLen := 255
	cases := []struct {
		name string
		col  Column
		want string
	}{
		{
			"required string",
			Column{Name: "title", GoType: GoTypeString},
			"required",
		},
		{
			"required string with max length",
			Column{Name: "title", GoType: GoTypeString, MaxLength: &maxLen},
			"required,max=255",
		},
		{
			"nullable field - not required",
			Column{Name: "bio", GoType: GoTypeString, Nullable: true},
			"",
		},
		{
			"field with default - not required",
			Column{Name: "role", GoType: GoTypeString, HasDefault: true},
			"",
		},
		{
			"PK - not required",
			Column{Name: "id", GoType: GoTypeInt64, IsPrimaryKey: true},
			"",
		},
		{
			"enum values",
			Column{Name: "status", GoType: GoTypeString, EnumValues: []string{"active", "inactive"}},
			"required,oneof=active inactive",
		},
		{
			"nullable with enum",
			Column{Name: "status", GoType: GoTypeString, Nullable: true, EnumValues: []string{"a", "b"}},
			"oneof=a b",
		},
		{
			"int field required",
			Column{Name: "age", GoType: GoTypeInt32},
			"required",
		},
		{
			"string with max only (nullable)",
			Column{Name: "bio", GoType: GoTypeString, Nullable: true, MaxLength: &maxLen},
			"max=255",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.col.ValidationTag(); got != c.want {
				t.Errorf("ValidationTag() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestUpdateValidationTag(t *testing.T) {
	maxLen := 100
	cases := []struct {
		name string
		col  Column
		want string
	}{
		{
			"no rules",
			Column{Name: "title", GoType: GoTypeString},
			"",
		},
		{
			"max length",
			Column{Name: "title", GoType: GoTypeString, MaxLength: &maxLen},
			"omitempty,max=100",
		},
		{
			"enum values",
			Column{Name: "status", GoType: GoTypeString, EnumValues: []string{"active", "inactive"}},
			"omitempty,oneof=active inactive",
		},
		{
			"enum + max",
			Column{Name: "status", GoType: GoTypeString, EnumValues: []string{"a", "b"}, MaxLength: &maxLen},
			"omitempty,oneof=a b,max=100",
		},
		{
			"int field - no rules on update",
			Column{Name: "age", GoType: GoTypeInt32},
			"",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.col.UpdateValidationTag(); got != c.want {
				t.Errorf("UpdateValidationTag() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestGoTypeIsFilterable(t *testing.T) {
	filterable := []GoType{GoTypeString, GoTypeUUID, GoTypeInt32, GoTypeInt64, GoTypeFloat64, GoTypeBool, GoTypeTime}
	for _, gt := range filterable {
		if !gt.IsFilterable() {
			t.Errorf("expected IsFilterable() = true for %s", gt.Name)
		}
	}

	notFilterable := []GoType{GoTypeByteSlice, GoTypeJSON, GoTypeStringArr, GoTypeInt64Arr}
	for _, gt := range notFilterable {
		if gt.IsFilterable() {
			t.Errorf("expected IsFilterable() = false for %s", gt.Name)
		}
	}
}

func TestGoTypeSupportsRangeOps(t *testing.T) {
	supports := []GoType{GoTypeInt32, GoTypeInt64, GoTypeFloat64, GoTypeTime}
	for _, gt := range supports {
		if !gt.SupportsRangeOps() {
			t.Errorf("expected SupportsRangeOps() = true for %s", gt.Name)
		}
	}

	doesNot := []GoType{GoTypeString, GoTypeBool, GoTypeUUID}
	for _, gt := range doesNot {
		if gt.SupportsRangeOps() {
			t.Errorf("expected SupportsRangeOps() = false for %s", gt.Name)
		}
	}
}
