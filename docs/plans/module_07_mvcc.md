# Module 7: MVCC Transactions

## Files

| File | Purpose |
|------|---------|
| `pkg/engine/txn/txn.go` | Transaction struct + Manager |
| `pkg/engine/txn/version.go` | RowVersion, version chain logic |
| `pkg/engine/txn/conflict.go` | Conflict detection |
| `pkg/engine/txn/gc.go` | Garbage collection of old versions |
| `pkg/engine/txn/txn_test.go` | Unit tests |

## Types

```go
// pkg/engine/txn/txn.go

type TxnStatus uint8

const (
    TxnActive   TxnStatus = iota
    TxnCommitted
    TxnAborted
)

type Transaction struct {
    ID       uint64
    Snapshot uint64    // timestamp at Begin — determines visibility
    Status   TxnStatus
    writes   []*WriteRecord
}

type WriteRecord struct {
    Table      string
    Key        []byte
    OldVersion *RowVersion  // for undo
    NewVersion *RowVersion  // for redo
}

type Manager struct {
    engine storage.StorageEngine
    pool   *buffer.Pool
    nextID uint64         // monotonically increasing txn IDs
    active map[uint64]*Transaction
    mu     sync.Mutex
}
```

```go
// pkg/engine/txn/version.go

type RowVersion struct {
    Data    []byte    // serialized row data
    BeginTS uint64    // txn ID that created this version
    EndTS   uint64    // 0 = still visible, >0 = superseded by txn EndTS
    TxnID   uint64    // txn that owns this version
}
```

## Visibility Rules

A version is visible to txn with Snapshot=S if:
```
version.BeginTS <= S AND (version.EndTS == 0 OR version.EndTS > S)
```

- `BeginTS <= S` — version existed before my snapshot
- `EndTS == 0` — version not yet superseded, OR
- `EndTS > S` — version superseded after my snapshot (still visible to me)

## Functions

```go
// pkg/engine/txn/txn.go
func NewManager(engine storage.StorageEngine, pool *buffer.Pool) *Manager
func (m *Manager) Begin() (*Transaction, error)
func (m *Manager) Commit(txn *Transaction) error
func (m *Manager) Rollback(txn *Transaction) error

// pkg/engine/txn/version.go
func (m *Manager) GetVisible(table string, key []byte, snapshot uint64) (*RowVersion, error)
func (m *Manager) Put(table string, key []byte, data []byte, txn *Transaction) error
func (m *Manager) Delete(table string, key []byte, txn *Transaction) error
func (m *Manager) applyWrites(txn *Transaction) error

// pkg/engine/txn/conflict.go
func (m *Manager) ConflictCheck(txn *Transaction) error
func (m *Manager) hasConflict(txn *Transaction, other *Transaction) bool

// pkg/engine/txn/gc.go
func (m *Manager) GC() error
func (m *Manager) oldestActiveTxn() uint64
```

## Write Path

**Put(table, key, data, txn):**
1. Validate txn is Active
2. Get current visible version for (table, key) at txn.Snapshot
3. Create NewVersion with BeginTS = txn.ID, TxnID = txn.ID
4. Create OldVersion (current visible, for undo)
5. Append WriteRecord to txn.writes

**Commit(txn):**
1. ConflictCheck — verify no write-write conflicts
2. For each WriteRecord:
   - Set OldVersion.EndTS = txn.ID (old version superseded)
   - Write NewVersion to storage
3. Set txn.Status = Committed
4. Remove from active map

**Rollback(txn):**
1. Discard all WriteRecords (no storage writes)
2. Set txn.Status = Aborted
3. Remove from active map

**GetVisible(table, key, snapshot):**
1. Scan all versions for (table, key)
2. Return first version matching visibility rules
3. If no visible version → ErrNotFound

## Conflict Detection

**Write-write conflict:**
- Two active txns both write same (table, key)
- When txn A tries to commit:
  - Check if any other active txn wrote same key
  - If yes → conflict detected
  - Resolution: abort younger txn (lower ID = older, higher = younger)

## Garbage Collection

**Goal:** Remove old versions no active transaction can see.

**Algorithm:**
1. Find oldest active txn ID (min Snapshot among active txns)
2. Scan all versions
3. A version V can be deleted if:
   - V.EndTS != 0 (superseded) AND
   - V.EndTS < oldestActiveTxn (no active txn can see it)
4. Delete qualifying versions

**Frequency:** Run periodically or after N commits.

## Implementation Steps

1. Create `pkg/engine/txn/version.go` — RowVersion type
2. Create `pkg/engine/txn/txn.go` — Transaction, Manager, Begin/Commit/Rollback
3. Implement GetVisible with visibility rules
4. Implement Put/Delete with version creation
5. Create `pkg/engine/txn/conflict.go` — ConflictCheck, hasConflict
6. Create `pkg/engine/txn/gc.go` — GC, oldestActiveTxn
7. Create `pkg/engine/txn/txn_test.go`
8. `make test`

## Test Cases

1. **Begin + commit** — start txn, write row, commit, verify visible
2. **Begin + rollback** — start txn, write row, rollback, verify not visible
3. **Snapshot isolation** — txn A reads row, txn B modifies + commits, txn A still sees old version
4. **Write-write conflict** — two txns write same row, second commit fails
5. **Non-conflicting writes** — two txns write different rows, both commit
6. **GC removes old versions** — create multiple versions, GC cleans superseded ones
7. **GC respects active txns** — old version kept if active txn can still see it
8. **Delete version** — delete creates tombstone version (EndTS set)
9. **Read deleted row** — after delete commit, new txn sees no row

## Key Decisions

- **TxnID as timestamp** — monotonically increasing, simple ordering
- **Optimistic concurrency** — conflicts detected at commit, not at write
- **No lock manager yet** — Module 10 adds locking
- **Tombstone for deletes** — delete = version with special marker, GC cleans later
- **In-memory active map** — fine for single goroutine (Module 10 adds concurrency)
