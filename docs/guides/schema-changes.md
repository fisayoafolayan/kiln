# Schema Changes

When your database schema changes, kiln regenerates the affected code while protecting your edits.

## Workflow

1. **Migrate your database** using your preferred tool (goose, atlas, golang-migrate, or raw SQL)
2. **Run `kiln generate`** to regenerate code from the updated schema
3. **Update write-once files** manually if needed (mappers, helpers, main.go)

## What Happens on Regenerate

**Auto-generated files** (models, store, handlers, router, OpenAPI) are regenerated. Each file has an embedded checksum. If you've edited a file, kiln detects it and skips:

```
  ⚠ SKIPPED  generated/store/users.go (user-modified; use --force to overwrite)
  ✓ generated/models/users.go
  ✓ generated/handlers/users.go
  ✓ generated/router.go
```

**Write-once files** (mappers, helpers, main.go, auth middleware) are never touched.

## Commands

| Command | Effect |
|---------|--------|
| `kiln generate` | Regenerate, skip edited files |
| `kiln generate --force` | Regenerate everything, overwrite edits |
| `kiln diff` | Show what would be generated without writing |
| `kiln generate --table users` | Regenerate only one table |

## Common Scenarios

### Added a new column

`kiln generate` updates models, store, handlers, and OpenAPI automatically. If you've customized a mapper, add the new field manually.

### Added a new table

`kiln generate` creates all new files. The router is updated. `cmd/server/main.go` is write-once so you'll need to add the new store and handler wiring manually.

### Renamed a column

`kiln generate` updates the generated files. Your mapper needs manual update. If you've edited store or handler files, use `kiln diff` to see what changed, then apply manually or use `--force`.

### Deleted a table

`kiln generate` won't delete old files — it only generates for tables that exist. Remove the old generated files manually.
