# Course Plan: Build SQL DB in Go

## Goal

Build minimal but complete SQL database engine in Go.
Type-safe SDK-like API. Pluggable storage (local file → S3).
Full ACID. B-tree indexing. MVCC transactions.

## Architecture

```
┌─────────────────────────────────┐
│   Type-Safe Query API (SDK)     │  Compile-time table awareness
│   db.Find[User]().Where(...)    │  No string queries
└──────────────┬──────────────────┘
               │
┌──────────────▼──────────────────┐
│   Query Executor                │  Scan, filter, join, aggregate
│   Planner → Executor pipeline   │
└──────────────┬──────────────────┘
               │
┌──────────────▼──────────────────┐
│   Transaction Manager (MVCC)    │  Snapshot isolation, conflict detect
│   Version chains, GC            │
└──────────────┬──────────────────┘
               │
┌──────────────▼──────────────────┐
│   B-Tree Index                  │  Primary + secondary indexes
│   Page-oriented, fan-out        │
└──────────────┬──────────────────┘
               │
┌──────────────▼──────────────────┐
│   Buffer Pool                   │  LRU page cache, pin/unpin, dirty tracking
└──────────────┬──────────────────┘
               │
┌──────────────▼──────────────────┐
│   Write-Ahead Log (WAL)         │  Crash recovery via log replay
│   Sequential append, checkpoint │
└──────────────┬──────────────────┘
               │
┌──────────────▼──────────────────┐
│   Storage Engine Interface      │  Pluggable backends
│   ReadPage / WritePage / Sync   │
│  ┌───────────┐  ┌────────────┐ │
│  │ LocalFile │  │ S3Storage  │ │
│  └───────────┘  └────────────┘ │
└─────────────────────────────────┘
```

## Project Structure

```
prac-go-sql-db/
├── go.mod
├── cmd/
│   └── dbshell/           # REPL for manual testing
│       └── main.go
├── pkg/
│   ├── schema/            # Table definitions, constraints
│   ├── engine/            # Core DB engine
│   │   ├── storage/       # Storage engine interface + impls
│   │   ├── buffer/        # Buffer pool
│   │   ├── wal/           # Write-ahead log
│   │   ├── index/         # B-tree index
│   │   ├── txn/           # Transaction manager (MVCC)
│   │   ├── executor/      # Query execution
│   │   └── api.go         # Public type-safe API
│   └── types/             # Shared types (Page, Row, Value, etc.)
├── tests/                 # Integration tests
└── docs/
    └── plans/
        └── 2026_07_12_course_plan.md
```

## Modules

### Module 1: Storage Engine Interface

**Concepts:** Page-based storage, pluggable backends, I/O abstraction.

**Deliverables:**
- `Page` type — fixed-size (4KB), byte slice with header
- `StorageEngine` interface:
  ```go
  type StorageEngine interface {
      ReadPage(pageID uint64) (*Page, error)
      WritePage(page *Page) error
      AllocatePage() (uint64, error)
      Sync() error
      Close() error
  }
  ```
- `LocalFileEngine` — stores pages in single file
  - File header: magic number, page count, schema version
  - Page slots: fixed-size, seek by pageID * pageSize
- Basic file format: header page + data pages
- Tests: write pages, read back, verify integrity

---

### Module 2: Schema & Type System

**Concepts:** Struct-to-table mapping, type-safe column references, constraints as metadata.

**Depends on:** Module 1 (storage for persistence).

**Deliverables:**
- `Schema` registry — register Go structs as tables
- Struct tags for column mapping: `db:"name"`, `primary`, `notnull`, `unique`, `index`
- `Column` type with name, Go type, constraints
- `TableSchema` — holds column defs, primary key, indexes list
- Compile-time table awareness: `db.Table[User]()` returns typed table ref
- Persist schema to storage engine (schema pages)

**Key types:**
```go
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
}
```

---

### Module 3: Type-Safe Query Builder

**Concepts:** Fluent API, builder pattern, no string interpolation, compiler-enforced correctness.

**Depends on:** Module 2 (schema knowledge).

**Deliverables:**
- `Find[T]()` — returns typed query builder
- `Insert[T](rows ...T)` — type-checked insert
- `Update[T](func(*T))` — type-checked update
- `Delete[T]()` — type-checked delete
- `Where` clause with typed predicates: `.Where(func(t *T) bool)`
- `OrderBy`, `Limit`, `Offset`
- Chained API, builds internal query plan (no SQL string anywhere)

**Example:**
```go
result, err := db.Find[User]().
    Select("Name", "Age").
    Where(func(u *User) bool { return u.Age > 18 }).
    OrderBy("Name").
    Limit(10).
    Execute(ctx)
```

---

### Module 4: Buffer Pool

**Concepts:** Page caching, LRU eviction, pin counts, dirty pages, flushing.

**Depends on:** Module 1 (storage engine).

**Deliverables:**
- `BufferPool` struct — in-memory cache of pages
- LRU eviction when pool full
- `Pin(pageID)` / `Unpin(pageID)` — prevent eviction of active pages
- Dirty page tracking — modified pages marked for write-back
- `Flush()` — write all dirty pages to storage
- `FetchPage(pageID)` — get page (from cache or disk), increment pin
- `ReleasePage(pageID)` — decrement pin
- Integration: query executor uses buffer pool, not storage directly

---

### Module 5: Write-Ahead Log (WAL)

**Concepts:** Durability, crash recovery, log-structured writes, sequential I/O.

**Depends on:** Module 1 (storage), Module 4 (buffer pool).

**Deliverables:**
- WAL entry format:
  ```
  [LSN:8][TxnID:8][Type:1][PageID:8][Offset:2][Length:2][Data:N][Checksum:4]
  ```
- Entry types: BEGIN, COMMIT, ABORT, PAGE_WRITE, CHECKPOINT
- Log writer — append-only, sequential writes
- Recovery protocol on startup:
  1. Read WAL from last checkpoint
  2. Replay PAGE_WRITE entries (redo committed txns)
  3. Undo uncommitted txns
  4. Write checkpoint marker
- Checkpoint — flush dirty pages, mark checkpoint LSN in WAL
- Integration: every page modification goes through WAL first

---

### Module 6: B-Tree Index

**Concepts:** Tree-structured index, page-oriented B-tree, split/merge, index scans.

**Depends on:** Module 1 (storage), Module 4 (buffer pool).

**Deliverables:**
- B-tree node layout in pages:
  ```
  [isLeaf:1][numKeys:2][nextLeaf:8][keys:...][pointers:...]
  ```
- Operations:
  - Search: traverse root → leaf, binary search within nodes
  - Insert: find leaf, insert, split if full (propagate split up)
  - Delete: find leaf, remove, merge/redistribute if underflow
- Leaf pages store (key, rowID) pairs
- Internal pages store (key, childPageID) pairs
- Primary key index (clustered) vs secondary index
- Index scan: iterate leaf pages via next pointer
- Integration: query executor picks index scan vs sequential scan

---

### Module 7: MVCC Transactions

**Concepts:** Snapshot isolation, multi-version concurrency control, version chains, conflict detection.

**Depends on:** Module 1 (storage), Module 4 (buffer pool), Module 5 (WAL for durability).

**Deliverables:**
- Transaction struct:
  ```go
  type Transaction struct {
      ID       uint64
      Snapshot uint64  // read timestamp
      Status   TxnStatus // ACTIVE, COMMITTED, ABORTED
  }
  ```
- Version chain: each row has `beginTS` and `endTS` fields
  - Visible if: `beginTS <= mySnapshot` and (`endTS == 0` or `endTS > mySnapshot`)
- Write path:
  1. Begin txn
  2. Write create new version (old version gets endTS)
  3. On commit: set commitTS, make versions visible
  4. On abort: mark versions as aborted
- Conflict detection (write-write):
  - If two txns write same row, second one blocks or aborts
- Garbage collection: periodically clean old versions no visible txn needs
- Integration: `Begin()`, `Commit()`, `Rollback()` on db handle

---

### Module 8: Query Executor

**Concepts:** Volcano-style execution, operator pipeline, scan strategies.

**Depends on:** Module 2 (schema), Module 3 (query builder), Module 6 (index).

**Deliverables:**
- Executor interface: `Next() (Row, error)` — pull-based iterator
- Operators:
  - `SequentialScan` — full table scan
  - `IndexScan` — use B-tree index
  - `Filter` — evaluate predicate, skip non-matching
  - `Projection` — select subset of columns
  - `Sort` — in-memory sort
  - `Limit` / `Offset`
  - `NestedLoopJoin` — basic join strategy
- Query plan:
  - Planner picks scan type (index vs sequential) based on WHERE clause
  - Builds operator tree
  - Executor pulls rows through pipeline
- Integration: query builder produces plan, executor runs it

---

### Module 9: S3 Storage Backend

**Concepts:** Cloud storage trade-offs, abstraction payoff, eventual consistency considerations.

**Depends on:** Module 1 (storage interface).

**Deliverables:**
- `S3StorageEngine` implementing `StorageEngine` interface
- Page storage in S3:
  - Key format: `/{db-name}/pages/{pageID}.bin`
  - Or: prefix-based partitioning for large datasets
- Trade-offs vs local:
  - Higher latency per read (network round-trip)
  - Better durability (replication)
  - Higher throughput for large sequential reads (S3 select, multipart)
- Configuration:
  ```go
  db, err := engine.New(engine.S3Config{
      Bucket: "my-db-bucket",
      Prefix: "mydb/",
      Region: "us-east-1",
  })
  ```
- Same API, swap backend only at init
- Integration test: run full test suite against S3 backend

---

### Module 10: Concurrency (Bonus)

**Concepts:** Latch coupling, lock manager, deadlock handling.

**Depends on:** Module 4 (buffer pool), Module 6 (B-tree), Module 7 (MVCC).

**Deliverables:**
- Buffer pool: RWMutex per page slot
- B-tree: latch coupling (lock child, unlock parent during traversal)
- Lock manager: table-level and row-level locks
- Lock modes: shared (read), exclusive (write)
- Deadlock detection: wait-for graph, cycle detection → abort youngest txn
- Integration: concurrent reads don't block, concurrent writes properly serialized

---

## Module Execution Order

| # | Module | Depends On | Key Concept |
|---|--------|-----------|-------------|
| 1 | Storage Engine Interface | — | Page-based pluggable storage |
| 2 | Schema & Type System | 1 | Struct → table mapping |
| 3 | Query Builder | 2 | Type-safe fluent API |
| 4 | Buffer Pool | 1 | In-memory page cache |
| 5 | WAL | 1, 4 | Crash recovery |
| 6 | B-Tree Index | 1, 4 | Fast lookups |
| 7 | MVCC Transactions | 1, 4, 5 | Snapshot isolation |
| 8 | Query Executor | 2, 3, 6 | Operator pipeline |
| 9 | S3 Backend | 1 | Storage swap proof |
| 10 | Concurrency | 4, 6, 7 | Parallel access |

## Testing Strategy

- Each module: unit tests + integration test
- Module 7+: full CRUD test suite runs against every new feature
- Module 9: same test suite runs against S3 backend
- Module 10: stress test with concurrent goroutines
- Final: end-to-end test — create schema, insert rows, query with index, transaction isolation, crash recovery

## Learning Outcomes

After completing all modules, you will understand:
- How real databases organize data on disk (pages, files)
- Why buffer pools exist and how LRU eviction works
- How WAL provides durability without synchronous disk writes
- How B-tree indexes accelerate queries
- How MVCC enables concurrent reads without blocking
- How query executors build and run operator pipelines
- How storage abstraction enables backend swaps
- How to design type-safe APIs that eliminate entire classes of bugs
