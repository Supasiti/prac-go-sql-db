package index

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/buffer"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/storage"
)

const testKeySize = 32
const testOrder = 50

func tempEngine(t *testing.T) *storage.LocalFileEngine {
	t.Helper()
	f, err := os.CreateTemp("", "btree-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())

	engine, err := storage.NewLocalFileEngine(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		engine.Close()
		os.Remove(f.Name())
	})
	return engine
}

func makeKey(n int) []byte {
	key := make([]byte, testKeySize)
	binary.LittleEndian.PutUint64(key, uint64(n))
	return key
}

func newTestTree(t *testing.T, order int) (*BTree, *storage.LocalFileEngine) {
	t.Helper()
	engine := tempEngine(t)
	pool := buffer.NewPool(engine, 100)

	rootID, err := pool.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	root := NewLeafNode(rootID)
	page, err := pool.FetchPage(rootID)
	if err != nil {
		t.Fatal(err)
	}
	copy(page.Data[:], root.Serialize(testKeySize))
	pool.MarkDirty(rootID)
	pool.ReleasePage(rootID)

	tree := NewBTree(pool, rootID, testKeySize, order)
	return tree, engine
}

func TestInsertAndSearch(t *testing.T) {
	tree, _ := newTestTree(t, testOrder)

	key := makeKey(42)
	if err := tree.Insert(key, 100); err != nil {
		t.Fatal(err)
	}

	rowID, err := tree.Search(key)
	if err != nil {
		t.Fatal(err)
	}
	if rowID != 100 {
		t.Errorf("rowID = %d, want 100", rowID)
	}
}

func TestInsertMany(t *testing.T) {
	tree, _ := newTestTree(t, testOrder)

	n := 1000
	for i := 0; i < n; i++ {
		key := makeKey(i)
		if err := tree.Insert(key, uint64(i*10)); err != nil {
			t.Fatalf("insert key %d: %v", i, err)
		}
	}

	for i := 0; i < n; i++ {
		key := makeKey(i)
		rowID, err := tree.Search(key)
		if err != nil {
			t.Fatalf("search key %d: %v", i, err)
		}
		if rowID != uint64(i*10) {
			t.Errorf("key %d: rowID = %d, want %d", i, rowID, i*10)
		}
	}
}

func TestInsertCausesSplit(t *testing.T) {
	tree, _ := newTestTree(t, 3)

	for i := 0; i < 4; i++ {
		key := makeKey(i)
		if err := tree.Insert(key, uint64(i)); err != nil {
			t.Fatalf("insert key %d: %v", i, err)
		}
	}

	for i := 0; i < 4; i++ {
		key := makeKey(i)
		rowID, err := tree.Search(key)
		if err != nil {
			t.Fatalf("search key %d: %v", i, err)
		}
		if rowID != uint64(i) {
			t.Errorf("key %d: rowID = %d, want %d", i, rowID, i)
		}
	}
}

func TestSearchMissingKey(t *testing.T) {
	tree, _ := newTestTree(t, testOrder)

	key := makeKey(999)
	_, err := tree.Search(key)
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestDeleteKey(t *testing.T) {
	tree, _ := newTestTree(t, testOrder)

	key := makeKey(42)
	if err := tree.Insert(key, 100); err != nil {
		t.Fatal(err)
	}

	if err := tree.Delete(key); err != nil {
		t.Fatal(err)
	}

	_, err := tree.Search(key)
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound after delete, got %v", err)
	}
}

func TestDeleteCausesMerge(t *testing.T) {
	tree, _ := newTestTree(t, testOrder)

	n := 100
	for i := 0; i < n; i++ {
		key := makeKey(i)
		if err := tree.Insert(key, uint64(i)); err != nil {
			t.Fatalf("insert key %d: %v", i, err)
		}
	}

	for i := 0; i < n; i++ {
		key := makeKey(i)
		if err := tree.Delete(key); err != nil {
			t.Fatalf("delete key %d: %v", i, err)
		}
	}

	for i := 0; i < n; i++ {
		key := makeKey(i)
		_, err := tree.Search(key)
		if err != ErrKeyNotFound {
			t.Errorf("key %d: expected ErrKeyNotFound, got %v", i, err)
		}
	}
}

func TestRangeScan(t *testing.T) {
	tree, _ := newTestTree(t, testOrder)

	for i := 1; i <= 100; i++ {
		key := makeKey(i)
		if err := tree.Insert(key, uint64(i)); err != nil {
			t.Fatalf("insert key %d: %v", i, err)
		}
	}

	from := makeKey(50)
	to := makeKey(60)
	scanner := tree.Scan(from, to)
	defer scanner.Close()

	count := 0
	for {
		key, rowID, ok := scanner.Next()
		if !ok {
			break
		}
		n := int(binary.LittleEndian.Uint64(key))
		if n < 50 || n > 60 {
			t.Errorf("key %d out of range [50, 60]", n)
		}
		if rowID != uint64(n) {
			t.Errorf("key %d: rowID = %d, want %d", n, rowID, n)
		}
		count++
	}

	if count != 11 {
		t.Errorf("scan returned %d entries, want 11", count)
	}
}

func TestScanFullRange(t *testing.T) {
	tree, _ := newTestTree(t, testOrder)

	n := 100
	for i := 1; i <= n; i++ {
		key := makeKey(i)
		if err := tree.Insert(key, uint64(i)); err != nil {
			t.Fatalf("insert key %d: %v", i, err)
		}
	}

	scanner := tree.Scan(nil, nil)
	defer scanner.Close()

	count := 0
	prev := -1
	for {
		key, _, ok := scanner.Next()
		if !ok {
			break
		}
		n := int(binary.LittleEndian.Uint64(key))
		if n <= prev {
			t.Errorf("keys not in order: got %d after %d", n, prev)
		}
		prev = n
		count++
	}

	if count != n {
		t.Errorf("scan returned %d entries, want %d", count, n)
	}
}

func TestInsertDuplicate(t *testing.T) {
	tree, _ := newTestTree(t, testOrder)

	key := makeKey(42)
	if err := tree.Insert(key, 100); err != nil {
		t.Fatal(err)
	}

	err := tree.Insert(key, 200)
	if err != ErrDuplicateKey {
		t.Errorf("expected ErrDuplicateKey, got %v", err)
	}

	rowID, err := tree.Search(key)
	if err != nil {
		t.Fatal(err)
	}
	if rowID != 100 {
		t.Errorf("rowID = %d, want 100 (original value preserved)", rowID)
	}
}

func TestNodeSerializeDeserialize(t *testing.T) {
	node := NewLeafNode(1)
	node.Keys = [][]byte{makeKey(10), makeKey(20), makeKey(30)}
	node.RowIDs = []uint64{100, 200, 300}
	node.NextLeaf = 99

	data := node.Serialize(testKeySize)
	got := Deserialize(data, 1)

	if !got.IsLeaf {
		t.Error("expected leaf node")
	}
	if len(got.Keys) != 3 {
		t.Fatalf("keys len = %d, want 3", len(got.Keys))
	}
	if got.NextLeaf != 99 {
		t.Errorf("nextLeaf = %d, want 99", got.NextLeaf)
	}
	for i, key := range got.Keys {
		expected := makeKey((i + 1) * 10)
		if fmt.Sprintf("%x", key) != fmt.Sprintf("%x", expected) {
			t.Errorf("key[%d] = %x, want %x", i, key, expected)
		}
	}
	if got.RowIDs[0] != 100 || got.RowIDs[1] != 200 || got.RowIDs[2] != 300 {
		t.Errorf("rowIDs = %v, want [100 200 300]", got.RowIDs)
	}
}
