package ir

import "testing"

// ─────────────────────────────────────────────────────────────────────────────
// toPascalCase
// ─────────────────────────────────────────────────────────────────────────────

func TestToPascalCase(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// Regular tables
		{"users", "User"},
		{"posts", "Post"},
		{"tags", "Tag"},
		{"comments", "Comment"},

		// Irregular plurals
		{"categories", "Category"},
		{"statuses", "Status"},
		{"analyses", "Analysis"},
		{"people", "Person"},
		{"children", "Child"},

		// Multi-word snake_case
		{"user_profiles", "UserProfile"},
		{"blog_posts", "BlogPost"},
		{"api_keys", "ApiKey"},

		// Already singular
		{"user", "User"},
		{"post", "Post"},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := toPascalCase(c.input)
			if got != c.want {
				t.Errorf("toPascalCase(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// toGoFieldName
// ─────────────────────────────────────────────────────────────────────────────

func TestToGoFieldName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// Common initialisms
		{"id", "ID"},
		{"user_id", "UserID"},
		{"post_id", "PostID"},
		{"api_key", "APIKey"},
		{"html_url", "HTMLURL"},
		{"created_at", "CreatedAt"},
		{"updated_at", "UpdatedAt"},

		// Regular fields
		{"email", "Email"},
		{"name", "Name"},
		{"body", "Body"},
		{"role", "Role"},
		{"status", "Status"},
		{"bio", "Bio"},

		// Multi-word
		{"published_at", "PublishedAt"},
		{"first_name", "FirstName"},
		{"last_name", "LastName"},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := toGoFieldName(c.input)
			if got != c.want {
				t.Errorf("toGoFieldName(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// toSnakeCase
// ─────────────────────────────────────────────────────────────────────────────

func TestToSnakeCase(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"User", "user"},
		{"UserProfile", "user_profile"},
		{"CreatedAt", "created_at"},
		{"APIKey", "a_p_i_key"},
		{"ID", "i_d"},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := toSnakeCase(c.input)
			if got != c.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// toKebabCase
// ─────────────────────────────────────────────────────────────────────────────

func TestToKebabCase(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"users", "users"},
		{"user_profiles", "user-profiles"},
		{"blog_posts", "blog-posts"},
		{"api_keys", "api-keys"},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := toKebabCase(c.input)
			if got != c.want {
				t.Errorf("toKebabCase(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Table.GoName and Table.GoNamePlural
// ─────────────────────────────────────────────────────────────────────────────

func TestTableGoName(t *testing.T) {
	cases := []struct {
		tableName  string
		wantSingle string
		wantPlural string
	}{
		{"users", "User", "Users"},
		{"posts", "Post", "Posts"},
		{"tags", "Tag", "Tags"},
		{"comments", "Comment", "Comments"},
		{"categories", "Category", "Categories"},
		{"user_profiles", "UserProfile", "UserProfiles"},
	}

	for _, c := range cases {
		t.Run(c.tableName, func(t *testing.T) {
			table := &Table{Name: c.tableName}

			if got := table.GoName(); got != c.wantSingle {
				t.Errorf("GoName(%q) = %q, want %q", c.tableName, got, c.wantSingle)
			}
			if got := table.GoNamePlural(); got != c.wantPlural {
				t.Errorf("GoNamePlural(%q) = %q, want %q", c.tableName, got, c.wantPlural)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Table.Endpoint
// ─────────────────────────────────────────────────────────────────────────────

func TestTableEndpoint(t *testing.T) {
	cases := []struct {
		tableName string
		want      string
	}{
		{"users", "users"},
		{"user_profiles", "user-profiles"},
		{"blog_posts", "blog-posts"},
		{"tags", "tags"},
	}

	for _, c := range cases {
		t.Run(c.tableName, func(t *testing.T) {
			table := &Table{Name: c.tableName}
			got := table.Endpoint()
			if got != c.want {
				t.Errorf("Endpoint(%q) = %q, want %q", c.tableName, got, c.want)
			}
		})
	}
}
