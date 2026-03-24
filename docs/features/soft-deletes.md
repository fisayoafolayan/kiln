# Soft Deletes

kiln automatically generates soft delete behavior when a table has a nullable `deleted_at` timestamp column.

## How It Works

Add a nullable timestamp column named `deleted_at`:

```sql
CREATE TABLE posts (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title      TEXT NOT NULL,
  deleted_at TIMESTAMPTZ  -- nullable = soft delete enabled
);
```

No config needed. kiln detects the column and changes the generated behavior:

| Operation | Without `deleted_at` | With `deleted_at` |
|-----------|---------------------|-------------------|
| **DELETE** | `DELETE FROM posts WHERE id = ?` | `UPDATE posts SET deleted_at = now() WHERE id = ?` |
| **GET** | `SELECT ... WHERE id = ?` | `SELECT ... WHERE id = ? AND deleted_at IS NULL` |
| **LIST** | `SELECT ... LIMIT ? OFFSET ?` | `SELECT ... WHERE deleted_at IS NULL LIMIT ? OFFSET ?` |
| **UPDATE** | `SELECT ... WHERE id = ?` | `SELECT ... WHERE id = ? AND deleted_at IS NULL` |

## What Changes

- **DELETE** sets `deleted_at = now()` instead of removing the row
- **GET** returns 404 for soft-deleted records
- **LIST** excludes soft-deleted records from results and counts
- **UPDATE** prevents modifying soft-deleted records (returns 404)
- **Response types** exclude the `deleted_at` field (it's internal)
- **Filters** exclude `deleted_at` (it's managed by kiln, not queryable)

## Requirements

The column must be:

- Named exactly `deleted_at`
- Nullable (no `NOT NULL` constraint)
- A timestamp type (`TIMESTAMPTZ` in Postgres, `DATETIME` in MySQL/SQLite)

## Database Support

=== "Postgres"

    ```sql
    deleted_at TIMESTAMPTZ
    ```

=== "MySQL"

    ```sql
    deleted_at DATETIME
    ```

=== "SQLite"

    ```sql
    deleted_at DATETIME
    ```
