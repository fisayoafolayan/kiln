# kiln

Turn your database schema into a complete Go HTTP API - models, validation,
handlers, routing, and OpenAPI.

The generated code uses [bob](https://github.com/stephenafamo/bob) (a query
builder) for database access. There is no runtime dependency on kiln - you
can remove kiln after generation and continue using the code as a normal Go
project.

## The Problem

You build an API by hand. Structs, validation, handlers, router, OpenAPI spec.
It works. Then the schema changes. A column is renamed. A constraint is added.
Your API doesn't match anymore. Tests pass. CI is green. Production breaks.

This is **schema drift**. kiln eliminates it. One command regenerates your
entire API layer from the database schema. Think of kiln as a compiler: the
schema is the input, and the generated API is the output.

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

Relationships in your database automatically become API routes. No runtime dependency on kiln. You own the output.

## Why kiln?

Stop writing by hand: CRUD handlers, request/response structs, validation
tags, pagination, filtering, sorting, OpenAPI specs, route wiring. kiln
generates all of it from your schema - and regenerates safely as your schema
evolves.

| | sqlc | ent | kiln |
|---|------|-----|------|
| Input | Hand-written SQL | Go schema DSL | Database schema |
| Output | Types + query functions | Runtime ORM | Types + store + handlers + router + OpenAPI |
| You write | SQL + handlers + router | Schema + handlers | kiln.yaml |
| Runtime dependency | None | ent runtime | bob (query builder) |

It does not generate business logic, workflows, or cross-table invariants.
Structural API layer only.

### Perfect for

- Internal tools and admin APIs
- CRUD-heavy SaaS backends
- B2B APIs where schema drives the domain

### Not ideal for

- Heavy domain logic (payments, approval workflows, state machines)
- Event-driven architectures (Kafka, queues, async pipelines)
- APIs where endpoints don't map cleanly to tables

### Handlers are optional

Most teams generate models, store, and OpenAPI - handlers are optional. In
practice, most non-trivial tables outgrow generated handlers quickly as they
pick up conditional writes, cross-table invariants, or side effects. Generate
handlers for the genuinely simple CRUD tables, write your own for everything
else.

## Schema Evolution

Keep the generated API in sync with database changes by regenerating.

```
1. Update your schema (add column, new table, change constraint)
2. Migrate the database
3. Run: kiln generate          ← takes seconds
4. Restart server
```

Edited files are protected by checksums. Some files are generated once and
never overwritten (mappers, helpers, main.go) - these are yours to customize
freely. See [Schema Evolution](guides/schema-changes.md) for the full workflow.

## Next Steps

- [Getting Started](getting-started.md) - install and generate your first API
- [Configuration](configuration.md) - customize what kiln generates
- [Schema Evolution](guides/schema-changes.md) - how kiln handles schema changes
- [API Evolution](guides/api-evolution.md) - versioning, deprecation, breaking changes
- [Escape Hatches](guides/escape-hatches.md) - transactions, custom handlers, mixing generated and hand-written code
- [Example Project](https://github.com/fisayoafolayan/demo-blog-api) - a complete blog API you can clone and run
