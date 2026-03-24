# kiln

**Turn your database schema into a production-ready Go API. One command.**

No runtime magic. No framework lock-in. Just clean, idiomatic Go code you own.

## The Problem

You have a database schema. To expose it as an API you write:
structs, validation, handlers, a router, an OpenAPI spec.
All by hand. All again when your schema changes.

**kiln generates all of it - from your schema. One command.**

## Quick Example

```sql
CREATE TABLE users (
  id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL UNIQUE,
  name  TEXT NOT NULL,
  role  TEXT NOT NULL DEFAULT 'member'
    CHECK (role IN ('member', 'moderator', 'admin'))
);
```

```bash
kiln init
kiln generate
go run cmd/server/main.go
```

You immediately get:

- `GET /api/v1/users` - list with pagination, filtering & sorting
- `POST /api/v1/users` - create with validation
- `GET /api/v1/users/{id}` - get by ID
- `PATCH /api/v1/users/{id}` - partial update
- `DELETE /api/v1/users/{id}` - delete
- OpenAPI spec at `docs/openapi.yaml`
- Enum validation: `role` rejects values outside `member`, `moderator`, `admin`

All with zero runtime dependency on kiln. You own the output.

## Why kiln?

| | sqlc | ent | kiln |
|---|------|-----|------|
| Input | Hand-written SQL | Go schema DSL | Database schema |
| Output | Types + query functions | Runtime ORM | Types + store + handlers + router + OpenAPI |
| You write | SQL + handlers + router | Schema + handlers | kiln.yaml |
| Runtime dependency | None | ent runtime | None |

kiln is the only tool that goes from database schema to **runnable REST API** in one command, where the output is plain Go you can fork and forget.

## Next Steps

- [Getting Started](getting-started.md) - install and generate your first API
- [Configuration](configuration.md) - customize what kiln generates
- [Example Project](https://github.com/fisayoafolayan/kiln/tree/main/examples/blog-api) - a complete blog API you can clone and run
