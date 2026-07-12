# Module 3: Type-Safe Query Builder

## Files

| File | Purpose |
|------|---------|
| `pkg/engine/api.go` | Public DB handle, Open/Register |
| `pkg/engine/query/find.go` | FindQuery[T] builder |
| `pkg/engine/query/insert.go` | InsertQuery builder |
| `pkg/engine/query/update.go` | UpdateQuery builder |
| `pkg/engine/query/delete.go` | DeleteQuery[T] builder |
| `pkg/engine/query/plan.go` | QueryPlan types |
| `pkg/engine/query/find_test.go` | Find tests |
| `pkg/engine/query/insert_test.go` | Insert tests |
| `pkg/engine/query/mutation_test.go` | Update + Delete tests |

## Types

```go
// pkg/engine/api.go

type DB struct {
    schema *schema.Schema
    pool   *buffer.Pool
}

func Open(engine storage.StorageEngine) (*DB, error)
func (db *DB) Register(v any) error
func (db *DB) Find[T any]() *FindQuery[T]
func (db *DB) Insert(rows ...any) *InsertQuery
func (db *DB) Update(table string, fn func(map[string]any)) *UpdateQuery
func (db *DB) Delete[T any]() *DeleteQuery[T]
```

```go
// pkg/engine/query/find.go

type FindQuery[T any] struct {
    db      *DB
    table   string
    cols    []string
    where   func(*T) bool
    orderBy string
    limit   int
    offset  int
}

func (q *FindQuery[T]) Select(cols ...string) *FindQuery[T]
func (q *FindQuery[T]) Where(fn func(*T) bool) *FindQuery[T]
func (q *FindQuery[T]) OrderBy(col string) *FindQuery[T]
func (q *FindQuery[T]) Limit(n int) *FindQuery[T]
func (q *FindQuery[T]) Offset(n int) *FindQuery[T]
func (q *FindQuery[T]) Execute(ctx context.Context) ([]T, error)
```

```go
// pkg/engine/query/insert.go

type InsertQuery struct {
    db    *DB
    table string
    rows  []any
}

func (q *InsertQuery) Execute(ctx context.Context) error
```

```go
// pkg/engine/query/update.go

type UpdateQuery struct {
    db      *DB
    table   string
    changes func(map[string]any)
    where   func(map[string]any) bool
}

func (q *UpdateQuery) Where(fn func(map[string]any) bool) *UpdateQuery
func (q *UpdateQuery) Execute(ctx context.Context) (int64, error) // rows affected
```

```go
// pkg/engine/query/delete.go

type DeleteQuery[T any] struct {
    db    *DB
    table string
    where func(*T) bool
}

func (q *DeleteQuery[T]) Where(fn func(*T) bool) *DeleteQuery[T]
func (q *DeleteQuery[T]) Execute(ctx context.Context) (int64, error)
```

```go
// pkg/engine/query/plan.go

type QueryPlan struct {
    Type    PlanType // Scan, Filter, Sort, Limit, Insert, Update, Delete
    Table   string
    Columns []string
    Where   any     // predicate (typed or func)
    OrderBy string
    Limit   int
    Offset  int
    Rows    []any   // for insert
    Updates func(map[string]any) // for update
}
```

## API Examples

```go
// Find
users, err := db.Find[User]().
    Select("Name", "Age").
    Where(func(u *User) bool { return u.Age > 18 }).
    OrderBy("Name").
    Limit(10).
    Execute(ctx)

// Insert
err := db.Insert(&User{Name: "Alice", Age: 30}, &User{Name: "Bob", Age: 25}).Execute(ctx)

// Update
rowsAffected, err := db.Update("users", func(row map[string]any) {
    row["age"] = row["age"].(int) + 1
}).Where(func(row map[string]any) bool {
    return row["name"] == "Alice"
}).Execute(ctx)

// Delete
rowsAffected, err := db.Delete[User]().
    Where(func(u *User) bool { return u.Age < 18 }).
    Execute(ctx)
```

## Implementation Steps

1. Create `pkg/engine/query/plan.go` — plan types
2. Create `pkg/engine/query/find.go` — FindQuery builder + Execute
3. Create `pkg/engine/query/insert.go` — InsertQuery builder + Execute
4. Create `pkg/engine/query/update.go` — UpdateQuery builder + Execute
5. Create `pkg/engine/query/delete.go` — DeleteQuery builder + Execute
6. Create `pkg/engine/api.go` — DB handle, Open, Register
7. Create test files
8. `make test`

## Test Cases

### Find
1. **Find all** — `Find[User]().Execute()` returns all rows
2. **Select columns** — `Find[User]().Select("Name").Execute()` returns subset
3. **Where clause** — filters correctly
4. **OrderBy** — results sorted
5. **Limit + Offset** — pagination works

### Insert
6. **Single row** — inserts, verify with Find
7. **Multiple rows** — batch insert
8. **Wrong type** — compile-time error (type safety)

### Update
9. **Update with where** — modifies matching rows
10. **Rows affected count** — correct number returned

### Delete
11. **Delete with where** — removes matching rows
12. **Delete all** — no where clause deletes everything

## Key Decisions

- **Generic Find/Delete** — type parameter T = struct type, compiler enforces column names
- **Map-based Update/Delete** — more flexible for dynamic column access
- **Execute(ctx)** — context support for cancellation/timeout
- **No SQL string anywhere** — all logic stays in Go types, impossible to inject
- **QueryPlan intermediate** — builds plan first, executor runs it (Module 8)
