# Many-to-Many Relationships

kiln automatically detects junction tables and generates link/unlink endpoints on both sides of the relationship.

## How it works

A junction table is detected when:

- It has a composite primary key with exactly 2 columns
- It has exactly 2 foreign keys
- Each FK column is one of the PK columns
- Both FK targets exist in the schema

For example, given this schema:

```sql
CREATE TABLE posts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title TEXT NOT NULL
);

CREATE TABLE tags (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE
);

CREATE TABLE post_tags (
  post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  tag_id  UUID NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
  PRIMARY KEY (post_id, tag_id)
);
```

kiln generates these endpoints on both sides:

**On posts:**
- `POST /api/v1/posts/{id}/tags` - link a tag to a post
- `DELETE /api/v1/posts/{id}/tags/{tagId}` - unlink a tag from a post
- `GET /api/v1/posts/{id}/tags` - list tags linked to a post

**On tags:**
- `POST /api/v1/tags/{id}/posts` - link a post to a tag
- `DELETE /api/v1/tags/{id}/posts/{postId}` - unlink a post from a tag
- `GET /api/v1/tags/{id}/posts` - list posts linked to a tag

## Generated code

### Models

A link request struct is generated for each side:

```go
type LinkTagToPostRequest struct {
    ID uuid.UUID `json:"id" validate:"required"`
}
```

### Store

Three methods are generated per M2M relationship:

- `LinkTag(ctx, postID, req)` - inserts a row into the junction table (idempotent)
- `UnlinkTag(ctx, postID, tagID)` - deletes the junction row
- `ListLinkedTags(ctx, postID, page, pageSize)` - returns paginated linked records

Link operations use `ON CONFLICT DO NOTHING` (Postgres), `INSERT IGNORE` (MySQL), or `INSERT OR IGNORE` (SQLite) to be idempotent.

### Handlers

Link returns `204 No Content`. Unlink returns `204 No Content`. List returns a paginated response using the target table's response type.

## Disabling M2M endpoints

Use `disable` in overrides to turn off link/unlink per table:

```yaml
overrides:
  posts:
    disable: [link, unlink]  # no M2M endpoints on posts
```

The `GET` (list linked) endpoint is always generated.

## Junction tables with extra columns

If the junction table has non-PK columns (e.g. `created_at`, `sort_order`), kiln detects them as extra columns. Currently, these are stored in the IR for future use but not included in the link request body.

## What junction tables are NOT

kiln does not generate full CRUD for junction tables. They don't appear as standalone resources in the API. They're a relationship mechanism - the API surface is on the parent tables.

If you need to store data on the junction (e.g. a `role` column on a `team_members` table), you may want to treat it as a regular table with its own single-column PK instead.
