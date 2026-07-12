# Module 10: Concurrency (Bonus)

## Files

| File | Purpose |
|------|---------|
| `pkg/engine/buffer/pool.go` | Add RWMutex per frame |
| `pkg/engine/index/btree.go` | Add latch coupling |
| `pkg/engine/txn/lock.go` | Lock manager (new file) |
| `pkg/engine/txn/lock_test.go` | Lock manager tests |
| `pkg/engine/concurrency_test.go` | Integration concurrency tests |

## Changes to Existing Code

### Buffer Pool — RWMutex per frame

```go
// pkg/engine/buffer/pool.go

type Frame struct {
    page     *types.Page
    pinCount int
    dirty    bool
    element  *list.Element
    mu       sync.RWMutex  // NEW: per-page latch
}
```

**Changes:**
- `FetchPage` → `frame.mu.RLock()` when returning cached page
- `WritePage` → `frame.mu.Lock()` for write access
- `ReleasePage` → `frame.mu.RUnlock()`
- `MarkDirty` → `frame.mu.Lock()`
- Eviction → `frame.mu.Lock()` to write + remove

### B-Tree — Latch Coupling

**Problem:** Reading a node needs its child locked, but parent no longer needed.

**Solution:** Latch coupling (crabbing):
1. Lock parent
2. Lock child
3. Unlock parent
4. Repeat

```go
// pkg/engine/index/btree.go

func (t *BTree) Search(key []byte) (uint64, error) {
    // Lock root
    root := t.fetchNode(t.rootID)
    root.mu.Lock()
    defer root.mu.Unlock()

    for !root.IsLeaf {
        childID := root.findChild(key)
        child := t.fetchNode(childID)
        child.mu.Lock()
        root.mu.Unlock()  // release parent after child locked
        root = child
    }
    // search within leaf...
}
```

**Insert/Delete** use safe-insert/safe-delete:
- Lock root + first child
- If child safe (won't split/merge) → unlock ancestors
- If child unsafe → keep ancestors locked until safe node found

### Lock Manager (new)

```go
// pkg/engine/txn/lock.go

type LockMode uint8

const (
    LockShared    LockMode = iota  // readers
    LockExclusive                   // writers
)

type LockManager struct {
    locks    map[string]*LockState  // "table:key" → lock state
    mu       sync.Mutex
}

type LockState struct {
    holders map[uint64]LockMode  // txnID → current mode
    waiters []LockRequest
}

type LockRequest struct {
    txnID uint64
    mode  LockMode
    granted chan struct{}
}
```

## Functions

```go
// pkg/engine/txn/lock.go
func NewLockManager() *LockManager
func (lm *LockManager) Acquire(table string, key []byte, mode LockMode, txnID uint64) error
func (lm *LockManager) Release(table string, key []byte, txnID uint64)
func (lm *LockManager) ReleaseAll(txnID uint64)
func (lm *LockManager) DetectDeadlock() uint64  // return txnID to abort, 0 if none
```

## Lock Compatibility

| Held \ Requested | Shared | Exclusive |
|------------------|--------|-----------|
| **Shared**       | ✅     | ❌        |
| **Exclusive**    | ❌     | ❌        |

- Multiple readers allowed
- Writer blocks everyone
- Reader blocks writer

## Deadlock Detection

**Wait-for graph:**
- Nodes = active transactions
- Edge A→B = A waiting for lock held by B
- Cycle = deadlock

**Algorithm:**
1. Build wait-for graph from lock waiter lists
2. DFS from each node, detect back edge
3. Return youngest txn (highest ID) in cycle → abort it

**Implementation:**
```go
func (lm *LockManager) DetectDeadlock() uint64 {
    // Build adjacency list
    // DFS for cycle
    // Return youngest txn in cycle
}
```

## Integration with MVCC

**Commit path (Module 7 updated):**
1. Acquire exclusive locks on all keys being written
2. ConflictCheck
3. Apply versions
4. Release all locks
5. Mark committed

**Read path:**
1. Acquire shared lock on key (optional for snapshot isolation)
2. GetVisible with snapshot
3. Release shared lock

## Implementation Steps

1. Add RWMutex to `pkg/engine/buffer/pool.go` Frame
2. Update pool methods to use locks
3. Add latch coupling to `pkg/engine/index/btree.go`
4. Create `pkg/engine/txn/lock.go` — LockManager
5. Create `pkg/engine/txn/lock_test.go`
6. Update Module 7 commit path to use lock manager
7. Create `pkg/engine/concurrency_test.go` — integration tests
8. `make test`

## Test Cases

### Buffer Pool
1. **Concurrent FetchPage** — multiple goroutines fetch same page, no corruption
2. **Concurrent write + read** — writer holds write lock, reader blocks

### B-Tree
3. **Concurrent search** — multiple readers traverse tree simultaneously
4. **Concurrent insert** — inserts don't corrupt tree structure

### Lock Manager
5. **Two readers** — both acquire shared lock, both succeed
6. **Reader + writer** — reader holds shared, writer blocks until release
7. **Two writers** — first holds exclusive, second blocks
8. **Lock release** — blocking txn unblocked after release
9. **Deadlock detection** — two txns waiting on each other, one aborted
10. **ReleaseAll** — commit/rollback releases all locks at once

### Integration
11. **Concurrent inserts** — 10 goroutines insert different rows, all committed
12. **Concurrent read + write** — reader sees consistent snapshot during writes
13. **Stress test** — 100 goroutines, mixed reads/writes, no data corruption

## Key Decisions

- **Per-frame RWMutex** — fine-grained, multiple pages can be read concurrently
- **Latch coupling** — standard B-tree concurrency technique
- **Pessimistic locking** — acquire locks before operation (vs optimistic in MVCC)
- **Deadlock detection** — cycle detection, abort youngest (simple, fair)
- **No lock升级** — shared → exclusive upgrade not supported (can add later)
- **Lock granularity** — row-level (table:key), not page-level
