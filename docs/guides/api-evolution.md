# API Evolution

kiln regenerates your API from the schema. But APIs also evolve in ways that
aren't schema changes: versioning, deprecation, and breaking changes. kiln
doesn't manage these for you. This guide explains what you own.

## Versioning

kiln generates routes under the base path you configure:

```yaml
api:
  base_path: "/api/v1"
```

When you need a v2, kiln doesn't help you run v1 and v2 side by side. That's
your responsibility. Some approaches:

**Separate base paths.** Run two kiln configs with different base paths and
output dirs. Both point to the same database. v1 stays frozen, v2 regenerates.

**Single base path, manual overrides.** Keep `/api/v1` for most endpoints.
For endpoints that changed, disable them in kiln and write v2 handlers by hand:

```yaml
overrides:
  users:
    disable: [get, update]  # you handle these with custom v2 logic
```

**Accept that internal APIs don't need versioning.** If your consumers are
internal teams, regenerate and coordinate. This is kiln's sweet spot.

kiln takes no stance on which approach is right. That's a product decision,
not a schema decision.

## Deprecating fields

If you remove a column from the schema and regenerate, the field disappears
from the API immediately. There's no deprecation period.

If you need a graceful deprecation:

1. Keep the column in the schema but add it to `hidden_fields` so it stops
   appearing in responses:

```yaml
overrides:
  users:
    hidden_fields:
      - legacy_field
```

2. After consumers have migrated, drop the column and regenerate.

For adding fields, regeneration handles it automatically. New columns appear
in responses and (if writable) in request bodies.

## Breaking vs non-breaking changes

**Non-breaking** (regenerate and done):
- Adding a column
- Adding a table
- Adding a CHECK constraint (enum)
- Adding `deleted_at` (enables soft delete)
- Widening a varchar

**Breaking** (requires consumer coordination):
- Removing a column
- Renaming a column
- Changing a column type
- Removing a table
- Tightening a constraint

kiln regenerates correctly in all cases. But breaking changes break your
consumers whether or not you use kiln. The schema is truth -- if truth changes,
the API changes.

## What kiln doesn't do

- No automatic API versioning
- No deprecation annotations in OpenAPI
- No changelog generation
- No consumer notification

These are application-level concerns. kiln generates structure from schema.
Everything beyond that is yours.
