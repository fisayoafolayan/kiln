# Escape Hatches

kiln generates CRUD. Real applications need more. This guide shows patterns
for extending generated code without fighting kiln.

## Disable an operation and write your own

The most common escape hatch. Disable a generated handler and replace it:

```yaml
overrides:
  posts:
    disable: [create]  # kiln generates list/get/update/delete, you handle create
```

Then write your custom create handler with business logic:

```go
// In your own handlers package (not generated/)
func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req models.CreatePostRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Custom business logic: validate state, check permissions, notify
    if req.Status == "published" && !h.canPublish(r.Context()) {
        http.Error(w, "not authorized to publish", http.StatusForbidden)
        return
    }

    post, err := h.store.Create(r.Context(), req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Side effects: send notification, update cache
    h.notifier.PostCreated(r.Context(), post)

    json.NewEncoder(w).Encode(post)
}
```

Register it alongside the generated routes in your `main.go`.

## Wrap a generated handler with middleware

The generated handlers are plain `http.HandlerFunc`. Wrap them:

```go
// Rate limit just the create endpoint
mux.HandleFunc("POST /api/v1/users", rateLimit(usersHandler.Create))

// Or wrap the entire handler via the generated router
generated.RegisterRoutes(mux, usersHandler, postsHandler)
// Then add global middleware in main.go
handler := logging(auth(mux))
```

## Use transactions across stores

Stores accept `bob.Executor`. Both `bob.DB` and `bob.Tx` satisfy it.
Create stores with a transaction to get atomicity:

```go
tx, err := sqlDB.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

bobTx := bob.NewTx(tx)
userStore := store.NewUserStore(bobTx)
postStore := store.NewPostStore(bobTx)

user, err := userStore.Create(ctx, userReq)
if err != nil {
    return err
}

postReq.UserID = user.ID
_, err = postStore.Create(ctx, postReq)
if err != nil {
    return err
}

return tx.Commit()
```

## Add computed fields in mappers

Mappers are write-once files. Add any transformation:

```go
// generated/store/mappers/users.go
func UserToType(m *dbmodels.User) *models.User {
    t := &models.User{
        ID:    m.ID,
        Email: m.Email,
        Name:  m.Name,
    }
    // Computed fields - not in the schema, but in the API
    t.DisplayName = m.Name
    if m.Bio.IsNull() {
        t.DisplayName = m.Name + " (no bio)"
    }
    return t
}
```

This requires adding the field to the response struct. Since `models/users.go`
is auto-generated, you have two options:

1. Add the field to a separate file in the same package (Go allows this)
2. Create a wrapper type in your own package

## Add custom routes alongside generated ones

The generated router registers routes on a standard `http.ServeMux` or chi
router. Add your own routes before or after:

```go
mux := http.NewServeMux()

// Generated CRUD routes
generated.RegisterRoutes(mux, usersHandler, postsHandler)

// Custom routes
mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
mux.HandleFunc("POST /api/v1/posts/{id}/publish", postWorkflow.Publish)
mux.HandleFunc("GET /api/v1/stats", statsHandler.Dashboard)
```

## Custom error formatting

`generated/handlers/helpers.go` is write-once. Change the error format:

```go
// helpers.go - yours to edit
func writeError(w http.ResponseWriter, code int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]any{
        "error":   msg,
        "code":    code,
        "traceId": getTraceID(w),  // add observability
    })
}
```

## When to stop using kiln for a table

If a table's API surface is more custom than generated, stop generating it:

```yaml
tables:
  exclude:
    - payments  # too much custom logic, hand-written is better
```

Or keep just the types and OpenAPI:

```yaml
overrides:
  payments:
    disable: [create, update, delete, list, get]
```

You still get validated request/response structs and an OpenAPI spec.
You write the handlers.

## The principle

kiln generates the boring parts. You own the interesting parts. When a table
stops being boring, take it back.
