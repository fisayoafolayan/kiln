# kiln

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.26-blue)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/fisayoafolayan/kiln)](https://goreportcard.com/report/github.com/fisayoafolayan/kiln)
[![Docs](https://img.shields.io/badge/docs-kiln.fisayoafolayan.com-blue)](https://kiln.fisayoafolayan.com)

> Compile your database schema into a production-ready Go API.
>
> Eliminate schema drift - your API always matches your schema.
> Zero runtime dependency. No framework lock-in. Delete kiln — your code still compiles.

## Table of Contents

- [The Problem](#the-problem)
- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Sample Schema](#sample-schema)
- [What Gets Generated](#what-gets-generated)
  - [Custom Logic](#custom-logic)
- [Generated Code Example](#generated-code-example)
- [Validation](#validation)
- [Database Support](#database-support)
- [Brownfield Adoption](#brownfield-adoption)
  - [Write-Once Files](#write-once-files)
- [Schema Evolution](#schema-evolution)
  - [Testing Generated Code](#testing-generated-code)
- [How It Works](#how-it-works)
- [Why Not sqlc?](#why-not-sqlc)
- [Configuration](#configuration)
  - [Filtering & Sorting](#filtering--sorting)
  - [Enum Validation](#enum-validation)
  - [Soft Deletes](#soft-deletes)
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

You write: structs, validation tags, handlers, router, OpenAPI spec, error handling, pagination, filtering. For every table. Again when the schema changes.

### After kiln

You write: the schema. kiln generates everything else.

Think of kiln like a compiler: the schema is your source, `kiln generate` is
the compile step, and the output is a working API. Change the schema, recompile.

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

kiln init        # 4 questions, done
kiln generate    # generates your full API
go run cmd/server/main.go
```

### Try the example

```bash
git clone https://github.com/fisayoafolayan/kiln.git
cd kiln/examples/blog-api
cp .env.example .env
make setup && make run

# In another terminal:
curl http://localhost:8080/api/v1/users | jq
```

See the [blog API example](examples/blog-api/) for the full walkthrough.

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
```

Run `kiln generate` and you immediately get:

- `GET /api/v1/users` - list users with pagination, filtering & sorting
- `POST /api/v1/users` - create a user with validation
- `GET /api/v1/users/{id}` - get a user by ID
- `PATCH /api/v1/users/{id}` - update a user
- `DELETE /api/v1/users/{id}` - delete a user
- `GET /api/v1/posts` - list posts (soft-deleted posts excluded automatically)
- `GET /api/v1/users/{id}/posts` - list posts by user ← **generated from the FK**

Relationships in your database automatically become API routes.

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
    // custom validation, state transitions, notifications — your code
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

**Auto-generated files** are updated to match the new schema. Each file has an embedded checksum. If you've edited one, kiln protects your changes:

```
  ⚠ SKIPPED  generated/store/users.go (user-modified; use --force to overwrite)
  ✓ generated/models/users.go
  ✓ generated/handlers/users.go
  ✓ generated/router.go
  ✓ docs/openapi.yaml
```

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
| `kiln diff` | Preview what would change before writing |
| `kiln generate` | Regenerate, skip edited files |
| `kiln generate --force` | Overwrite everything, including edits |
| `kiln generate --table users` | Regenerate only one table |

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

**Store methods** are single-query functions that work against any `bob.DB`:

```go
db := bob.NewDB(testDB) // real DB or txdb for isolation
store := store.NewUserStore(db)
user, err := store.Get(ctx, id)
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

kiln uses [bob](https://github.com/stephenafamo/bob) (v0.42.0+) under the hood
to read your database schema. You don't need to learn bob - kiln installs it,
configures it, and manages `bobgen.yaml` automatically.

**What bob handles:** schema introspection, column type resolution, foreign key
detection, and Go model generation (`./models/`). If you hit a type that kiln
doesn't recognise or a column that behaves unexpectedly, bob's generated
`dbinfo/` files are the place to look - they contain the raw schema metadata
kiln reads from.

**Foreign key resolution:** kiln reads bob's generated relationship structs to
accurately resolve FK relationships - even when column names don't match table
names (e.g. `author_id → users`). This is how nested routes like
`GET /users/{id}/posts` are discovered automatically.

**Known bob limitations:**
- Composite primary keys are detected but not supported for code generation (kiln warns and skips)
- Postgres array types (`text[]`) require bob v0.42.0+ for correct Go type mapping
- Custom Postgres types (ltree, ranges) are mapped to `string`

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
| Runtime dependency | None | None |

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
  header: Authorization         # header to read credentials from

bob:
  enabled: true                 # set to false to skip schema introspection
  models_dir: "./models"        # where bob writes its models

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
    disable:                    # disable specific operations: create|update|delete|list|get
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

---

## CLI

```
kiln init                  Create kiln.yaml interactively
kiln generate              Generate your API (runs schema introspection first)
kiln generate --table X    Regenerate only one table (useful for large schemas)
kiln generate --no-bob     Skip schema reading, use existing models
kiln generate --dry-run    Preview changes without writing files
kiln generate --force      Overwrite files even if they have been manually edited
kiln diff                  Preview what would be generated without writing files
kiln introspect            Print the parsed schema (text format)
kiln introspect --format json   Print as JSON (for scripting)
kiln version               Print kiln version
```

All commands accept `--config path/to/kiln.yaml` (default: `kiln.yaml`).

---

## Philosophy

- **Schema is truth.** Your database already describes your domain. The API should follow it, not the other way around.
- **Correctness over speed.** Generation is fast, but the real value is that your API never drifts from your schema. Change the schema, regenerate, done.
- **You own the output.** Zero runtime dependency on kiln. Fork and forget.
- **Escape hatches everywhere.** Write-once files are yours forever. Checksums protect your edits. Nothing is locked down.
- **Idiomatic Go.** Output looks like code a senior Go dev wrote by hand.
- **Brownfield friendly.** Adopt one layer at a time. Start with types, add store later, add handlers when ready.

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
1. Update the relevant generator in `internal/generator/`
2. Add tests in `internal/generator/generator_test.go`
3. Run `make e2e/postgres` to verify end to end
4. Open a PR with a clear description of what changed and why

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

**Up next:**
- [ ] Composite PK link/unlink endpoints (many-to-many)
- [ ] Cursor-based pagination
- [ ] Gin framework support
- [ ] Relationship loading (`?include=author`)
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