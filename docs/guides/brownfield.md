# Adopt What You Need

Every layer is optional. You don't have to generate everything - adopt one
layer at a time.

Most teams generate models, store, and OpenAPI - handlers are optional.
Handlers are the layer closest to business logic, and in practice most
non-trivial tables outgrow generated handlers quickly as they pick up
conditional writes, cross-table invariants, or side effects. Generate handlers
for the genuinely simple CRUD tables, write your own for everything else.

## Toggle Layers

```yaml
generate:
  models: true      # request/response structs
  store: true       # type-safe DB operations
  handlers: false   # write your own handlers
  router: false     # wire routes yourself
  openapi: true     # free OpenAPI spec, always in sync
```

Start with `models: true` to get validated request/response types. Add layers as you gain confidence.

## Common Adoption Paths

### "I just want types"

```yaml
generate:
  models: true
  store: false
  handlers: false
  router: false
  openapi: true
```

You get `generated/models/` with validated structs and `docs/openapi.yaml`. Write your own store and handlers using kiln's types.

### "I want types and store, but my own handlers"

```yaml
generate:
  models: true
  store: true
  handlers: false
  router: false
  openapi: true
```

You get type-safe store methods. Your custom handlers call the generated store:

```go
// your code - sits alongside generated handlers
func (h *PaymentHandler) Charge(w http.ResponseWriter, r *http.Request) {
    var req models.CreatePaymentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    // your business logic
    if err := h.billing.Authorize(r.Context(), req.Amount); err != nil {
        http.Error(w, "payment declined", http.StatusUnprocessableEntity)
        return
    }

    // then use kiln's generated store
    payment, err := h.store.Create(r.Context(), req)
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(payment)
}
```

### "Generate everything for new tables, keep my existing ones"

```yaml
tables:
  include:
    - new_table_1
    - new_table_2
```

Or exclude existing tables:

```yaml
tables:
  exclude:
    - legacy_users
    - legacy_orders
```

## Write-Once Files

These files are generated once, then yours forever:

| File | Purpose |
|------|---------|
| `generated/store/mappers/*.go` | Map bob models to response types |
| `generated/handlers/helpers.go` | Error formatting, pagination, validation |
| `generated/auth/middleware.go` | Auth middleware (if enabled) |
| `cmd/server/main.go` | Server entry point and wiring |

Add computed fields in mappers, custom error formats in helpers, auth logic in middleware. kiln will never overwrite them.

## Regeneration Safety

Auto-generated files have embedded checksums. If you edit one, kiln skips it:

```
  ⚠ SKIPPED  generated/store/users.go (user-modified; use --force to overwrite)
```

Use `kiln generate --force` to overwrite anyway, or `kiln diff` to preview changes.
