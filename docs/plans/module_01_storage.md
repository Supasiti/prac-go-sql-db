# Module 1: Storage Engine Interface

## Files

| File | Purpose |
|------|---------|
| `go.mod` | Init module |
| `pkg/types/page.go` | `Page`, `PageID`, `PageSize` constants |
| `pkg/engine/storage/engine.go` | `StorageEngine` interface |
| `pkg/engine/storage/local.go` | `LocalFileEngine` — file-backed impl |
| `pkg/engine/storage/local_test.go` | Unit tests |

## Types

```go
// pkg/types/page.go
type PageID uint64
const PageSize = 4096

type Page struct {
    ID   PageID
    Data [PageSize]byte
}
```

```go
// pkg/engine/storage/engine.go
type StorageEngine interface {
    ReadPage(id PageID) (*Page, error)
    WritePage(page *Page) error
    AllocatePage() (PageID, error)
    Sync() error
    Close() error
}
```

## LocalFileEngine Design

**File layout:**
```
[Header Page (4KB)] [Data Page 0] [Data Page 1] [Data Page 2] ...
```

**Header page:**
```
[Magic:4 bytes][PageCount:8][Version:4][Reserved:4076]
```

- `Magic` = `"GSQL"` — file validation on open
- `PageCount` — total allocated pages (increment on AllocatePage)
- `Version` — schema version, start at 1

**Page addressing:**
- Data page N lives at file offset `(N + 1) * PageSize` (+1 for header)

**Operations:**
| Method | Behavior |
|--------|----------|
| `NewLocalFileEngine(path)` | Create/open file, read/validate header |
| `ReadPage(id)` | Seek to offset, read 4KB into `Page.Data` |
| `WritePage(page)` | Seek to offset, write `Page.Data` using `page.ID` |
| `AllocatePage()` | Increment header PageCount, write header, return new ID |
| `Sync()` | `file.Sync()` — force OS flush to disk |
| `Close()` | Write header, sync, close file |

## Test Cases

1. **Create new file** — verify header written, magic correct, page count = 0
2. **Allocate pages** — IDs sequential (0, 1, 2...)
3. **Write + read back** — write known pattern to page, read, verify byte-equal
4. **Read unallocated page** — returns error
5. **Multiple pages** — allocate 100 pages, write unique data to each, read all back
6. **Sync** — write page, sync, verify no error
7. **Close + reopen** — write pages, close, reopen file, verify pages still readable
8. **Close prevents ops** — close engine, call ReadPage → error

## Implementation Steps

1. `go mod init github.com/tharatornsupasiti/prac-go-sql-db`
2. Create `pkg/types/page.go`
3. Create `pkg/engine/storage/engine.go` — interface only
4. Create `pkg/engine/storage/local.go` — `LocalFileEngine` struct + all methods
5. Create `pkg/engine/storage/local_test.go` — all 8 test cases
6. `go test ./pkg/engine/storage/...` — verify pass

## Key Decisions

- **Fixed page size (4KB)** — matches OS page size, simple for now
- **Header page** — validates file integrity, stores metadata
- **PageID = uint64** — supports massive datasets, no overflow worry
- **No encryption/compression** — later modules or S3 backend concern

## Implementation Log

### What was built

- Module initialized: `go mod init github.com/tharatornsupasiti/prac-go-sql-db`
- `pkg/types/page.go` — `PageID` (uint64), `Page` (ID + 4KB Data array), `PageSize` constant
- `pkg/engine/storage/engine.go` — `StorageEngine` interface with 5 methods
- `pkg/engine/storage/local.go` — `LocalFileEngine` implementation:
  - Uses `os.OpenFile` with `O_RDWR|O_CREATE` — creates file if missing
  - Header page uses `binary.LittleEndian` for encoding
  - `ReadAt`/`WriteAt` for offset-based page I/O (no seeking, safe for concurrent reads later)
  - `closed` bool flag — all ops return `ErrFileClosed` after close
  - `Close()` writes header → syncs → closes file (idempotent)
- `pkg/engine/storage/local_test.go` — 8 test cases using `os.CreateTemp` for isolated test files

### Errors encountered

None. All tests passed first run.

### Test results

```
=== RUN   TestCreateNewFile       --- PASS
=== RUN   TestAllocatePages       --- PASS
=== RUN   TestWriteReadBack       --- PASS
=== RUN   TestReadUnallocatedPage --- PASS
=== RUN   TestMultiplePages       --- PASS
=== RUN   TestSync                --- PASS
=== RUN   TestCloseReopen         --- PASS
=== RUN   TestClosePreventsOps    --- PASS
PASS  ok  0.468s
```

### Design notes for future modules

- **`WritePage(page)` not `WritePage(id, page)`** — page already has ID, no redundancy
- **`ReadAt`/`WriteAt` over `Seek`+`Read`/`Write`** — avoids shared offset state, safe when buffer pool adds concurrent access
- **PageID starts at 0** — data page 0 is at file offset `1 * PageSize` (after header)
- **AllocatePage writes header immediately** — simple, small overhead, ensures page count persists even on crash
- **Header is one full page (4KB)** — wastes space but keeps everything page-aligned, can store more metadata later (root page ID, free list head, etc.)
