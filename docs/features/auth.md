# Authentication

kiln generates auth middleware as a write-once file. You choose the strategy, kiln generates the skeleton, you add your validation logic.

## Strategies

### None (default)

```yaml
auth:
  strategy: none
```

No auth middleware generated. All endpoints are public.

### API Key

```yaml
auth:
  strategy: api_key
  header: X-API-Key
```

Generates `generated/auth/middleware.go` with a middleware that reads the API key from the configured header. The generated file has a TODO where you add your key validation:

```go
// generated/auth/middleware.go - THIS FILE IS YOURS
func Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := r.Header.Get("X-API-Key")
        if key == "" {
            writeError(w, http.StatusUnauthorized, "missing api key")
            return
        }
        // TODO: validate the key against your database or config
        next.ServeHTTP(w, r)
    })
}
```

### JWT

```yaml
auth:
  strategy: jwt
  header: Authorization
```

Generates middleware that extracts a Bearer token from the Authorization header. You add your JWT verification logic (issuer, audience, signing key).

## How It's Wired

=== "stdlib"

    ```go
    // cmd/server/main.go
    handler := auth.Middleware(mux)
    http.ListenAndServe(addr, handler)
    ```

=== "chi"

    ```go
    // cmd/server/main.go
    r := chi.NewRouter()
    r.Use(auth.Middleware)
    generated.RegisterRoutes(r, ...)
    http.ListenAndServe(addr, r)
    ```

## Customizing

The middleware file is write-once. Edit it freely:

- Add database lookups for API keys
- Add JWT verification with your signing key
- Add role-based access control
- Add rate limiting per key
- Extract user info into request context

kiln will never overwrite your changes.
