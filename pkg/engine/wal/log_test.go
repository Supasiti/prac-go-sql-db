package wal

import (
	"os"
	"testing"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/buffer"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/storage"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"
)

func tempPath(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "wal-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func tempEngine(t *testing.T) *storage.LocalFileEngine {
	t.Helper()
	f, err := os.CreateTemp("", "wal-test-*.db")
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

func TestAppendAndReadBack(t *testing.T) {
	path := tempPath(t)
	log, err := NewLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	entry := &Entry{
		TxnID:  1,
		Type:   EntryBegin,
		PageID: 10,
		Offset: 0,
		Length: 0,
		Data:   nil,
	}

	if err := log.Append(entry); err != nil {
		t.Fatal(err)
	}

	entries, err := log.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].TxnID != 1 {
		t.Errorf("txnid = %d, want 1", entries[0].TxnID)
	}
	if entries[0].Type != EntryBegin {
		t.Errorf("type = %d, want EntryBegin", entries[0].Type)
	}
	if entries[0].PageID != 10 {
		t.Errorf("pageid = %d, want 10", entries[0].PageID)
	}
	if entries[0].LSN != 0 {
		t.Errorf("lsn = %d, want 0", entries[0].LSN)
	}
}

func TestAppendMultiple(t *testing.T) {
	path := tempPath(t)
	log, err := NewLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	for i := 0; i < 10; i++ {
		entry := &Entry{
			TxnID:  uint64(i),
			Type:   EntryPageWrite,
			PageID: types.PageID(i + 1),
			Offset: 0,
			Length: 4,
			Data:   []byte{byte(i), byte(i + 1), byte(i + 2), byte(i + 3)},
		}
		if err := log.Append(entry); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := log.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 10 {
		t.Fatalf("got %d entries, want 10", len(entries))
	}

	for i, e := range entries {
		if e.TxnID != uint64(i) {
			t.Errorf("entry %d: txnid = %d, want %d", i, e.TxnID, i)
		}
		if e.LSN != uint64(i) {
			t.Errorf("entry %d: lsn = %d, want %d", i, e.LSN, i)
		}
	}
}

func TestChecksumValid(t *testing.T) {
	entry := &Entry{
		LSN:    42,
		TxnID:  7,
		Type:   EntryPageWrite,
		PageID: 3,
		Offset: 100,
		Length: 8,
		Data:   []byte{1, 2, 3, 4, 5, 6, 7, 8},
	}

	data, err := entry.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	if !got.VerifyChecksum() {
		t.Error("checksum verification failed for valid entry")
	}
	if got.LSN != entry.LSN {
		t.Errorf("lsn = %d, want %d", got.LSN, entry.LSN)
	}
	if got.TxnID != entry.TxnID {
		t.Errorf("txnid = %d, want %d", got.TxnID, entry.TxnID)
	}
	if got.Type != entry.Type {
		t.Errorf("type = %d, want %d", got.Type, entry.Type)
	}
	if got.PageID != entry.PageID {
		t.Errorf("pageid = %d, want %d", got.PageID, entry.PageID)
	}
	if got.Offset != entry.Offset {
		t.Errorf("offset = %d, want %d", got.Offset, entry.Offset)
	}
	if got.Length != entry.Length {
		t.Errorf("length = %d, want %d", got.Length, entry.Length)
	}
	if len(got.Data) != len(entry.Data) {
		t.Fatalf("data len = %d, want %d", len(got.Data), len(entry.Data))
	}
	for i := range got.Data {
		if got.Data[i] != entry.Data[i] {
			t.Errorf("data[%d] = %d, want %d", i, got.Data[i], entry.Data[i])
		}
	}
}

func TestChecksumCorruption(t *testing.T) {
	entry := &Entry{
		LSN:    1,
		TxnID:  1,
		Type:   EntryCommit,
		PageID: 0,
		Offset: 0,
		Length: 4,
		Data:   []byte{10, 20, 30, 40},
	}

	data, err := entry.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	data[0] ^= 0xFF

	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	if got.VerifyChecksum() {
		t.Error("checksum should fail for corrupted entry")
	}
}

func TestEntryTypes(t *testing.T) {
	types := []EntryType{EntryBegin, EntryCommit, EntryAbort, EntryPageWrite, EntryCheckpoint}
	names := []string{"Begin", "Commit", "Abort", "PageWrite", "Checkpoint"}

	for i, typ := range types {
		entry := &Entry{
			TxnID:  1,
			Type:   typ,
			PageID: 0,
			Offset: 0,
			Length: 0,
			Data:   nil,
		}

		data, err := entry.Marshal()
		if err != nil {
			t.Fatalf("%s: marshal: %v", names[i], err)
		}

		got, err := Unmarshal(data)
		if err != nil {
			t.Fatalf("%s: unmarshal: %v", names[i], err)
		}

		if got.Type != typ {
			t.Errorf("%s: type = %d, want %d", names[i], got.Type, typ)
		}
	}
}

func TestRecoveryRedo(t *testing.T) {
	logPath := tempPath(t)

	f, err := os.CreateTemp("", "wal-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	f.Close()
	os.Remove(dbPath)
	t.Cleanup(func() { os.Remove(dbPath) })

	engine, err := storage.NewLocalFileEngine(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	id, err := engine.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	page := &types.Page{ID: id}
	page.Data[0] = 1
	page.Data[1] = 2
	if err := engine.WritePage(page); err != nil {
		t.Fatal(err)
	}

	wal, err := NewLog(logPath)
	if err != nil {
		t.Fatal(err)
	}

	begin := &Entry{TxnID: 1, Type: EntryBegin}
	if err := wal.Append(begin); err != nil {
		t.Fatal(err)
	}

	pageWrite := &Entry{
		TxnID:  1,
		Type:   EntryPageWrite,
		PageID: id,
		Offset: 0,
		Length: 2,
		Data:   []byte{10, 20},
	}
	if err := wal.Append(pageWrite); err != nil {
		t.Fatal(err)
	}

	commit := &Entry{TxnID: 1, Type: EntryCommit}
	if err := wal.Append(commit); err != nil {
		t.Fatal(err)
	}

	wal.Close()
	engine.Close()

	newEngine, err := storage.NewLocalFileEngine(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer newEngine.Close()
	newPool := buffer.NewPool(newEngine, 10)

	if err := Recover(logPath, newPool); err != nil {
		t.Fatal(err)
	}

	fetched, err := newPool.FetchPage(id)
	if err != nil {
		t.Fatal(err)
	}

	if fetched.Data[0] != 10 {
		t.Errorf("data[0] = %d, want 10", fetched.Data[0])
	}
	if fetched.Data[1] != 20 {
		t.Errorf("data[1] = %d, want 20", fetched.Data[1])
	}
}

func TestRecoveryUndo(t *testing.T) {
	logPath := tempPath(t)

	f, err := os.CreateTemp("", "wal-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	f.Close()
	os.Remove(dbPath)
	t.Cleanup(func() { os.Remove(dbPath) })

	engine, err := storage.NewLocalFileEngine(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	id, err := engine.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	page := &types.Page{ID: id}
	page.Data[0] = 1
	page.Data[1] = 2
	if err := engine.WritePage(page); err != nil {
		t.Fatal(err)
	}

	wal, err := NewLog(logPath)
	if err != nil {
		t.Fatal(err)
	}

	begin := &Entry{TxnID: 1, Type: EntryBegin}
	if err := wal.Append(begin); err != nil {
		t.Fatal(err)
	}

	pageWrite := &Entry{
		TxnID:  1,
		Type:   EntryPageWrite,
		PageID: id,
		Offset: 0,
		Length: 2,
		Data:   []byte{99, 99},
	}
	if err := wal.Append(pageWrite); err != nil {
		t.Fatal(err)
	}

	wal.Close()
	engine.Close()

	newEngine, err := storage.NewLocalFileEngine(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer newEngine.Close()
	newPool := buffer.NewPool(newEngine, 10)

	if err := Recover(logPath, newPool); err != nil {
		t.Fatal(err)
	}

	fetched, err := newPool.FetchPage(id)
	if err != nil {
		t.Fatal(err)
	}

	if fetched.Data[0] != 1 {
		t.Errorf("data[0] = %d, want 1 (should not be changed)", fetched.Data[0])
	}
	if fetched.Data[1] != 2 {
		t.Errorf("data[1] = %d, want 2 (should not be changed)", fetched.Data[1])
	}
}

func TestCheckpoint(t *testing.T) {
	logPath := tempPath(t)
	engine := tempEngine(t)
	pool := buffer.NewPool(engine, 10)

	id, err := engine.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	page := &types.Page{ID: id}
	page.Data[0] = 1
	if err := engine.WritePage(page); err != nil {
		t.Fatal(err)
	}

	fetched, err := pool.FetchPage(id)
	if err != nil {
		t.Fatal(err)
	}
	fetched.Data[0] = 55
	pool.MarkDirty(id)

	wal, err := NewLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	if err := Checkpoint(wal, pool); err != nil {
		t.Fatal(err)
	}

	got, err := engine.ReadPage(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Data[0] != 55 {
		t.Errorf("data[0] = %d, want 55", got.Data[0])
	}

	entries, err := wal.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range entries {
		if e.Type == EntryCheckpoint {
			found = true
			break
		}
	}
	if !found {
		t.Error("checkpoint entry not found in WAL")
	}
}
