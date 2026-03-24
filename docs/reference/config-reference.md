# kiln.yaml Reference

Complete reference for all configuration options.

## `database`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `driver` | string | `"postgres"` | Database driver: `postgres`, `mysql`, `sqlite` |
| `dsn` | string | | Direct database connection string |
| `dsn_env` | string | | Environment variable name containing the DSN (takes precedence over `dsn`) |

## `output`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `dir` | string | `"./generated"` | Directory for generated code |
| `package` | string | `"generated"` | Go package name for generated code |

## `api`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `base_path` | string | `"/api/v1"` | URL prefix for all routes |
| `framework` | string | `"stdlib"` | Router framework: `stdlib` or `chi` |

## `auth`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `strategy` | string | `"none"` | Auth strategy: `none`, `jwt`, `api_key` |
| `header` | string | `"Authorization"` | HTTP header to read credentials from |

## `bob`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Run schema introspection via bob |
| `models_dir` | string | `"./models"` | Where bob writes its query builder models |

## `generate`

Toggle individual layers on/off. All default to `true`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `models` | bool | `true` | Request/response type structs |
| `store` | bool | `true` | Database operation layer |
| `handlers` | bool | `true` | HTTP handlers |
| `router` | bool | `true` | Route registration |
| `openapi` | bool | `true` | OpenAPI 3.0 specification |

## `openapi`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Generate OpenAPI spec |
| `output` | string | `"./docs/openapi.yaml"` | Output file path |
| `title` | string | `"My API"` | API title in spec |
| `version` | string | `"1.0.0"` | API version in spec |
| `description` | string | `""` | API description in spec |

## `tables`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `include` | []string | `[]` | If set, ONLY generate these tables |
| `exclude` | []string | `[]` | Skip these tables (mutually exclusive with `include`) |

## `overrides`

Per-table customizations. Keyed by table name.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `endpoint` | string | table name | Override URL path segment |
| `hidden_fields` | []string | `[]` | Excluded from all response types |
| `readonly_fields` | []string | `[]` | Excluded from Create/Update request types |
| `disable` | []string | `[]` | Disable operations: `create`, `update`, `delete`, `list`, `get` |
| `filterable_fields` | []string | `[]` | Allowlist for query filters (empty = all columns) |
| `sortable_fields` | []string | `[]` | Allowlist for sorting (empty = all columns) |
| `enums` | map[string][]string | `{}` | Allowed values per column name |
| `disable_filters` | bool | `false` | Opt-out of filtering entirely |
| `disable_sorting` | bool | `false` | Opt-out of sorting entirely |
