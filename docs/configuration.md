# Configuration

kiln is configured via `kiln.yaml` in your project root. Run `kiln init` to create one interactively, or write it by hand.

## Full Reference

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

generate:                       # toggle individual layers
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
  description: ""

tables:
  include: []                   # if set, ONLY generate these tables
  exclude:                      # skip these tables
    - schema_migrations

overrides:
  users:
    endpoint: members           # /api/v1/members instead of /api/v1/users
    hidden_fields:              # excluded from all response types
      - password_hash
    readonly_fields:            # excluded from Create/Update requests
      - created_at
      - updated_at
    disable:                    # disable operations: create|update|delete|list|get|link|unlink
      - delete
    filterable_fields:          # allowlist for filters; empty = all columns
      - email
      - role
    sortable_fields:            # allowlist for sorting; empty = all columns
      - created_at
      - name
    enums:                      # allowed values for string columns
      role: [member, moderator, admin]
    # disable_filters: true     # opt-out of filtering entirely
    # disable_sorting: true     # opt-out of sorting entirely
```

## Database Config

=== "Postgres"

    ```yaml
    database:
      driver: postgres
      dsn: "postgres://user:pass@localhost:5432/mydb?sslmode=disable"
    ```

=== "MySQL"

    ```yaml
    database:
      driver: mysql
      dsn: "user:pass@tcp(localhost:3306)/mydb?parseTime=true"
    ```

=== "SQLite"

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

## Per-Table Overrides

Every table can be customized independently under `overrides`:

| Field | Effect |
|-------|--------|
| `endpoint` | Override URL path segment |
| `hidden_fields` | Excluded from all response types |
| `readonly_fields` | Excluded from Create/Update requests |
| `disable` | Disable specific operations |
| `filterable_fields` | Allowlist for query filters |
| `sortable_fields` | Allowlist for sorting |
| `enums` | Allowed values per column |
| `disable_filters` | Opt-out of filtering entirely |
| `disable_sorting` | Opt-out of sorting entirely |
