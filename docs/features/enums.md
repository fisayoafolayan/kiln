# Enum Validation

kiln validates string columns against a set of allowed values, generating `oneof=` validation tags.

## Auto-Detection

If your schema uses `CHECK` constraints, kiln detects allowed values automatically:

```sql
CREATE TABLE posts (
  status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft', 'published', 'archived'))
);
```

This generates:

```go
type CreatePostRequest struct {
    Status string `json:"status" validate:"required,oneof=draft published archived"`
}
```

No config needed.

## Config Override

For columns without CHECK constraints, specify values in `kiln.yaml`:

```yaml
overrides:
  users:
    enums:
      role: [member, moderator, admin]
  posts:
    enums:
      status: [draft, published, archived]
```

Config values always take precedence over auto-detected constraints.

## Error Response

Invalid values return a structured error:

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "name": "Test", "role": "superadmin"}'
```

```json
{
  "error": "validation failed",
  "fields": {
    "role": "must be one of: member moderator admin"
  }
}
```

## Update Requests

On PATCH requests, enum fields are optional but still validated when provided:

```go
type UpdatePostRequest struct {
    Status *string `json:"status,omitempty" validate:"omitempty,oneof=draft published archived"`
}
```

Omitting the field is fine. Sending an invalid value is rejected.
