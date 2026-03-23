package ir

import (
	"strings"
	"unicode"

	"github.com/gobuffalo/flect"
)

// toPascalCase converts snake_case to PascalCase singular.
// "user_profiles" → "UserProfile", "categories" → "Category"
func toPascalCase(s string) string {
	parts := splitWords(s)
	result := make([]string, len(parts))
	for i, p := range parts {
		if i == len(parts)-1 {
			p = flect.Singularize(p)
		}
		if len(p) == 0 {
			continue
		}
		result[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(result, "")
}

// pluralize returns the plural form of a word using flect.
func pluralize(s string) string {
	return flect.Pluralize(s)
}

// toKebabCase converts snake_case to kebab-case.
// "user_profiles" → "user-profiles"
func toKebabCase(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

// toSnakeCase converts PascalCase or camelCase to snake_case.
// "UserProfile" → "user_profile"
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteRune('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// splitWords splits a snake_case or kebab-case string into words.
func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
}

// commonInitialisms is the set of well-known Go initialisms.
var commonInitialisms = map[string]bool{
	"ACL": true, "API": true, "ASCII": true, "CPU": true,
	"CSS": true, "DB": true, "DNS": true, "EOF": true,
	"GUID": true, "HTML": true, "HTTP": true, "HTTPS": true,
	"ID": true, "IP": true, "JSON": true, "LHS": true,
	"QPS": true, "RAM": true, "RHS": true, "RPC": true,
	"SLA": true, "SMTP": true, "SQL": true, "SSH": true,
	"TCP": true, "TLS": true, "TTL": true, "UDP": true,
	"UI": true, "UID": true, "UUID": true, "URI": true,
	"URL": true, "UTF8": true, "VM": true, "XML": true,
	"XMPP": true, "XSRF": true, "XSS": true,
}

// toGoFieldName converts a snake_case column name to an idiomatic Go field name,
// respecting common initialisms.
// "user_id" → "UserID", "api_key" → "APIKey"
func toGoFieldName(s string) string {
	parts := splitWords(s)
	for i, p := range parts {
		upper := strings.ToUpper(p)
		if commonInitialisms[upper] {
			parts[i] = upper
		} else if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, "")
}
