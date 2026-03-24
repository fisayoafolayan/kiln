# kiln

**Compile your database schema into a production-ready Go API.**

Eliminate schema drift - your API always matches your schema. Zero runtime dependency. No framework lock-in. Delete kiln - your code still compiles.

## The Problem

You build an API by hand. Structs, validation, handlers, router, OpenAPI spec.
It works. Then the schema changes. A column is renamed. A constraint is added.
Your API doesn't match anymore. Tests pass. CI is green. Production breaks.

Because your API no longer matches your schema. This is **schema drift**.

kiln eliminates it. One command generates your entire API layer from
the database schema. Schema changes? Regenerate. Your API is correct
by construction - not by convention or tests.

Think of kiln like a compiler: the schema is your source, `kiln generate`
is the compile step, and the output is a working API. Change the schema, recompile.

## Quick Example

```sql
CREATE TABLE users (
  id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL UNIQUE,
  name  TEXT NOT NULL,
  role  TEXT NOT NULL DEFAULT 'member'
    CHECK (role IN ('member', 'moderator', 'admin'))
);

CREATE TABLE posts (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title      TEXT NOT NULL,
  status     TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft', 'published', 'archived')),
  deleted_at TIMESTAMPTZ  -- nullable = soft delete enabled
);
```

```bash
kiln init
kiln generate
go run cmd/server/main.go
```

You immediately get:

- `GET /api/v1/users` - list with pagination, filtering & sorting
- `POST /api/v1/users` - create with validation (enum-checked `role`)
- `GET /api/v1/users/{id}` - get by ID
- `PATCH /api/v1/users/{id}` - partial update
- `DELETE /api/v1/users/{id}` - delete
- `GET /api/v1/posts` - list posts (soft-deleted posts excluded automatically)
- `DELETE /api/v1/posts/{id}` - soft-delete (sets `deleted_at`, doesn't remove)
- `GET /api/v1/users/{id}/posts` - nested route from FK relationship
- OpenAPI spec at `docs/openapi.yaml`

Relationships in your database automatically become API routes. All with zero runtime dependency on kiln. You own the output.

## Why kiln?

| | sqlc | ent | kiln |
|---|------|-----|------|
| Input | Hand-written SQL | Go schema DSL | Database schema |
| Output | Types + query functions | Runtime ORM | Types + store + handlers + router + OpenAPI |
| You write | SQL + handlers + router | Schema + handlers | kiln.yaml |
| Runtime dependency | None | ent runtime | None |

kiln is the only tool that goes from database schema to **runnable REST API** in one command, where the output is plain Go you can fork and forget.

### What kiln is not

- **Not an ORM** - no runtime query layer
- **Not a framework** - no runtime control, no hidden magic
- **Not one-shot scaffolding** - regenerate safely as your schema evolves

kiln is a compiler for APIs. Schema in, Go code out. Delete kiln and the code still compiles.

### Where kiln is not a good fit

kiln works best for CRUD-heavy APIs. It may not be the right tool if:

- Your API is workflow-driven (payments, state machines, multi-step processes)
- You rely on complex joins or hand-tuned SQL queries
- Your schema is not the source of truth for your domain

For these cases, kiln can still generate the boilerplate layers (models, OpenAPI) while you write the rest by hand.

## Schema Evolution

This is where kiln differs from one-time scaffolding. Your schema changes over time - kiln keeps your API in sync:

```
1. Change your schema
2. Run: kiln generate
3. Done. API matches the new schema.
```

Edited files are protected by checksums. Write-once files are never touched. See [Schema Evolution](guides/schema-changes.md) for the full workflow.

## Next Steps

- [Getting Started](getting-started.md) - install and generate your first API
- [Configuration](configuration.md) - customize what kiln generates
- [Schema Evolution](guides/schema-changes.md) - how kiln handles schema changes
- [Example Project](https://github.com/fisayoafolayan/kiln/tree/main/examples/blog-api) - a complete blog API you can clone and run
