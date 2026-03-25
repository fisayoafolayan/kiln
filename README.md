# kiln

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.26-blue)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/fisayoafolayan/kiln)](https://goreportcard.com/report/github.com/fisayoafolayan/kiln)
[![Docs](https://img.shields.io/badge/docs-kiln.fisayoafolayan.com-blue)](https://kiln.fisayoafolayan.com)
[![Release](https://img.shields.io/github/v/release/fisayoafolayan/kiln)](https://github.com/fisayoafolayan/kiln/releases)

> Compile your database schema into a production-ready Go API.
>
> Eliminate schema drift - your API always matches your schema.
> Delete kiln - your code still compiles. No framework lock-in.
>
> Generated code uses [bob](https://github.com/stephenafamo/bob) for type-safe query building.
> Bob is the one runtime dependency of the generated output.

## Table of Contents

- [The Problem](#the-problem)
  - [Scope](#scope)
- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Examples](#examples)
- [Sample Schema](#sample-schema)
- [What Gets Generated](#what-gets-generated)
  - [Custom Logic](#custom-logic)
- [Generated Code Example](#generated-code-example)
- [Validation](#validation)
  - [Error Responses](#error-responses)
- [Database Support](#database-support)
- [Brownfield Adoption](#brownfield-adoption)
  - [Write-Once Files](#write-once-files)
- [Schema Evolution](#schema-evolution)
  - [Testing Generated Code](#testing-generated-code)
- [How It Works](#how-it-works)
- [Bob Plugin Mode](#bob-plugin-mode)
- [Why Not sqlc?](#why-not-sqlc)
- [Configuration](#configuration)
  - [Filtering & Sorting](#filtering--sorting)
  - [Enum Validation](#enum-validation)
  - [Soft Deletes](#soft-deletes)
  - [Authentication](#authentication)
- [CLI](#cli)
- [Philosophy](#philosophy)
- [Contributing](#contributing)
- [Roadmap](#roadmap)

---

## The Problem

You build an API by hand. Structs, validation, handlers, router, OpenAPI spec.
It works. Then the schema changes.

A column is renamed. A constraint is added. A table is split.
Your API doesn't match anymore. Tests pass. CI is green. Production breaks.

Because your API no longer matches your schema. This is **schema drift**.

kiln eliminates it. One command generates your entire API layer from
the database schema - structs, validation, handlers, router, OpenAPI spec.
Schema changes? Regenerate. Your API is correct by construction.

*Ideal for CRUD-heavy APIs and backends where the schema evolves frequently.*

### Before kiln

For every table, you write:

- Structs (response, create request, update request)
- Validation tags
- HTTP handlers (list, get, create, update, delete)
- Router wiring
- OpenAPI spec
- Error handling, pagination, filtering

Then do it again when the schema changes.

### After kiln

You write:

- The database schema
- `kiln.yaml` (required config, overrides are optional)

kiln generates everything else.

Think of kiln like a compiler: the schema is your source, `kiln generate` is
the compile step, and the output is a working API. Change the schema, recompile.

### Scope

kiln is for CRUD-heavy APIs where the database schema drives the domain.
Internal tools, admin panels, B2B APIs.

It is **not** a framework. It generates plain Go code that depends on
[bob](https://github.com/stephenafamo/bob) for query building - a compile-time
bet on a younger library. Bob is good, actively maintained, and the generated
code is yours to fork if that ever changes. But it's worth knowing the dependency
is there.

kiln handles additive schema changes perfectly (new columns, new tables).
Destructive changes (renames, type changes) regenerate correctly, but your
write-once files (mappers, helpers) may reference old names - kiln won't
touch those files, so you update them by hand. `kiln doctor` helps catch
stale generated files but can't check your custom code.

**Reach for something else** when the schema isn't the source of truth -
workflow engines, event-driven systems, or APIs shaped by business rules
more than tables.

---

## Requirements

- Go 1.26 or later
- Docker (for running a local database during development)
- A Postgres, MySQL/MariaDB, or SQLite database

No other tools need to be installed manually - kiln sets up everything it
needs when you run `kiln generate`.

---

## Quick Start

```bash
go install github.com/fisayoafolayan/kiln/cmd/kiln@latest

kiln init        # interactive setup, done in seconds
kiln generate    # generates your full API
go run cmd/server/main.go
```

### Try the example

```bash
git clone https://github.com/fisayoafolayan/demo-blog-api.git
cd demo-blog-api
cp .env.example .env
make setup && make run

# In another terminal:
curl http://localhost:8080/api/v1/users | jq
```

See the [Blog API demo](https://github.com/fisayoafolayan/demo-blog-api) for the full walkthrough.

---

## Examples

| Example | Description |
|---------|-------------|
| [Blog API](https://github.com/fisayoafolayan/demo-blog-api) | Full CRUD API with filtering, soft deletes, M2M, enums |
| [Team Task Tracker](https://github.com/fisayoafolayan/demo-team-task-tracker) | Bob plugin mode example |

---

## Sample Schema

Start with a Postgres schema like this (MySQL and SQLite equivalents work too):

```sql
CREATE TABLE users (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  email      TEXT        NOT NULL UNIQUE,
  name       TEXT        NOT NULL,
  bio        TEXT,
  role       TEXT        NOT NULL DEFAULT 'member'
                         CHECK (role IN ('member', 'moderator', 'admin')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE posts (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title        TEXT        NOT NULL,
  body         TEXT        NOT NULL,
  status       TEXT        NOT NULL DEFAULT 'draft'
                           CHECK (status IN ('draft', 'published', 'archived')),
  published_at TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at   TIMESTAMPTZ           -- nullable = soft delete enabled
);

CREATE TABLE tags (
  id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE
);

CREATE TABLE post_tags (
  post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  tag_id  UUID NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
  PRIMARY KEY (post_id, tag_id)
);
```

Run `kiln generate` and you immediately get:

- `GET /api/v1/users` - list users with pagination, filtering & sorting
- `POST /api/v1/users` - create a user with validation
- `GET /api/v1/users/{id}` - get a user by ID
- `PATCH /api/v1/users/{id}` - update a user
- `DELETE /api/v1/users/{id}` - delete a user
- `GET /api/v1/posts` - list posts (soft-deleted posts excluded automatically)
- `GET /api/v1/users/{id}/posts` - list posts by user ← **generated from the FK**
- `POST /api/v1/posts/{id}/tags` - link a tag to a post ← **generated from junction table**
- `DELETE /api/v1/posts/{id}/tags/{tagId}` - unlink a tag from a post
- `GET /api/v1/posts/{id}/tags` - list tags linked to a post

Relationships in your database automatically become API routes. Many-to-many
relationships via junction tables generate link/unlink endpoints on both sides.

All with an OpenAPI spec, type-safe store, and a running server. No boilerplate
written by hand.

---

## What Gets Generated

| File | Contents                                               |
|------|--------------------------------------------------------|
| `generated/models/users.go` | Request/response structs with validation tags          |
| `generated/store/users.go` | Type-safe Store implementation                         |
| `generated/store/mappers/users.go` | Type mapper - yours to customise (write-once)          |
| `generated/handlers/users.go` | Full CRUD HTTP handlers                                |
| `generated/handlers/helpers.go` | Error helpers, pagination, validator (write-once)      |
| `generated/auth/middleware.go` | Auth middleware - JWT or API key (write-once)          |
| `generated/router.go` | Route registration, including FK-derived nested routes |
| `docs/openapi.yaml` | OpenAPI 3.0 spec, always in sync                       |
| `cmd/server/main.go` | Wired-up server, ready to run (write-once)             |

### Custom Logic

Need business logic beyond CRUD? kiln stays out of your way:

- **Transform data** in mappers (`store/mappers/`) - add computed fields, hide sensitive data
- **Custom error handling** in helpers (`handlers/helpers.go`) - change formats, add logging
- **Wrap or replace handlers** - the store uses interfaces, mock or swap anything
- **Disable generated endpoints** per table and write your own
- **Delete and replace anything** - it's your code, not a framework

```go
// Disable the generated "update" for posts, add your own with business logic:
func (h *PostHandler) Publish(w http.ResponseWriter, r *http.Request) {
    // custom validation, state transitions, notifications - your code
}
```

You're never locked in. The generated code is just Go.

---

## Generated Code Example

Here's what kiln generates for the `users` table. The generated code is
intentionally boring - no abstractions, no magic. Just idiomatic Go you'd
write by hand:

```go
// generated/handlers/users.go - Code generated by kiln. DO NOT EDIT.
// Re-generated on each run. Customise helpers.go or disable operations in kiln.yaml.
package handlers

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req models.CreateUserRequest
    if !decodeJSON(w, r, &req) {
        return
    }
    if !validateRequest(w, req) {
        return
    }
    row, err := h.store.Create(r.Context(), req)
    if err != nil {
        handleStoreError(w, err, "users", "create")
        return
    }
    writeJSON(w, http.StatusCreated, row)
}
```

```go
// generated/models/users.go - Code generated by kiln. DO NOT EDIT.
// Re-generated on each run. Use hidden_fields/readonly_fields in kiln.yaml to customise.
package models

type User struct {
    ID        uuid.UUID  `json:"id"`
    Email     string     `json:"email"`
    Name      string     `json:"name"`
    Bio       *string    `json:"bio,omitempty"`
    Role      string     `json:"role"`
    CreatedAt time.Time  `json:"created_at"`
    UpdatedAt time.Time  `json:"updated_at"`
}

type CreateUserRequest struct {
    Email string  `json:"email" validate:"required,email"`
    Name  string  `json:"name"  validate:"required,min=1,max=255"`
    Role  string  `json:"role"  validate:"omitempty,oneof=member admin"`
}
```

---

## Validation

kiln generates request structs with `validate` tags and wires up
[go-playground/validator](https://github.com/go-playground/validator)
automatically. Invalid requests return structured error responses:

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name": ""}'
```

```json
{
  "error": "validation failed",
  "fields": {
    "email": "is required",
    "name": "must be at least 1 characters"
  }
}
```

### Error Responses

All generated handlers use consistent error shapes. The error logic lives in
`helpers.go` (write-once) so you can customize formats, add logging, or
integrate with your observability stack.

| Scenario | Status | Response |
|----------|--------|----------|
| Validation failure | 400 | `{"error": "validation failed", "fields": {"name": "is required"}}` |
| Invalid filter/sort | 400 | `{"error": "invalid value for created_at: expected RFC3339 format"}` |
| Malformed JSON | 400 | `{"error": "invalid request body"}` |
| Body too large | 413 | `{"error": "request body too large"}` |
| Not found | 404 | `{"error": "user not found"}` |
| Unique violation | 409 | `{"error": "user already exists"}` |
| FK violation | 422 | `{"error": "referenced user does not exist"}` |
| Server error | 500 | `{"error": "internal server error"}` |

Error detection uses database error codes (not string matching) so it works
correctly across Postgres, MySQL, and SQLite regardless of locale.

---

## Database Support

kiln supports Postgres, MySQL/MariaDB, and SQLite out of the box.

**Postgres**
```yaml
database:
  driver: postgres
  dsn: "postgres://user:pass@localhost:5432/mydb?sslmode=disable"
```

**MySQL / MariaDB**
```yaml
database:
  driver: mysql
  dsn: "user:pass@tcp(localhost:3306)/mydb?parseTime=true"
```

**SQLite**
```yaml
database:
  driver: sqlite
  dsn: "./mydb.db"
```

Use an environment variable instead of hardcoding the DSN:

```yaml
database:
  driver: postgres
  dsn_env: DATABASE_URL
```

---

## Brownfield Adoption

Already have an existing project? kiln is **additive, never destructive**.
Adopt one layer at a time:

```yaml
generate:
  models: true     # just the structs to start
  store: false     # keep your own DB layer
  handlers: false  # keep your own handlers
  openapi: true    # free OpenAPI spec, always in sync
```

### Write-Once Files

Mapper files (`store/mappers/`) are generated once then handed to you.
Customize them freely - kiln will never overwrite them.

```go
// store/mappers/users.go - THIS FILE IS YOURS
package mappers

func UserToType(m *dbmodels.User) *models.User {
    return &models.User{
        ID:       m.ID,
        Email:    m.Email,
        Name:     m.Name,
        // Add computed fields, hide sensitive data, transform values:
        FullName: m.FirstName + " " + m.LastName,
        IsAdmin:  m.Role == "admin",
    }
}
```

Change your schema, regenerate - your mapper is untouched.

---

## Schema Evolution

This is where kiln differs from one-time scaffolding tools. Your schema changes over time - kiln keeps your API in sync.

### The workflow

```
1. Change your schema (add column, rename field, add table)
2. Migrate the database (goose, atlas, golang-migrate, raw SQL)
3. Run: kiln generate
4. Done. API matches the new schema.
```

### What happens on regenerate

**Auto-generated files** are updated to match the new schema. Each file has an
embedded SHA-256 checksum in a comment on line 2. On regeneration, kiln:

1. Reads the existing file's checksum
2. Recomputes the hash of the file's current contents
3. If they differ (you edited the file), **skips it** with a warning
4. If they match (untouched), overwrites with the new version

```
  ⚠ SKIPPED  generated/store/users.go (user-modified; use --force to overwrite)
  ✓ generated/models/users.go
  ✓ generated/handlers/users.go
  ✓ generated/router.go
  ✓ docs/openapi.yaml
```

Any content change counts as a modification - even a comment or whitespace.
`--force` overrides the check and regenerates everything. There is no undo,
so use `kiln diff` first to review what would change.

**Write-once files** (mappers, helpers, main.go) are never touched. You update those manually when needed.

### Preview before regenerating

Not sure what will change? Preview first:

```bash
kiln diff
```

```
  + generated/models/users.go
  + generated/models/posts.go
  + generated/store/users.go
  + generated/store/posts.go
  + generated/handlers/users.go
  + generated/handlers/posts.go
  + generated/router.go
  + docs/openapi.yaml
```

No files written. No risk. See exactly what kiln would generate, then decide.

### Tools for safe evolution

| Command | Use when |
|---------|----------|
| `kiln diff` | Preview what would be generated (no files written) |
| `kiln generate` | Regenerate, skip edited files |
| `kiln generate --force` | Overwrite everything, including edits |
| `kiln generate --table users` | Regenerate only one table |

### Example: adding a column end-to-end

```bash
# 1. Add the column to your database
psql "$DATABASE_URL" -c "ALTER TABLE users ADD COLUMN avatar_url TEXT;"

# 2. Regenerate
kiln generate

# 3. Commit
git add -A && git commit -m "add avatar_url to users"
```

kiln updates the response struct, create/update requests, store queries,
handler filters, and OpenAPI spec. One migration, one command, everything
in sync.

### Why this matters

Without kiln, a schema change means manually updating structs, handlers, validation, filters, OpenAPI spec - hoping nothing falls out of sync. With kiln, the schema is the single source of truth. Change it, regenerate, and the API is correct by construction - not by convention or tests.

### Testing generated code

kiln doesn't generate tests - testing strategies are too project-specific. But the generated code is designed for testability:

**Handlers** depend on a store interface, not a concrete type. Mock the interface:

```go
type mockUserStore struct{}

func (m *mockUserStore) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
    return &models.User{Name: "Alice"}, nil
}
```

**Store methods** accept `bob.Executor`, which both `bob.DB` and `bob.Tx` satisfy.
This means you can pass a transaction for test isolation or multi-step operations:

```go
db := bob.NewDB(testDB) // bob.DB satisfies bob.Executor
store := store.NewUserStore(db)
user, err := store.Get(ctx, id)

// Or wrap in a transaction:
tx, _ := testDB.BeginTx(ctx, nil)
txStore := store.NewUserStore(bob.NewTx(tx))
// both calls in one transaction
```

---

## How It Works

```
Your Database Schema
      │
      ├──► bob reads schema ──► ./models/              (query builders - internal)
      │
      └──► kiln generates ──► ./generated/
                                  models/              (request/response structs)
                                  store/               (DB operations)
                                  store/mappers/       (type mappers - yours to edit)
                                  handlers/            (HTTP handlers)
                                  router.go            (route registration)
                               ./docs/openapi.yaml    (API spec)
                               ./cmd/server/main.go   (runnable server - yours to edit)
```

kiln uses [bob](https://github.com/stephenafamo/bob) (v0.42.0+) for schema
introspection and as the query builder in generated store code. Bob is a
runtime dependency of the generated code - your `go.mod` will include it.
Bob runs in-process during generation - no external binary to install, no
`bobgen.yaml` to manage. Just `kiln generate`.

**What bob handles:** schema introspection, column type resolution, foreign key
detection, and Go model generation (`./models/`). If you hit a type that kiln
doesn't recognise or a column that behaves unexpectedly, bob's generated
`dbinfo/` files are the place to look - they contain the raw schema metadata
kiln reads from.

**Foreign key resolution:** kiln reads bob's generated relationship structs to
accurately resolve FK relationships - even when column names don't match table
names (e.g. `author_id → users`). This is how nested routes like
`GET /users/{id}/posts` are discovered automatically.

**Primary key handling:**
- Single-column PKs generate standard `/{id}` routes
- Composite PKs generate multi-param routes (e.g. `/tenant-users/{tenantId}/{userId}`)
- Junction tables (composite PK + exactly 2 FKs) generate M2M link/unlink endpoints instead of CRUD

**Known limitations:**
- Postgres array types (`text[]`) require bob v0.42.0+ for correct Go type mapping
- Custom Postgres types (ltree, ranges) are mapped to `string`

**Advanced: bob plugin mode.** For teams already using bob's code generation
pipeline, kiln is also available as a
[bob plugin](https://github.com/fisayoafolayan/kiln/tree/main/plugin). See
[Bob Plugin Mode](#bob-plugin-mode) below.

---

## Bob Plugin Mode

By default, `kiln generate` handles everything - most projects should use that.
For teams already using bob's code generation pipeline, kiln is also available
as a bob plugin. One command generates bob models and kiln's API layer together.

### Install

```bash
go get github.com/fisayoafolayan/kiln@latest
```

### Create a custom gen/main.go

Instead of running `bobgen-psql` directly, create a small entrypoint that loads
kiln as a plugin:

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"

    "github.com/stephenafamo/bob/gen"
    "github.com/stephenafamo/bob/gen/bobgen-psql/driver"
    "github.com/stephenafamo/bob/gen/plugins"
    kilnplugin "github.com/fisayoafolayan/kiln/plugin"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    driverConfig := driver.Config{DSN: os.Getenv("DATABASE_URL")}
    config := gen.Config{}

    bobPlugins := plugins.Setup[any, any, driver.IndexExtra](
        plugins.Config{}, gen.PSQLTemplates,
    )

    kiln := kilnplugin.New[any, any, driver.IndexExtra](kilnplugin.Options{
        ConfigPath: "kiln.yaml",
    })

    state := &gen.State[any]{Config: config}
    allPlugins := append(bobPlugins, kiln)

    if err := gen.Run(ctx, state, driver.New(driverConfig), allPlugins...); err != nil {
        log.Fatal(err)
    }
}
```

```bash
DATABASE_URL="postgres://..." go run gen/main.go
```

kiln implements bob's `DBInfoPlugin` interface - bob calls it after schema
introspection, and kiln generates the same output as `kiln generate`.

For the full guide including MySQL/SQLite setup and plugin options, see the
[Bob Plugin Mode documentation](https://kiln.fisayoafolayan.com/guides/bob-plugin/).
For a complete working example, see the
[Team Task Tracker](https://github.com/fisayoafolayan/demo-team-task-tracker).

---

## Why Not sqlc?

[sqlc](https://github.com/sqlc-dev/sqlc) generates type-safe Go code from
hand-written SQL queries. It's excellent - but it stops at the database layer.

kiln generates the **full HTTP layer on top**: request/response types with
validation, handlers, a router, and an OpenAPI spec. You don't write SQL or
handler code. If sqlc gives you a type-safe database client, kiln gives you
a runnable API server.

| | sqlc | kiln |
|---|------|------|
| Input | Hand-written SQL queries | Database schema |
| Output | Go types + query functions | Types + store + handlers + router + OpenAPI |
| You write | SQL + handlers + router | kiln.yaml |
| Runtime dependency | None | bob (query builder) |

Use sqlc when you need fine-grained control over every query. Use kiln when
you want a working API from your schema with minimal code.

---

## Configuration

`kiln.yaml` reference:

```yaml
version: 1

database:
  driver: "postgres"            # postgres | mysql | sqlite
  dsn: "postgres://..."         # direct DSN
  # dsn_env: DATABASE_URL       # or use an env var (takes precedence over dsn)

output:
  dir: "./generated"            # where generated code goes
  package: generated            # Go package name for generated code

api:
  base_path: "/api/v1"          # prefix for all routes
  framework: stdlib             # stdlib | chi

auth:
  strategy: none                # none | jwt | api_key
  header: Authorization         # header to read (X-API-Key for api_key strategy)

bob:
  enabled: true                 # false = skip DB introspection, use existing models
  models_dir: "./models"        # where bob writes its query builder models

generate:                       # toggle individual layers for brownfield adoption
  models: true
  store: true
  handlers: true
  router: true
  openapi: true

openapi:
  enabled: true
  output: "./docs/openapi.yaml"
  title: "My API"
  version: "1.0.0"
  description: ""               # optional

tables:
  include: []                   # if set, ONLY generate these tables
  exclude:                      # skip these tables (mutually exclusive with include)
    - schema_migrations

overrides:
  users:
    endpoint: members           # override URL path: /api/v1/members instead of /api/v1/users
    hidden_fields:              # excluded from all response types
      - password_hash
    readonly_fields:            # excluded from Create/Update request types
      - created_at
      - updated_at
    disable:                    # disable specific operations: create|update|delete|list|get|link|unlink
      - delete
    filterable_fields:          # allowlist for query filters; empty = all columns
      - email
      - role
      - created_at
    sortable_fields:            # allowlist for sorting; empty = all columns
      - created_at
      - name
    enums:                      # allowed values for string columns
      role: [member, moderator, admin]
    # disable_filters: true     # opt-out of filtering entirely
    # disable_sorting: true     # opt-out of sorting entirely
```

### Filtering & Sorting

List endpoints support filtering via query parameters and sorting via the `sort` parameter:

```bash
# Filter by exact match
GET /api/v1/users?role=admin

# Filter with operators
GET /api/v1/users?created_at[gte]=2024-01-01T00:00:00Z&created_at[lt]=2025-01-01T00:00:00Z

# Sort ascending
GET /api/v1/users?sort=created_at

# Sort descending (prefix with -)
GET /api/v1/users?sort=-created_at

# Combine filters, sorting, and pagination
GET /api/v1/users?role=admin&sort=-created_at&page=2&page_size=10
```

Supported operators: `eq` (default), `neq`, `gt`, `gte`, `lt`, `lte`.
Range operators (`gt/gte/lt/lte`) are available for numeric and timestamp columns.

By default, all non-hidden columns are filterable and sortable.
Use `filterable_fields` / `sortable_fields` in overrides to restrict which columns are exposed.

### Enum Validation

If your schema uses `CHECK` constraints, kiln auto-detects allowed values and generates validation:

```sql
CREATE TABLE posts (
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived'))
);
```

This generates `validate:"oneof=draft published archived"` automatically - no config needed.

For columns without `CHECK` constraints, specify allowed values in `kiln.yaml`:

```yaml
overrides:
  posts:
    enums:
      status: [draft, published, archived]
```

Invalid values return a structured error:

```json
{
  "error": "validation failed",
  "fields": {
    "status": "must be one of: draft published archived"
  }
}
```

Config values always take precedence over auto-detected constraints.

### Soft Deletes

If a table has a nullable `deleted_at` timestamp column, kiln automatically generates soft delete behavior:

```sql
CREATE TABLE posts (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title      TEXT NOT NULL,
  deleted_at TIMESTAMPTZ  -- nullable = soft delete enabled
);
```

No config needed. Kiln detects `deleted_at` and:

- **DELETE** sets `deleted_at = now()` instead of removing the row
- **GET/LIST** adds `WHERE deleted_at IS NULL` to exclude soft-deleted records
- **UPDATE** prevents modifying soft-deleted records
- **Response types** exclude the `deleted_at` field (it's internal)

### Authentication

Set `strategy: api_key` or `strategy: jwt` in kiln.yaml. kiln generates a
write-once middleware file at `generated/auth/middleware.go` with a skeleton
you fill in:

```go
// generated/auth/middleware.go - THIS FILE IS YOURS
func Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := r.Header.Get("X-API-Key")
        if key == "" {
            writeError(w, http.StatusUnauthorized, "missing api key")
            return
        }
        // TODO: validate the key against your database or config
        next.ServeHTTP(w, r)
    })
}
```

kiln deliberately does not generate token validation, secret management, or
JWT parsing. These are application-specific. The middleware gives you the
hook point - you add your logic. This is consistent with kiln's philosophy:
generate the structure, you own the decisions.

---

## CLI

```
kiln init                  Create kiln.yaml interactively
kiln generate              Generate your API (runs schema introspection first)
kiln generate --table X    Regenerate only one table (useful for large schemas)
kiln generate --no-bob     Skip schema reading, use existing models
kiln generate --force      Overwrite files even if they have been manually edited
kiln diff                  Preview what would be generated (no files written)
kiln doctor                Check project health: config, schema, generated files
kiln introspect            Print the parsed schema (text format)
kiln introspect --format json   Print as JSON (for scripting)
kiln version               Print kiln version
```

All commands accept `--config path/to/kiln.yaml` (default: `kiln.yaml`).

Running `kiln generate` without a `kiln.yaml` file will fail with a clear
error pointing you to `kiln init`.

---

## Philosophy

- **Schema is truth.** Your database already describes your domain. The API should follow it, not the other way around.
- **Correctness over speed.** Generation is fast, but the real value is that your API never drifts from your schema. Change the schema, regenerate, done.
- **You own the output.** No runtime dependency on kiln. The generated code depends on bob for query building - a standard Go library, not a framework. Fork and forget.
- **Escape hatches everywhere.** Write-once files are yours forever. Checksums protect your edits. Nothing is locked down.
- **Idiomatic Go.** Output looks like code a senior Go dev wrote by hand.
- **Brownfield friendly.** Adopt one layer at a time. Start with types, add store later, add handlers when ready.

### What kiln is not

- **Not an ORM** - no runtime query layer
- **Not a framework** - no runtime control, no hidden magic
- **Not one-shot scaffolding** - regenerate safely as your schema evolves

kiln is a compiler for APIs. Schema in, Go code out. Delete kiln and the code still compiles.

### When to use kiln partially

Some tables are CRUD, others need custom logic. Generate models and OpenAPI
for everything, handlers for the simple tables, write your own for the rest:

```yaml
overrides:
  payments:
    disable: [create, update, delete]  # kiln generates list/get, you handle mutations
```

---

## Contributing

```bash
git clone https://github.com/fisayoafolayan/kiln
cd kiln

# Run unit tests
make test

# Run end-to-end tests against Postgres (requires Docker)
make e2e/postgres

# Run against all databases
make e2e/all

# See all available commands
make help
```

The e2e tests spin up Docker containers automatically - no manual database
setup required.

When adding a new feature:

1. Update the relevant generator in `internal/generator/`:
   - `types/` - generates `generated/models/` (request/response structs)
   - `store/` - generates `generated/store/` and `generated/store/mappers/`
   - `handlers/` - generates `generated/handlers/` and `helpers.go`
   - `router/` - generates `generated/router.go`
   - `openapi/` - generates `docs/openapi.yaml`
   - `auth/` - generates `generated/auth/middleware.go`
   - `mainfile/` - generates `cmd/server/main.go`
2. Add unit tests in the generator's package or `internal/generator/generator_test.go`
3. Add parser tests in `internal/parser/bob/bob_test.go` if changing type resolution
4. Run `make e2e/postgres` to verify end to end
5. Open a PR with a clear description of what changed and why

---

## Roadmap

**Shipped:**
- [x] Postgres, MySQL/MariaDB, SQLite support
- [x] Full CRUD handler generation
- [x] OpenAPI 3.0 spec generation
- [x] FK-derived nested routes (from foreign keys)
- [x] Brownfield layer adoption (toggle individual layers)
- [x] go-playground/validator integration
- [x] Enum validation (auto-detected from CHECK constraints + config)
- [x] Soft deletes (auto-detected from `deleted_at` column)
- [x] MaxLength extraction from varchar(N)
- [x] Chi and stdlib router support
- [x] Filtering, sorting & pagination
- [x] Authentication middleware (JWT and API key)
- [x] Checksum-based regeneration safety
- [x] Locale-independent database error classification
- [x] Bob plugin support (use kiln as a bob plugin or standalone CLI)
- [x] In-process schema reading (no external bob binary needed)
- [x] Many-to-many link/unlink endpoints (via junction tables)
- [x] Filter and sort validation (400 on invalid values)
- [x] `kiln doctor` diagnostic command
- [x] Store accepts `bob.Executor` (enables transactions without generated code changes)

**Next:**
- [ ] Cursor-based pagination
- [ ] Relationship loading (`?include=author`)

**Later:**
- [ ] Gin framework support
- [ ] Transaction support in store
- [ ] Batch create/update/delete endpoints
- [ ] Test scaffolding (generated test helpers and fixtures)
- [ ] Multi-tenancy / row-level scoping (`tenant_id` auto-injection)

---

## Inspired By

- [bob](https://github.com/stephenafamo/bob) - schema-driven query building for Go
- [sqlboiler](https://github.com/volatiletech/sqlboiler) - the codegen philosophy

---

## License

MIT - see [LICENSE](LICENSE)