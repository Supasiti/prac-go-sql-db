package buffer

import (
	"os"
	"testing"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/storage"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"
)

func tempEngine(t *testing.T) *storage.LocalFileEngine {
	t.Helper()
	f, err := os.CreateTemp("", "buffer-test-*.db")
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

func TestFetchPageReadsFromDisk(t *testing.T) {
	engine := tempEngine(t)
	pool := NewPool(engine, 10)

	id, _ := engine.AllocatePage()
	page := &types.Page{ID: id}
	page.Data[0] = 42
	engine.WritePage(page)

	got, err := pool.FetchPage(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Data[0] != 42 {
		t.Errorf("data = %d, want 42", got.Data[0])
	}
}

func TestFetchPageReturnsCached(t *testing.T) {
	engine := tempEngine(t)
	pool := NewPool(engine, 10)

	id, _ := engine.AllocatePage()
	page := &types.Page{ID: id}
	page.Data[0] = 99
	engine.WritePage(page)

	p1, _ := pool.FetchPage(id)
	p2, _ := pool.FetchPage(id)

	if p1 != p2 {
		t.Error("expected same pointer for cached page")
	}
	if pool.PinCount(id) != 2 {
		t.Errorf("pin count = %d, want 2", pool.PinCount(id))
	}
}

func TestReleasePageDecrementsPin(t *testing.T) {
	engine := tempEngine(t)
	pool := NewPool(engine, 10)

	id, _ := engine.AllocatePage()
	page := &types.Page{ID: id}
	engine.WritePage(page)

	pool.FetchPage(id)
	pool.FetchPage(id)

	if pool.PinCount(id) != 2 {
		t.Fatalf("pin count = %d, want 2", pool.PinCount(id))
	}

	pool.ReleasePage(id)
	if pool.PinCount(id) != 1 {
		t.Errorf("pin count = %d, want 1", pool.PinCount(id))
	}

	pool.ReleasePage(id)
	if pool.PinCount(id) != 0 {
		t.Errorf("pin count = %d, want 0", pool.PinCount(id))
	}
}

func TestMarkDirtyAndFlushPage(t *testing.T) {
	engine := tempEngine(t)
	pool := NewPool(engine, 10)

	id, _ := engine.AllocatePage()
	page := &types.Page{ID: id}
	engine.WritePage(page)

	fetched, _ := pool.FetchPage(id)
	fetched.Data[100] = 77
	pool.MarkDirty(id)

	if err := pool.FlushPage(id); err != nil {
		t.Fatal(err)
	}

	got, _ := engine.ReadPage(id)
	if got.Data[100] != 77 {
		t.Errorf("data = %d, want 77", got.Data[100])
	}
}

func TestFlushWritesAllDirty(t *testing.T) {
	engine := tempEngine(t)
	pool := NewPool(engine, 10)

	ids := make([]types.PageID, 3)
	for i := range ids {
		id, _ := engine.AllocatePage()
		page := &types.Page{ID: id}
		engine.WritePage(page)
		ids[i] = id
	}

	for i, id := range ids {
		fetched, _ := pool.FetchPage(id)
		fetched.Data[0] = byte(i * 10)
		pool.MarkDirty(id)
		pool.ReleasePage(id)
	}

	if err := pool.Flush(); err != nil {
		t.Fatal(err)
	}

	for i, id := range ids {
		got, _ := engine.ReadPage(id)
		if got.Data[0] != byte(i*10) {
			t.Errorf("page %d data = %d, want %d", id, got.Data[0], i*10)
		}
	}
}

func TestEvictionWhenFull(t *testing.T) {
	engine := tempEngine(t)
	pool := NewPool(engine, 2)

	ids := make([]types.PageID, 3)
	for i := range ids {
		id, _ := engine.AllocatePage()
		page := &types.Page{ID: id}
		page.Data[0] = byte(i)
		engine.WritePage(page)
		ids[i] = id
	}

	for _, id := range ids {
		pool.FetchPage(id)
		pool.ReleasePage(id)
	}

	if pool.lru.Len() != 2 {
		t.Errorf("pool size = %d, want 2", pool.lru.Len())
	}

	if pool.PinCount(ids[0]) != 0 {
		t.Error("first page should have been evicted")
	}
}

func TestPinnedPagesNotEvicted(t *testing.T) {
	engine := tempEngine(t)
	pool := NewPool(engine, 2)

	id1, _ := engine.AllocatePage()
	page1 := &types.Page{ID: id1}
	page1.Data[0] = 11
	engine.WritePage(page1)

	id2, _ := engine.AllocatePage()
	page2 := &types.Page{ID: id2}
	page2.Data[0] = 22
	engine.WritePage(page2)

	pool.FetchPage(id1) // pin count = 1
	pool.FetchPage(id2) // pin count = 1

	// Try to add third page — both pinned, should error
	id3, _ := engine.AllocatePage()
	page3 := &types.Page{ID: id3}
	page3.Data[0] = 33
	engine.WritePage(page3)

	_, err := pool.FetchPage(id3)
	if err == nil {
		t.Error("expected error when pool full and all pages pinned")
	}

	// id1 and id2 should still be in cache
	if pool.PinCount(id1) != 1 {
		t.Error("pinned page id1 should not be evicted")
	}
	if pool.PinCount(id2) != 1 {
		t.Error("pinned page id2 should not be evicted")
	}
}

func TestEvictionWritesDirty(t *testing.T) {
	engine := tempEngine(t)
	pool := NewPool(engine, 2)

	id1, _ := engine.AllocatePage()
	page1 := &types.Page{ID: id1}
	engine.WritePage(page1)

	id2, _ := engine.AllocatePage()
	page2 := &types.Page{ID: id2}
	engine.WritePage(page2)

	fetched, _ := pool.FetchPage(id1)
	fetched.Data[50] = 88
	pool.MarkDirty(id1)
	pool.ReleasePage(id1)

	pool.FetchPage(id2)
	pool.ReleasePage(id2)

	// Evict id1 (dirty) — should write to storage
	evicted, err := pool.Evict()
	if err != nil {
		t.Fatal(err)
	}
	if evicted != id1 {
		t.Errorf("evicted = %d, want %d", evicted, id1)
	}

	got, _ := engine.ReadPage(id1)
	if got.Data[50] != 88 {
		t.Errorf("dirty data lost: got %d, want 88", got.Data[50])
	}
}

func TestFetchEvictedPage(t *testing.T) {
	engine := tempEngine(t)
	pool := NewPool(engine, 2)

	id1, _ := engine.AllocatePage()
	page1 := &types.Page{ID: id1}
	page1.Data[0] = 55
	engine.WritePage(page1)

	id2, _ := engine.AllocatePage()
	page2 := &types.Page{ID: id2}
	engine.WritePage(page2)

	id3, _ := engine.AllocatePage()
	page3 := &types.Page{ID: id3}
	engine.WritePage(page3)

	pool.FetchPage(id1)
	pool.ReleasePage(id1)

	pool.FetchPage(id2)
	pool.ReleasePage(id2)

	// id1 evicted when id3 fetched
	pool.FetchPage(id3)
	pool.ReleasePage(id3)

	// Fetch id1 again — should reload from disk
	got, err := pool.FetchPage(id1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Data[0] != 55 {
		t.Errorf("data = %d, want 55 after re-fetch", got.Data[0])
	}
}
