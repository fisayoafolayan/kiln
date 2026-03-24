# Schema Evolution

This is where kiln differs from one-time scaffolding tools. Your schema changes over time - kiln keeps your API in sync.

## The Workflow

```
1. Change your schema (add column, rename field, add table)
2. Migrate the database (goose, atlas, golang-migrate, raw SQL)
3. Run: kiln generate
4. Done. API matches the new schema.
```

## What Happens on Regenerate

**Auto-generated files** are updated to match the new schema. Each file has an embedded checksum. If you've edited one, kiln protects your changes:

```
  ⚠ SKIPPED  generated/store/users.go (user-modified; use --force to overwrite)
  ✓ generated/models/users.go
  ✓ generated/handlers/users.go
  ✓ generated/router.go
  ✓ docs/openapi.yaml
```

**Write-once files** (mappers, helpers, main.go) are never touched. You update those manually when needed.

## Tools for Safe Evolution

| Command | Use when |
|---------|----------|
| `kiln diff` | Preview what would change before writing |
| `kiln generate` | Regenerate, skip edited files |
| `kiln generate --force` | Overwrite everything, including edits |
| `kiln generate --table users` | Regenerate only one table |

## Why This Matters

Without kiln, a schema change means manually updating structs, handlers, validation, filters, OpenAPI spec - hoping nothing falls out of sync. With kiln, the schema is the single source of truth. Change it, regenerate, and the API is correct by construction.

## Common Scenarios

### Added a new column

`kiln generate` updates models, store, handlers, and OpenAPI automatically. If you've customized a mapper, add the new field manually.

### Added a new table

`kiln generate` creates all new files. The router is updated. `cmd/server/main.go` is write-once so you'll need to add the new store and handler wiring manually.

### Renamed a column

`kiln generate` updates the generated files. Your mapper needs manual update. If you've edited store or handler files, use `kiln diff` to see what changed, then apply manually or use `--force`.

### Deleted a table

`kiln generate` won't delete old files - it only generates for tables that exist. Remove the old generated files manually.

### Changed a column type

`kiln generate` updates the Go types, store queries, and validation tags. If you've edited handlers or mappers that reference the column, check them manually.

### Added a CHECK constraint (enum)

kiln auto-detects the new constraint and adds `oneof=` validation. No config needed.

### Added deleted_at column

kiln auto-detects the soft delete column and switches from hard delete to `SET deleted_at = now()`. No config needed.
