# Module 5: Write-Ahead Log (WAL)

## Files

| File | Purpose |
|------|---------|
| `pkg/engine/wal/entry.go` | Entry types, serialization |
| `pkg/engine/wal/log.go` | Log writer, append/sync |
| `pkg/engine/wal/recovery.go` | Crash recovery, checkpoint |
| `pkg/engine/wal/log_test.go` | Unit tests |

## Entry Format

```
[LSN:8][TxnID:8][Type:1][PageID:8][Offset:2][Length:2][Data:N][Checksum:4]
Total header = 29 bytes + N data bytes
```

- `LSN` — Log Sequence Number, monotonically increasing
- `TxnID` — Transaction that created this entry
- `Type` — Entry type (see below)
- `PageID` — Affected page (0 for non-page entries)
- `Offset` — Byte offset within page where change starts
- `Length` — Bytes of change data
- `Data` — Actual page bytes changed (before image for undo, after image for redo)
- `Checksum` — CRC32 of all preceding fields

## Types

```go
// pkg/engine/wal/entry.go

type EntryType uint8

const (
    EntryBegin      EntryType = iota // Start of transaction
    EntryCommit                      // Commit transaction
    EntryAbort                       // Abort transaction
    EntryPageWrite                   // Page modification
    EntryCheckpoint                  // Checkpoint marker
)

type Entry struct {
    LSN      uint64
    TxnID    uint64
    Type     EntryType
    PageID   types.PageID
    Offset   uint16
    Length   uint16
    Data     []byte
    Checksum uint32
}
```

```go
// pkg/engine/wal/log.go

type Log struct {
    file     *os.File
    nextLSN  uint64
    mu       sync.Mutex
}
```

## Functions

```go
// pkg/engine/wal/entry.go
func (e *Entry) Marshal() ([]byte, error)     // serialize to bytes
func Unmarshal(data []byte) (*Entry, error)    // deserialize from bytes
func (e *Entry) ComputeChecksum() uint32
func (e *Entry) VerifyChecksum() bool

// pkg/engine/wal/log.go
func NewLog(path string) (*Log, error)
func (l *Log) Append(entry *Entry) error
func (l *Log) Sync() error
func (l *Log) ReadAll() ([]*Entry, error)
func (l *Log) Close() error

// pkg/engine/wal/recovery.go
func Recover(logPath string, pool *buffer.Pool) error
func Checkpoint(log *Log, pool *buffer.Pool) error
```

## WAL Protocol

**Write path (every page modification):**
1. Build Entry with Type=EntryPageWrite
2. Compute checksum
3. Append to log file
4. Sync log to disk (fsync)
5. Modify page in buffer pool
6. Mark page dirty

**Recovery on startup:**
1. Open WAL file, ReadAll entries
2. Build last checkpoint LSN
3. Build set of committed txn IDs after checkpoint
4. Forward scan: redo EntryPageWrite for committed txns
5. Forward scan: undo (restore before-image) for uncommitted txns
6. Write checkpoint entry at end

**Checkpoint:**
1. Flush all dirty pages from buffer pool to storage
2. Sync storage
3. Append EntryCheckpoint to WAL
4. Sync WAL

## Implementation Steps

1. Create `pkg/engine/wal/entry.go` — Entry type, Marshal/Unmarshal, checksum
2. Create `pkg/engine/wal/log.go` — Log struct, NewLog, Append, Sync, ReadAll, Close
3. Create `pkg/engine/wal/recovery.go` — Recover, Checkpoint
4. Create `pkg/engine/wal/log_test.go`
5. `make test`

## Test Cases

1. **Append + read back** — write entry, ReadAll, verify match
2. **Append multiple** — 10 entries, verify all 10 returned in order
3. **Checksum valid** — marshal entry, verify checksum matches
4. **Checksum corruption** — flip bit, VerifyChecksum returns false
5. **Entry types** — verify all 5 types serialize/deserialize
6. **Recovery redo** — write committed page entries, crash, recover, verify pages restored
7. **Recovery undo** — write uncommitted entries, crash, recover, verify pages not changed
8. **Checkpoint** — checkpoint flushes dirty pages, writes marker

## Key Decisions

- **CRC32 checksum** — fast, sufficient for integrity (not security)
- **Separate WAL file** — not in data file, avoids mixing sequential + random I/O
- **Log-first protocol** — write WAL entry before page modification ensures durability
- **`sync` after every append** — simple correctness, can optimize later (batch sync)
- **Before-image in PageWrite** — enables undo during recovery
