# Testing Generated Code

kiln doesn't generate tests — testing strategies are too project-specific. But the generated code is designed to be easy to test.

## Testing Handlers

Handlers depend on a store **interface**, not a concrete type. Mock the interface:

```go
package handlers_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "yourmodule/generated/handlers"
    "yourmodule/generated/models"
    "github.com/gofrs/uuid/v5"
)

type mockUserStore struct{}

func (m *mockUserStore) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
    return &models.User{ID: id, Name: "Alice", Email: "alice@example.com"}, nil
}

// ... implement other interface methods

func TestGetUser(t *testing.T) {
    handler := handlers.NewUserHandler(&mockUserStore{})

    req := httptest.NewRequest("GET", "/api/v1/users/"+uuid.Must(uuid.NewV4()).String(), nil)
    w := httptest.NewRecorder()

    handler.Get(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("got status %d, want 200", w.Code)
    }
}
```

## Testing Store Methods

Store methods are single-query functions that work against any `bob.DB`. Use a real test database:

```go
package store_test

import (
    "context"
    "database/sql"
    "testing"

    "yourmodule/generated/models"
    "yourmodule/generated/store"
    "github.com/stephenafamo/bob"
    _ "github.com/jackc/pgx/v5/stdlib"
)

func testDB(t *testing.T) bob.DB {
    t.Helper()
    sqlDB, err := sql.Open("pgx", "postgres://test:test@localhost:5432/test?sslmode=disable")
    if err != nil {
        t.Fatalf("opening test DB: %v", err)
    }
    t.Cleanup(func() { sqlDB.Close() })
    return bob.NewDB(sqlDB)
}

func TestCreateUser(t *testing.T) {
    db := testDB(t)
    s := store.NewUserStore(db)

    user, err := s.Create(context.Background(), models.CreateUserRequest{
        Email: "test@example.com",
        Name:  "Test User",
    })
    if err != nil {
        t.Fatalf("Create: %v", err)
    }
    if user.Email != "test@example.com" {
        t.Errorf("email = %q, want test@example.com", user.Email)
    }
}
```

## Test Isolation

For test isolation, use [txdb](https://github.com/DATA-DOG/go-txdb) to wrap each test in a transaction that rolls back:

```go
import "github.com/DATA-DOG/go-txdb"

func init() {
    txdb.Register("txdb", "pgx", "postgres://test:test@localhost:5432/test?sslmode=disable")
}

func testDB(t *testing.T) bob.DB {
    sqlDB, _ := sql.Open("txdb", t.Name())
    t.Cleanup(func() { sqlDB.Close() })
    return bob.NewDB(sqlDB)
}
```

## Integration Tests

For full HTTP integration tests, wire up the real server:

```go
func TestAPI(t *testing.T) {
    db := testDB(t)
    mux := http.NewServeMux()

    userStore := store.NewUserStore(db)
    userHandler := handlers.NewUserHandler(userStore)
    generated.RegisterRoutes(mux, userHandler)

    server := httptest.NewServer(mux)
    defer server.Close()

    resp, err := http.Get(server.URL + "/api/v1/users")
    if err != nil {
        t.Fatalf("GET /users: %v", err)
    }
    if resp.StatusCode != 200 {
        t.Errorf("status = %d, want 200", resp.StatusCode)
    }
}
```
