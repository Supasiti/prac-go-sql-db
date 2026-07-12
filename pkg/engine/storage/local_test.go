package storage

import (
	"os"
	"testing"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"
)

func tempPath(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "test-db-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())
	return f.Name()
}

func TestCreateNewFile(t *testing.T) {
	path := tempPath(t)
	defer os.Remove(path)

	engine, err := NewLocalFileEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if engine.header.Magic != magic {
		t.Errorf("magic = %v, want %v", engine.header.Magic, magic)
	}
	if engine.header.PageCount != 0 {
		t.Errorf("page count = %d, want 0", engine.header.PageCount)
	}
	if engine.header.Version != 1 {
		t.Errorf("version = %d, want 1", engine.header.Version)
	}
}

func TestAllocatePages(t *testing.T) {
	path := tempPath(t)
	defer os.Remove(path)

	engine, err := NewLocalFileEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	for i := types.PageID(0); i < 5; i++ {
		id, err := engine.AllocatePage()
		if err != nil {
			t.Fatal(err)
		}
		if id != i {
			t.Errorf("allocated id = %d, want %d", id, i)
		}
	}
}

func TestWriteReadBack(t *testing.T) {
	path := tempPath(t)
	defer os.Remove(path)

	engine, err := NewLocalFileEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	id, _ := engine.AllocatePage()

	page := &types.Page{ID: id}
	for i := 0; i < 256; i++ {
		page.Data[i] = byte(i)
	}

	if err := engine.WritePage(id, page); err != nil {
		t.Fatal(err)
	}

	got, err := engine.ReadPage(id)
	if err != nil {
		t.Fatal(err)
	}

	if got.Data != page.Data {
		t.Error("read data does not match written data")
	}
}

func TestReadUnallocatedPage(t *testing.T) {
	path := tempPath(t)
	defer os.Remove(path)

	engine, err := NewLocalFileEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.ReadPage(0)
	if err != ErrPageNotFound {
		t.Errorf("expected ErrPageNotFound, got %v", err)
	}
}

func TestMultiplePages(t *testing.T) {
	path := tempPath(t)
	defer os.Remove(path)

	engine, err := NewLocalFileEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	pages := make([]*types.Page, 100)
	for i := range pages {
		id, _ := engine.AllocatePage()
		page := &types.Page{ID: id}
		for j := range page.Data {
			page.Data[j] = byte(i)
		}
		pages[i] = page
		if err := engine.WritePage(id, page); err != nil {
			t.Fatal(err)
		}
	}

	for i, want := range pages {
		got, err := engine.ReadPage(want.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Data != want.Data {
			t.Errorf("page %d data mismatch", i)
		}
	}
}

func TestSync(t *testing.T) {
	path := tempPath(t)
	defer os.Remove(path)

	engine, err := NewLocalFileEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	id, _ := engine.AllocatePage()
	page := &types.Page{ID: id}
	engine.WritePage(id, page)

	if err := engine.Sync(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseReopen(t *testing.T) {
	path := tempPath(t)
	defer os.Remove(path)

	engine, err := NewLocalFileEngine(path)
	if err != nil {
		t.Fatal(err)
	}

	id, _ := engine.AllocatePage()
	page := &types.Page{ID: id}
	for i := range page.Data {
		page.Data[i] = 42
	}
	engine.WritePage(id, page)
	engine.Close()

	engine2, err := NewLocalFileEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	defer engine2.Close()

	if engine2.header.PageCount != 1 {
		t.Errorf("page count after reopen = %d, want 1", engine2.header.PageCount)
	}

	got, err := engine2.ReadPage(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Data != page.Data {
		t.Error("data mismatch after reopen")
	}
}

func TestClosePreventsOps(t *testing.T) {
	path := tempPath(t)
	defer os.Remove(path)

	engine, err := NewLocalFileEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	engine.Close()

	_, err = engine.ReadPage(0)
	if err != ErrFileClosed {
		t.Errorf("expected ErrFileClosed, got %v", err)
	}

	err = engine.WritePage(0, &types.Page{})
	if err != ErrFileClosed {
		t.Errorf("expected ErrFileClosed, got %v", err)
	}

	_, err = engine.AllocatePage()
	if err != ErrFileClosed {
		t.Errorf("expected ErrFileClosed, got %v", err)
	}

	err = engine.Sync()
	if err != ErrFileClosed {
		t.Errorf("expected ErrFileClosed, got %v", err)
	}
}
