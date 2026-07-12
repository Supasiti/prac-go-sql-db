# Module 3: Buffer Pool

## Files

| File | Purpose |
|------|---------|
| `pkg/engine/buffer/pool.go` | BufferPool struct + all methods |
| `pkg/engine/buffer/pool_test.go` | Unit tests |

## Types

```go
// pkg/engine/buffer/pool.go

type Pool struct {
    engine  storage.StorageEngine
    frames  map[types.PageID]*Frame
    lru     *list.List  // front = most recent, back = least recent
    maxSize int
}

type Frame struct {
    page     *types.Page
    pinCount int
    dirty    bool
    element  *list.Element  // position in LRU list
}
```

## Functions

```go
func NewPool(engine storage.StorageEngine, maxSize int) *Pool
func (p *Pool) FetchPage(id types.PageID) (*types.Page, error)
func (p *Pool) ReleasePage(id types.PageID)
func (p *Pool) MarkDirty(id types.PageID)
func (p *Pool) FlushPage(id types.PageID) error
func (p *Pool) Flush() error
func (p *Pool) Evict() (types.PageID, error)
func (p *Pool) PinCount(id types.PageID) int
```

## Behavior

| Method | Behavior |
|--------|----------|
| `FetchPage(id)` | If in cache → move to front of LRU, increment pin, return. If not → read from disk into new frame, pin=1, add to LRU. If pool full → evict first. |
| `ReleasePage(id)` | Decrement pin. If pin=0 → eligible for eviction. |
| `MarkDirty(id)` | Set dirty=true on frame. Triggers write on eviction/flush. |
| `FlushPage(id)` | If dirty → write to storage, mark clean. |
| `Flush()` | Flush all dirty pages. |
| `Evict()` | Walk LRU back-to-front. Find frame with pinCount=0. If dirty → flush first. Remove from cache, return its PageID. If all pinned → error. |
| `PinCount(id)` | Return current pin count (0 if not cached). |

## Eviction Policy

LRU (Least Recently Used):
- `FetchPage` moves accessed page to front
- `Evict` scans from back (least recently used)
- Pinned pages (pinCount > 0) skipped

## Implementation Steps

1. Create `pkg/engine/buffer/pool.go` — Pool struct, NewPool
2. Implement `FetchPage` — cache hit + cache miss + eviction
3. Implement `ReleasePage` — pin decrement
4. Implement `MarkDirty` — dirty tracking
5. Implement `FlushPage` + `Flush` — write dirty pages
6. Implement `Evict` — LRU eviction with dirty write-back
7. Create `pkg/engine/buffer/pool_test.go`
8. `make test`

## Test Cases

1. **FetchPage reads from disk** — allocate page, write via storage, fetch via pool
2. **FetchPage returns cached** — fetch twice, verify same pointer
3. **ReleasePage decrements pin** — fetch, release, verify pin count 0
4. **MarkDirty + FlushPage** — mark dirty, flush, verify page written to storage
5. **Flush writes all dirty** — multiple dirty pages, flush all, verify storage
6. **Eviction when full** — pool size 2, fetch 3 pages, verify first evicted
7. **Pinned pages not evicted** — pin page, fill pool, verify pinned survives
8. **Eviction writes dirty** — dirty page evicted → data written to storage
9. **Fetch evicted page** — fetch, release, fetch again, verify data intact

## Key Decisions

- **`container/list`** for LRU — simple, O(1) move/access, stdlib
- **Frame pointer in map** — avoid copy, direct mutation
- **Pin count** — prevents eviction of pages in active use (critical for B-tree traversal)
- **No background flush** — synchronous for now, simpler reasoning
- **`Evict` returns evicted PageID** — caller can log or track
