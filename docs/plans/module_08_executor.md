# Module 8: Query Executor

## Files

| File | Purpose |
|------|---------|
| `pkg/engine/executor/executor.go` | Executor interface, base types |
| `pkg/engine/executor/scan.go` | SeqScan, IndexScan operators |
| `pkg/engine/executor/filter.go` | Filter operator |
| `pkg/engine/executor/sort.go` | Sort operator |
| `pkg/engine/executor/join.go` | NestedLoopJoin operator |
| `pkg/engine/executor/project.go` | Projection operator |
| `pkg/engine/executor/limit.go` | Limit + Offset operator |
| `pkg/engine/executor/plan.go` | QueryPlan → Executor tree builder |
| `pkg/engine/executor/executor_test.go` | Unit tests |

## Types

```go
// pkg/engine/executor/executor.go

type Executor interface {
    Init() error           // prepare (open files, allocate memory)
    Next() (*types.Row, error)  // pull next row, nil = exhausted
    Close() error          // cleanup
}

type Row struct {
    Values map[string]any  // column name → value
}
```

```go
// pkg/engine/executor/scan.go

type SeqScan struct {
    pool    *buffer.Pool
    table   string
    current types.PageID
    offset  int
    tableEnd types.PageID  // last page ID for this table
}

type IndexScan struct {
    tree    *index.BTree
    scanner *index.IndexScanner
}
```

```go
// pkg/engine/executor/filter.go

type Filter struct {
    child Executor
    pred  func(*Row) bool
}
```

```go
// pkg/engine/executor/sort.go

type Sort struct {
    child    Executor
    orderBy  string
    rows     []*Row
    loaded   bool
    idx      int
    compare  func(a, b *Row) int
}
```

```go
// pkg/engine/executor/join.go

type NestedLoopJoin struct {
    left   Executor
    right  Executor
    cond   func(*Row, *Row) bool
    leftRow *Row  // current left row being joined
    rightInit bool
}
```

```go
// pkg/engine/executor/project.go

type Projection struct {
    child Executor
    cols  []string  // columns to keep
}
```

```go
// pkg/engine/executor/limit.go

type LimitOffset struct {
    child   Executor
    limit   int
    offset  int
    skipped int
    yielded int
}
```

```go
// pkg/engine/executor/plan.go

type PlanType uint8

const (
    PlanSeqScan   PlanType = iota
    PlanIndexScan
    PlanFilter
    PlanSort
    PlanJoin
    PlanProject
    PlanLimitOffset
)

type QueryPlan struct {
    Type     PlanType
    Table    string
    Columns  []string
    Where    func(*Row) bool
    OrderBy  string
    Limit    int
    Offset   int
    Left     *QueryPlan   // for join
    Right    *QueryPlan   // for join
    JoinCond func(*Row, *Row) bool
    IndexFrom []byte      // for index scan
    IndexTo   []byte      // for index scan
}
```

## Operator Behavior

**SeqScan:** Read pages sequentially, decode rows from each page, return one at a time.

**IndexScan:** Use BTree.Scan for range, decode rowID → row lookup.

**Filter:** Pull from child, apply predicate, skip non-matching.

**Sort:** Pull ALL from child into memory, sort, then return one at a time.

**NestedLoopJoin:** For each left row, scan all right rows, yield matching pairs.

**Projection:** Pull from child, keep only specified columns.

**LimitOffset:** Skip `offset` rows, then yield up to `limit` rows.

## Pipeline

```
QueryBuilder → QueryPlan → BuildPlan(plan) → Executor tree
                                                      │
                                                      ▼
                                               Init() all nodes
                                                      │
                                                      ▼
                                            while Next() != nil:
                                              yield row to caller
                                                      │
                                                      ▼
                                               Close() all nodes
```

## Functions

```go
// pkg/engine/executor/executor.go
func NewSeqScan(pool *buffer.Pool, table string) *SeqScan
func NewIndexScan(tree *index.BTree, from, to []byte) *IndexScan
func NewFilter(child Executor, pred func(*Row) bool) *Filter
func NewSort(child Executor, orderBy string) *Sort
func NewNestedLoopJoin(left, right Executor, cond func(*Row, *Row) bool) *NestedLoopJoin
func NewProjection(child Executor, cols []string) *Projection
func NewLimitOffset(child Executor, limit, offset int) *LimitOffset

// pkg/engine/executor/plan.go
func BuildPlan(plan *QueryPlan, pool *buffer.Pool, indexes map[string]*index.BTree) Executor
```

## Implementation Steps

1. Create `pkg/engine/executor/executor.go` — interface, Row type
2. Create `pkg/engine/executor/scan.go` — SeqScan, IndexScan
3. Create `pkg/engine/executor/filter.go` — Filter
4. Create `pkg/engine/executor/sort.go` — Sort
5. Create `pkg/engine/executor/join.go` — NestedLoopJoin
6. Create `pkg/engine/executor/project.go` — Projection
7. Create `pkg/engine/executor/limit.go` — LimitOffset
8. Create `pkg/engine/executor/plan.go` — BuildPlan
9. Create `pkg/engine/executor/executor_test.go`
10. `make test`

## Test Cases

1. **SeqScan** — scan table, verify all rows returned
2. **IndexScan** — scan index range, verify correct rows
3. **Filter** — predicate reduces result set
4. **Sort ascending** — verify ordering
5. **Sort descending** — verify reverse ordering
6. **NestedLoopJoin** — cross product of two tables
7. **NestedLoopJoin with condition** — only matching pairs
8. **Projection** — only specified columns in output
9. **Limit** — returns at most N rows
10. **Offset** — skips first N rows
11. **Full pipeline** — SeqScan → Filter → Sort → Projection → Limit
12. **Empty result** — filter matches nothing, returns 0 rows

## Key Decisions

- **Volcano-style (pull)** — simple, one row at a time, low memory
- **Sort loads all into memory** — acceptable for learning, real DBs use external sort
- **NestedLoopJoin only** — hash join / merge join are optimization topics
- **`map[string]any` for row values** — flexible, type conversion at use site
- **`BuildPlan` factory** — turns declarative plan into executable tree
