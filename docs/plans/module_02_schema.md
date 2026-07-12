# Module 2: Schema & Type System

## Files

| File | Purpose |
|------|---------|
| `pkg/schema/schema.go` | Schema registry, persistence |
| `pkg/schema/reflect.go` | Struct reflection, tag parsing |
| `pkg/schema/types.go` | TableSchema, Column, IndexDef types |
| `pkg/schema/schema_test.go` | Unit tests |

## Types

```go
// pkg/schema/types.go

type TableSchema struct {
    Name       string
    Columns    []Column
    PrimaryKey string
    Indexes    []IndexDef
}

type Column struct {
    Name     string
    DataType reflect.Type
    NotNull  bool
    Unique   bool
    Indexed  bool
    Ordinal  int
}

type IndexDef struct {
    Name    string
    Columns []string
    Unique  bool
}
```

```go
// pkg/schema/schema.go

type Schema struct {
    tables   map[string]*TableSchema
    engine   storage.StorageEngine
}
```

## Functions

```go
// pkg/schema/schema.go
func NewSchema(engine storage.StorageEngine) (*Schema, error)
func (s *Schema) Register(v any) error
func (s *Schema) GetTable(name string) (*TableSchema, error)
func (s *Schema) TableNames() []string
func (s *Schema) save() error            // persist schema to storage
func (s *Schema) load() error            // load schema from storage

// pkg/schema/reflect.go
func parseStruct(v any) (*TableSchema, error)
func parseTag(tag string) (name string, opts map[string]bool)
```

## Struct Tag Format

```go
type User struct {
    ID   int64  `db:"id" primary:"true" autoincrement:"true"`
    Name string `db:"name" notnull:"true" unique:"true"`
    Age  int    `db:"age"`
    Email string `db:"email" index:"true"`
}
```

Tags: `db` (column name), `primary`, `notnull`, `unique`, `index`, `autoincrement`

## Schema Persistence

Schema stored on dedicated pages (page IDs 0 reserved for schema):

```
Page 0: [TableCount:4][Table1Offset:2][Table2Offset:2]...
Page N: [TableData JSON/msgpack]
```

- On `Register()`: parse struct → add to map → save to storage
- On `NewSchema()`: load from storage if pages exist, else fresh
- Format: use `encoding/gob` for simplicity (binary, no external deps)

## Implementation Steps

1. Create `pkg/schema/types.go` — all type definitions
2. Create `pkg/schema/reflect.go` — `parseStruct` + `parseTag`
3. Create `pkg/schema/schema.go` — `NewSchema`, `Register`, `GetTable`, `save`, `load`
4. Create `pkg/schema/schema_test.go` — all test cases
5. `make test`

## Test Cases

1. **Register simple struct** — verify table name, columns extracted
2. **Register struct with all tags** — verify notnull, unique, index, primary
3. **Register duplicate table** — returns error
4. **GetTable existing** — returns correct schema
5. **GetTable non-existing** — returns error
6. **TableNames** — returns all registered tables
7. **Persistence** — register, create new schema from same engine, tables still present
8. **Column ordering** — verify Ordinal matches struct field order
9. **Auto-increment detected** — primary key with autoincrement tag recognized

## Key Decisions

- **`encoding/gob`** for schema persistence — binary, fast, no external deps
- **Page 0 reserved** for schema storage — avoids metadata page management
- **Map-based registry** — O(1) lookup by table name
- **Reflection at register time only** — no reflection hot path during queries
