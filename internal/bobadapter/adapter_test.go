package bobadapter

import (
	"testing"

	"github.com/fisayoafolayan/kiln/internal/ir"
	"github.com/stephenafamo/bob/gen/drivers"
)

func TestGoTypeFromString(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"string", "string"},
		{"int32", "int32"},
		{"int64", "int64"},
		{"int", "int64"},
		{"float64", "float64"},
		{"bool", "bool"},
		{"time.Time", "time.Time"},
		{"uuid.UUID", "uuid.UUID"},
		{"[]byte", "[]byte"},
		{"json.RawMessage", "json.RawMessage"},
		{"[]string", "[]string"},
		{"[]int64", "[]int64"},
		{"[]uuid.UUID", "[]uuid.UUID"},
		{"null.Val[string]", "*string"},
		{"null.Val[time.Time]", "*time.Time"},
		{"omitnull.Val[uuid.UUID]", "*uuid.UUID"},
		{"unknown_type", "string"},
	}

	for _, c := range cases {
		got := GoTypeFromString(c.input)
		if got.String() != c.want {
			t.Errorf("GoTypeFromString(%q) = %q, want %q", c.input, got.String(), c.want)
		}
	}
}

func TestExtractINValues(t *testing.T) {
	cases := []struct {
		expr string
		want []string
	}{
		{"((role)::text = ANY ((ARRAY['member'::text, 'admin'::text])::text[]))", []string{"member", "admin"}},
		{"(status IN ('draft', 'published', 'archived'))", []string{"draft", "published", "archived"}},
		{"(`role` in ('member','moderator','admin'))", []string{"member", "moderator", "admin"}},
		{"(status > 0)", nil},
	}

	for _, c := range cases {
		got := ExtractINValues(c.expr)
		if len(got) != len(c.want) {
			t.Errorf("ExtractINValues(%q) = %v, want %v", c.expr, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ExtractINValues(%q)[%d] = %q, want %q", c.expr, i, got[i], c.want[i])
			}
		}
	}
}

func TestConvertDBInfo(t *testing.T) {
	info := &drivers.DBInfo[any, any, any]{
		Tables: drivers.Tables[any, any]{
			{
				Key:  "users",
				Name: "users",
				Columns: []drivers.Column{
					{Name: "id", DBType: "uuid", Type: "uuid.UUID", Default: "gen_random_uuid()"},
					{Name: "email", DBType: "varchar(255)", Type: "string"},
					{Name: "name", DBType: "text", Type: "string"},
					{Name: "role", DBType: "text", Type: "string", Default: "'member'::text"},
					{Name: "bio", DBType: "text", Type: "null.Val[string]", Nullable: true},
				},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"id"}},
					Uniques: []drivers.Constraint[any]{{Columns: []string{"email"}}},
					Checks: []drivers.Check[any]{
						{
							Constraint: drivers.Constraint[any]{Columns: []string{"role"}},
							Expression: "((role)::text = ANY ((ARRAY['member'::text, 'admin'::text])::text[]))",
						},
					},
				},
			},
		},
		Driver: "psql",
	}

	schema := ConvertDBInfo(info, ir.DriverPostgres, nil)

	if len(schema.Tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(schema.Tables))
	}

	users := schema.TableMap["users"]
	if !users.HasPK() || users.PrimaryKeys[0].Name != "id" {
		t.Fatal("PrimaryKey should be id")
	}

	email := users.ColumnMap["email"]
	if email.MaxLength == nil || *email.MaxLength != 255 {
		t.Errorf("email MaxLength = %v, want 255", email.MaxLength)
	}
	if !email.IsUnique {
		t.Error("email should be unique")
	}

	role := users.ColumnMap["role"]
	if len(role.EnumValues) != 2 || role.EnumValues[0] != "member" {
		t.Errorf("role EnumValues = %v, want [member admin]", role.EnumValues)
	}
}

func TestConvertDBInfo_CompositePKIncluded(t *testing.T) {
	// A composite PK table without matching junction pattern should be included.
	info := &drivers.DBInfo[any, any, any]{
		Tables: drivers.Tables[any, any]{
			{
				Key:  "tenant_users",
				Name: "tenant_users",
				Columns: []drivers.Column{
					{Name: "tenant_id", DBType: "uuid", Type: "uuid.UUID"},
					{Name: "user_id", DBType: "uuid", Type: "uuid.UUID"},
					{Name: "role", DBType: "text", Type: "string", Default: "'member'"},
				},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"tenant_id", "user_id"}},
				},
			},
		},
	}

	schema := ConvertDBInfo(info, ir.DriverPostgres, nil)
	if len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(schema.Tables))
	}
	tu := schema.TableMap["tenant_users"]
	if len(tu.PrimaryKeys) != 2 {
		t.Fatalf("PrimaryKeys = %d, want 2", len(tu.PrimaryKeys))
	}
	if tu.PrimaryKeys[0].Name != "tenant_id" {
		t.Errorf("PK[0] = %q, want tenant_id", tu.PrimaryKeys[0].Name)
	}
	if tu.PrimaryKeys[1].Name != "user_id" {
		t.Errorf("PK[1] = %q, want user_id", tu.PrimaryKeys[1].Name)
	}
}

func TestConvertDBInfo_DetectsM2M(t *testing.T) {
	info := &drivers.DBInfo[any, any, any]{
		Tables: drivers.Tables[any, any]{
			{
				Key:     "posts",
				Name:    "posts",
				Columns: []drivers.Column{{Name: "id", DBType: "uuid", Type: "uuid.UUID", Default: "gen_random_uuid()"}},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"id"}},
				},
			},
			{
				Key:     "tags",
				Name:    "tags",
				Columns: []drivers.Column{{Name: "id", DBType: "uuid", Type: "uuid.UUID", Default: "gen_random_uuid()"}},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"id"}},
				},
			},
			{
				Key:  "post_tags",
				Name: "post_tags",
				Columns: []drivers.Column{
					{Name: "post_id", DBType: "uuid", Type: "uuid.UUID"},
					{Name: "tag_id", DBType: "uuid", Type: "uuid.UUID"},
				},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"post_id", "tag_id"}},
					Foreign: []drivers.ForeignKey[any]{
						{
							Constraint:     drivers.Constraint[any]{Columns: []string{"post_id"}},
							ForeignTable:   "posts",
							ForeignColumns: []string{"id"},
						},
						{
							Constraint:     drivers.Constraint[any]{Columns: []string{"tag_id"}},
							ForeignTable:   "tags",
							ForeignColumns: []string{"id"},
						},
					},
				},
			},
		},
	}

	schema := ConvertDBInfo(info, ir.DriverPostgres, nil)

	// Junction table should NOT be in schema.Tables.
	if len(schema.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(schema.Tables))
	}
	if _, ok := schema.TableMap["post_tags"]; ok {
		t.Fatal("post_tags should not be in TableMap")
	}

	// Both sides should have ManyToMany populated.
	posts := schema.TableMap["posts"]
	if len(posts.ManyToMany) != 1 {
		t.Fatalf("posts M2M = %d, want 1", len(posts.ManyToMany))
	}
	m2m := posts.ManyToMany[0]
	if m2m.JunctionTable != "post_tags" {
		t.Errorf("junction table = %q, want post_tags", m2m.JunctionTable)
	}
	if m2m.JunctionSourceCol != "post_id" {
		t.Errorf("source col = %q, want post_id", m2m.JunctionSourceCol)
	}
	if m2m.JunctionTargetCol != "tag_id" {
		t.Errorf("target col = %q, want tag_id", m2m.JunctionTargetCol)
	}
	if m2m.TargetTable.Name != "tags" {
		t.Errorf("target table = %q, want tags", m2m.TargetTable.Name)
	}

	tags := schema.TableMap["tags"]
	if len(tags.ManyToMany) != 1 {
		t.Fatalf("tags M2M = %d, want 1", len(tags.ManyToMany))
	}
	if tags.ManyToMany[0].TargetTable.Name != "posts" {
		t.Errorf("tags M2M target = %q, want posts", tags.ManyToMany[0].TargetTable.Name)
	}
}

func TestConvertDBInfo_M2MWithExtraColumns(t *testing.T) {
	info := &drivers.DBInfo[any, any, any]{
		Tables: drivers.Tables[any, any]{
			{
				Key:     "posts",
				Name:    "posts",
				Columns: []drivers.Column{{Name: "id", DBType: "uuid", Type: "uuid.UUID", Default: "gen_random_uuid()"}},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"id"}},
				},
			},
			{
				Key:     "tags",
				Name:    "tags",
				Columns: []drivers.Column{{Name: "id", DBType: "uuid", Type: "uuid.UUID", Default: "gen_random_uuid()"}},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"id"}},
				},
			},
			{
				Key:  "post_tags",
				Name: "post_tags",
				Columns: []drivers.Column{
					{Name: "post_id", DBType: "uuid", Type: "uuid.UUID"},
					{Name: "tag_id", DBType: "uuid", Type: "uuid.UUID"},
					{Name: "created_at", DBType: "timestamptz", Type: "time.Time", Default: "now()"},
				},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"post_id", "tag_id"}},
					Foreign: []drivers.ForeignKey[any]{
						{
							Constraint:     drivers.Constraint[any]{Columns: []string{"post_id"}},
							ForeignTable:   "posts",
							ForeignColumns: []string{"id"},
						},
						{
							Constraint:     drivers.Constraint[any]{Columns: []string{"tag_id"}},
							ForeignTable:   "tags",
							ForeignColumns: []string{"id"},
						},
					},
				},
			},
		},
	}

	schema := ConvertDBInfo(info, ir.DriverPostgres, nil)

	posts := schema.TableMap["posts"]
	if len(posts.ManyToMany) != 1 {
		t.Fatalf("posts M2M = %d, want 1", len(posts.ManyToMany))
	}
	if len(posts.ManyToMany[0].ExtraColumns) != 1 {
		t.Fatalf("extra cols = %d, want 1", len(posts.ManyToMany[0].ExtraColumns))
	}
	if posts.ManyToMany[0].ExtraColumns[0].Name != "created_at" {
		t.Errorf("extra col = %q, want created_at", posts.ManyToMany[0].ExtraColumns[0].Name)
	}
}

func TestConvertDBInfo_M2MSkippedWhenTargetExcluded(t *testing.T) {
	info := &drivers.DBInfo[any, any, any]{
		Tables: drivers.Tables[any, any]{
			{
				Key:     "posts",
				Name:    "posts",
				Columns: []drivers.Column{{Name: "id", DBType: "uuid", Type: "uuid.UUID", Default: "gen_random_uuid()"}},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"id"}},
				},
			},
			{
				Key:     "tags",
				Name:    "tags",
				Columns: []drivers.Column{{Name: "id", DBType: "uuid", Type: "uuid.UUID", Default: "gen_random_uuid()"}},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"id"}},
				},
			},
			{
				Key:  "post_tags",
				Name: "post_tags",
				Columns: []drivers.Column{
					{Name: "post_id", DBType: "uuid", Type: "uuid.UUID"},
					{Name: "tag_id", DBType: "uuid", Type: "uuid.UUID"},
				},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"post_id", "tag_id"}},
					Foreign: []drivers.ForeignKey[any]{
						{
							Constraint:     drivers.Constraint[any]{Columns: []string{"post_id"}},
							ForeignTable:   "posts",
							ForeignColumns: []string{"id"},
						},
						{
							Constraint:     drivers.Constraint[any]{Columns: []string{"tag_id"}},
							ForeignTable:   "tags",
							ForeignColumns: []string{"id"},
						},
					},
				},
			},
		},
	}

	// Exclude tags - M2M should not be detected.
	exclude := map[string]bool{"tags": true}
	schema := ConvertDBInfo(info, ir.DriverPostgres, exclude)

	posts := schema.TableMap["posts"]
	if len(posts.ManyToMany) != 0 {
		t.Errorf("posts M2M = %d, want 0 (tags excluded)", len(posts.ManyToMany))
	}
}

func TestConvertDBInfo_ForeignKeys(t *testing.T) {
	info := &drivers.DBInfo[any, any, any]{
		Tables: drivers.Tables[any, any]{
			{
				Key:     "users",
				Name:    "users",
				Columns: []drivers.Column{{Name: "id", DBType: "uuid", Type: "uuid.UUID", Default: "gen_random_uuid()"}},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"id"}},
				},
			},
			{
				Key:  "posts",
				Name: "posts",
				Columns: []drivers.Column{
					{Name: "id", DBType: "uuid", Type: "uuid.UUID", Default: "gen_random_uuid()"},
					{Name: "user_id", DBType: "uuid", Type: "uuid.UUID"},
				},
				Constraints: drivers.Constraints[any]{
					Primary: &drivers.Constraint[any]{Columns: []string{"id"}},
					Foreign: []drivers.ForeignKey[any]{
						{
							Constraint:     drivers.Constraint[any]{Columns: []string{"user_id"}},
							ForeignTable:   "users",
							ForeignColumns: []string{"id"},
						},
					},
				},
			},
		},
	}

	schema := ConvertDBInfo(info, ir.DriverPostgres, nil)

	posts := schema.TableMap["posts"]
	if len(posts.ForeignKeys) != 1 {
		t.Fatalf("posts FKs = %d, want 1", len(posts.ForeignKeys))
	}
	if posts.ForeignKeys[0].SourceColumn.Name != "user_id" {
		t.Error("FK source should be user_id")
	}
	if len(schema.TableMap["users"].ReferencedBy) != 1 {
		t.Error("users should be referenced by posts")
	}
}
