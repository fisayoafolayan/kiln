# Chi Router

kiln supports [chi](https://github.com/go-chi/chi) as an alternative to the Go stdlib router.

## Enable

```yaml
api:
  framework: chi
```

Run `kiln generate` and the router uses chi:

```go
// generated/router.go
func RegisterRoutes(r chi.Router, ...) {
    r.Get("/api/v1/users", users.List)
    r.Post("/api/v1/users", users.Create)
    r.Get("/api/v1/users/{id}", users.Get)
    r.Patch("/api/v1/users/{id}", users.Update)
    r.Delete("/api/v1/users/{id}", users.Delete)
}
```

## What Changes

| | stdlib | chi |
|---|--------|-----|
| Router type | `*http.ServeMux` | `chi.Router` |
| Route registration | `mux.HandleFunc("GET /path", h)` | `r.Get("/path", h)` |
| Import | `net/http` | `github.com/go-chi/chi/v5` |
| Auth middleware | `auth.Middleware(mux)` | `r.Use(auth.Middleware)` |

## What Doesn't Change

- **Handlers** still use `(http.ResponseWriter, *http.Request)` - no chi-specific code
- **Path parameters** still use `r.PathValue("id")` (Go 1.22+, works with chi v5)
- **Store layer** is unchanged
- **Models** are unchanged
- **OpenAPI spec** is unchanged

## Why Chi?

The stdlib router (Go 1.22+) works well for simple APIs. Chi adds:

- Middleware chaining (`r.Use(...)`)
- Route groups (`r.Group(...)`)
- Subrouters
- A large middleware ecosystem

If you don't need these, `framework: stdlib` is simpler.
