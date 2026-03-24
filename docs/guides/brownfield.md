# Brownfield Adoption

kiln is designed for incremental adoption. You don't have to generate everything - adopt one layer at a time.

## Toggle Layers

```yaml
generate:
  models: true      # just the structs to start
  store: false      # keep your own DB layer
  handlers: false   # keep your own handlers
  router: false     # keep your own router
  openapi: true     # free OpenAPI spec
```

Start with `models: true` to get validated request/response types. Add layers as you gain confidence.

## Common Adoption Paths

### "I just want types"

```yaml
generate:
  models: true
  store: false
  handlers: false
  router: false
  openapi: true
```

You get `generated/models/` with validated structs and `docs/openapi.yaml`. Write your own store and handlers using kiln's types.

### "I want types and store, but my own handlers"

```yaml
generate:
  models: true
  store: true
  handlers: false
  router: false
  openapi: true
```

You get type-safe store methods. Write handlers that call them.

### "Generate everything for new tables, keep my existing ones"

```yaml
tables:
  include:
    - new_table_1
    - new_table_2
```

Or exclude existing tables:

```yaml
tables:
  exclude:
    - legacy_users
    - legacy_orders
```

## Write-Once Files

These files are generated once, then yours forever:

| File | Purpose |
|------|---------|
| `generated/store/mappers/*.go` | Map bob models to response types |
| `generated/handlers/helpers.go` | Error formatting, pagination, validation |
| `generated/auth/middleware.go` | Auth middleware (if enabled) |
| `cmd/server/main.go` | Server entry point and wiring |

Add computed fields in mappers, custom error formats in helpers, auth logic in middleware. kiln will never overwrite them.

## Regeneration Safety

Auto-generated files have embedded checksums. If you edit one, kiln skips it:

```
  ⚠ SKIPPED  generated/store/users.go (user-modified; use --force to overwrite)
```

Use `kiln generate --force` to overwrite anyway, or `kiln diff` to preview changes.
