# Module 6: B-Tree Index

## Files

| File | Purpose |
|------|---------|
| `pkg/engine/index/node.go` | Node type, serialize/deserialize |
| `pkg/engine/index/btree.go` | B-tree struct, search/insert/delete |
| `pkg/engine/index/scanner.go` | IndexScanner for range queries |
| `pkg/engine/index/btree_test.go` | Unit tests |

## Page Layout

**Leaf node page:**
```
[isLeaf:1][numKeys:2][nextLeaf:8][pad:5]
[key1Len:2][key1Data:N][rowID1:8]
[key2Len:2][key2Data:N][rowID2:8]
...
```

**Internal node page:**
```
[isLeaf:1][numKeys:2][reserved:8]
[key1Len:2][key1Data:N][childPageID1:8]
[key2Len:2][key2Data:N][childPageID2:8]
...
[childPageID_N+1:8]  // rightmost child
```

## Types

```go
// pkg/engine/index/node.go

type Node struct {
    IsLeaf     bool
    Keys       [][]byte
    ChildIDs   []types.PageID  // internal nodes only: child pointers
    RowIDs     []uint64        // leaf nodes only: row references
    NextLeaf   types.PageID    // leaf nodes: sibling pointer
    PageID     types.PageID
    Dirty      bool
}

// pkg/engine/index/btree.go

type BTree struct {
    pool     *buffer.Pool
    rootID   types.PageID
    keySize  int    // max key size in bytes
    order    int    // max keys per node (branching factor)
}

// pkg/engine/index/scanner.go

type IndexScanner struct {
    tree     *BTree
    current  *Node
    idx      int
    from     []byte
    to       []byte
}
```

## Functions

```go
// pkg/engine/index/node.go
func NewLeafNode(pageID types.PageID) *Node
func NewInternalNode(pageID types.PageID) *Node
func (n *Node) Serialize() []byte       // write node data to byte slice
func Deserialize(data []byte, pageID types.PageID) *Node  // parse bytes into node

// pkg/engine/index/btree.go
func NewBTree(pool *buffer.Pool, rootID types.PageID, keySize, order int) *BTree
func (t *BTree) Search(key []byte) (uint64, error)            // find rowID
func (t *BTree) Insert(key []byte, rowID uint64) error        // insert, split if needed
func (t *BTree) Delete(key []byte) error                      // remove, merge if needed
func (t *BTree) Scan(from, to []byte) *IndexScanner           // range scan

// pkg/engine/index/scanner.go
func (s *IndexScanner) Next() ([]byte, uint64, bool)  // key, rowID, hasMore
func (s *IndexScanner) Close() error
```

## B-Tree Properties

- **Order** = max keys per node (e.g., 50 → max 50 keys, 51 children)
- **Min keys** = order/2 (except root)
- **Leaf nodes** store (key, rowID) pairs
- **Internal nodes** store (key, childPageID) + rightmost child pointer
- **Leaf sibling pointer** for sequential scans
- **No duplicate keys** — insert duplicate → update rowID or error

## Operations

**Search:**
1. Read root node
2. Binary search keys to find child pointer
3. Recurse into child
4. At leaf: binary search for key, return rowID

**Insert:**
1. Find leaf node
2. Insert key+rowID in sorted order
3. If node overflows (numKeys > order):
   - Split: create new right node, move upper half
   - Insert new key+child pointer into parent
   - If parent overflows → recursive split up to root
   - If root splits → new root, tree height +1

**Delete:**
1. Find leaf node
2. Remove key+rowID
3. If node underflows (numKeys < order/2):
   - Try borrow from sibling (redistribute)
   - If can't borrow → merge with sibling
   - Update parent key/pointer
   - If parent underflows → recursive merge
   - If root becomes empty → root = only child, height -1

## Implementation Steps

1. Create `pkg/engine/index/node.go` — Node struct, serialize/deserialize
2. Create `pkg/engine/index/btree.go` — NewBTree, Search
3. Implement Insert with split logic
4. Implement Delete with merge logic
5. Create `pkg/engine/index/scanner.go` — range scan
6. Create `pkg/engine/index/btree_test.go`
7. `make test`

## Test Cases

1. **Insert + search** — insert 1 key, search finds it
2. **Insert many** — 1000 keys, all searchable
3. **Insert causes split** — order=3, insert 4 keys, verify tree grows
4. **Search missing key** — returns error
5. **Delete key** — insert, delete, search returns not found
6. **Delete causes merge** — insert many, delete many, verify tree shrinks
7. **Range scan** — insert 1-100, scan 50-60, verify correct subset
8. **Scan full range** — from=nil, to=nil, returns everything in order
9. **Insert duplicate** — returns error (or updates rowID)

## Key Decisions

- **Binary search within nodes** — O(log n) per node, simple
- **Order parameterized** — adjustable branching factor, default 50
- **Leaf sibling pointer** — enables efficient range scans without parent traversal
- **Fixed max key size** — simplifies serialization, no variable-length headers
- **No duplicate keys** — one rowID per key, simplifies index semantics
