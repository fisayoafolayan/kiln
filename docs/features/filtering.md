# Filtering & Sorting

List endpoints support filtering via query parameters and sorting via the `sort` parameter.

## Filtering

```bash
# Exact match
GET /api/v1/users?role=admin

# Not equal
GET /api/v1/users?role[neq]=admin

# Range operators (numeric and timestamp columns)
GET /api/v1/users?created_at[gte]=2024-01-01T00:00:00Z
GET /api/v1/users?created_at[lt]=2025-01-01T00:00:00Z

# Combine filters
GET /api/v1/users?role=admin&created_at[gte]=2024-01-01T00:00:00Z
```

### Supported Operators

| Operator | Meaning | Available For |
|----------|---------|---------------|
| (default) | Equal | All types |
| `neq` | Not equal | All types |
| `gt` | Greater than | Numeric, timestamp |
| `gte` | Greater than or equal | Numeric, timestamp |
| `lt` | Less than | Numeric, timestamp |
| `lte` | Less than or equal | Numeric, timestamp |

### Restricting Filterable Columns

By default, all non-hidden columns are filterable. **For production APIs, you
should lock this down** - unrestricted filtering can expose internal fields and
hit unindexed columns. Restrict with an allowlist:

```yaml
overrides:
  users:
    filterable_fields:
      - email
      - role
      - created_at
```

Or disable filtering entirely:

```yaml
overrides:
  users:
    disable_filters: true
```

## Sorting

```bash
# Sort ascending
GET /api/v1/users?sort=created_at

# Sort descending (prefix with -)
GET /api/v1/users?sort=-created_at
```

### Restricting Sortable Columns

```yaml
overrides:
  users:
    sortable_fields:
      - created_at
      - name
```

## Pagination

All list endpoints are paginated:

```bash
GET /api/v1/users?page=2&page_size=10
```

Response includes total count:

```json
{
  "data": [...],
  "total": 42,
  "page": 2,
  "page_size": 10
}
```

Default page size is 20. Maximum is 100.
