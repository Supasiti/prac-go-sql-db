# Module 9: S3 Storage Backend

## Files

| File | Purpose |
|------|---------|
| `pkg/engine/storage/s3.go` | S3StorageEngine implementation |
| `pkg/engine/storage/s3_test.go` | Unit tests |
| `go.mod` | Add AWS SDK dependency |

## Types

```go
// pkg/engine/storage/s3.go

type S3StorageEngine struct {
    client    *s3.Client
    bucket    string
    prefix    string
    header    header        // reuse from local.go
    mu        sync.Mutex
    closed    bool
}

type S3Config struct {
    Bucket   string
    Prefix   string  // e.g. "mydb/"
    Region   string
    Endpoint string  // optional, for localstack/testing
}
```

## S3 Key Layout

```
{prefix}header           → header page (magic, page count, version)
{prefix}pages/{pageID}   → data page N
```

Example with prefix `mydb/`:
```
mydb/header
mydb/pages/0
mydb/pages/1
mydb/pages/2
...
```

## Functions

```go
// pkg/engine/storage/s3.go
func NewS3StorageEngine(cfg S3Config) (*S3StorageEngine, error)
func (e *S3StorageEngine) ReadPage(id types.PageID) (*types.Page, error)
func (e *S3StorageEngine) WritePage(id types.PageID, page *types.Page) error
func (e *S3StorageEngine) AllocatePage() (types.PageID, error)
func (e *S3StorageEngine) Sync() error   // no-op, S3 is durable
func (e *S3StorageEngine) Close() error

// Internal helpers
func (e *S3StorageEngine) pageKey(id types.PageID) string
func (e *S3StorageEngine) readHeader() error
func (e *S3StorageEngine) writeHeader() error
```

## Behavior

| Method | Behavior |
|--------|----------|
| `NewS3StorageEngine` | Create S3 client, check if header exists, create if not |
| `ReadPage(id)` | GetObject from `{prefix}pages/{id}`, read into Page |
| `WritePage(id)` | PutObject to `{prefix}pages/{id}` |
| `AllocatePage()` | Read header, increment PageCount, PutObject header |
| `Sync()` | No-op (S3 provides durability) |
| `Close()` | Mark closed, no further ops |

## Dependencies

```bash
go get github.com/aws/aws-sdk-go-v2
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/s3
```

## Implementation Steps

1. `go get github.com/aws/aws-sdk-go-v2/...`
2. Create `pkg/engine/storage/s3.go`
3. Implement S3StorageEngine with all interface methods
4. Create `pkg/engine/storage/s3_test.go`
5. `make test`

## Test Cases

**Unit tests (require localstack or mock):**
1. **Create new bucket** — verify header written
2. **Write + read page** — write known data, read back, verify
3. **Allocate pages** — sequential IDs
4. **Read unallocated** — returns error
5. **Multiple pages** — 10 pages, all readable
6. **Close prevents ops** — returns ErrFileClosed

**Integration test (same suite as LocalFileEngine):**
7. **Run full Module 1 test suite** — swap LocalFileEngine for S3StorageEngine, all 8 original tests pass

## Test Setup

Option A: **localstack** (Docker)
```bash
docker run -d -p 4566:4566 localstack/localstack
export AWS_ENDPOINT=http://localhost:4566
```

Option B: **minio** (local S3)
```bash
docker run -d -p 9000:9000 minio/minio server /data
```

Option C: **Mock** — implement StorageEngine with in-memory map, no real S3

## Key Decisions

- **Same interface** — S3StorageEngine implements StorageEngine, drop-in replacement
- **Header page in S3** — same format as local, stores page count
- **AllocatePage reads header** — not cached (adds latency), simple correctness
  - Optimization: cache header locally, sync on close
- **Sync is no-op** — S3 provides 11 nines durability, no fsync needed
- **`sync.Mutex`** — serialize AllocatePage to prevent race on header
- **Prefix-based** — multiple databases can share one S3 bucket
