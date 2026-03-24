# Getting Started

From database schema to running API in under 5 minutes.

## Prerequisites

- Go 1.26 or later
- Docker (for running a local database)
- A Postgres, MySQL/MariaDB, or SQLite database

## Install

```bash
go install github.com/fisayoafolayan/kiln/cmd/kiln@latest
```

## Your First API

### 1. Set up your database

Start a Postgres database (or use an existing one):

```bash
docker run --rm -d \
  --name kiln-db \
  -e POSTGRES_USER=app \
  -e POSTGRES_PASSWORD=app \
  -e POSTGRES_DB=app \
  -p 5432:5432 \
  postgres:16
```

Apply your schema:

```bash
docker exec -i kiln-db psql -U app -d app < schema.sql
```

### 2. Initialize kiln

```bash
mkdir my-api && cd my-api
go mod init my-api

export DATABASE_URL="postgres://app:app@localhost:5432/app?sslmode=disable"
kiln init
```

kiln asks 4 questions:

```
Database driver (postgres/mysql/sqlite): postgres
Database DSN (or leave blank to use an env var):
Environment variable name for DSN: DATABASE_URL
Output directory: ./generated
API base path: /api/v1

✓ Created kiln.yaml
```

### 3. Generate

```bash
kiln generate
```

kiln reads your database schema and generates:

```
✓ generated/models/users.go
✓ generated/store/users.go
✓ generated/store/mappers/users.go
✓ generated/handlers/users.go
✓ generated/handlers/helpers.go
✓ generated/router.go
✓ docs/openapi.yaml
✓ cmd/server/main.go
```

### 4. Run

```bash
go mod tidy
go run cmd/server/main.go
```

Your API is running at `http://localhost:8080`.

### 5. Try it

```bash
# List users
curl http://localhost:8080/api/v1/users | jq

# Create a user
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "name": "Alice"}' | jq

# Filter by role
curl "http://localhost:8080/api/v1/users?role=admin" | jq

# Sort descending
curl "http://localhost:8080/api/v1/users?sort=-created_at" | jq
```

## What's Next

- [Configuration](configuration.md) - customize fields, auth, enums, and more
- [Features](features/filtering.md) - filtering, sorting, soft deletes, enums
- [Example Project](https://github.com/fisayoafolayan/kiln/tree/main/examples/blog-api) - a complete blog API
