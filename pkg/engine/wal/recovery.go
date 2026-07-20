package wal

import (
	"fmt"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/buffer"
)

func Recover(logPath string, pool *buffer.Pool) error {
	log, err := NewLog(logPath)
	if err != nil {
		return fmt.Errorf("open wal: %w", err)
	}
	defer log.Close()

	entries, err := log.ReadAll()
	if err != nil {
		return fmt.Errorf("read wal: %w", err)
	}

	var checkpointLSN uint64
	for _, e := range entries {
		if e.Type == EntryCheckpoint {
			checkpointLSN = e.LSN
		}
	}

	committed := make(map[uint64]bool)
	for _, e := range entries {
		if e.LSN <= checkpointLSN {
			continue
		}
		switch e.Type {
		case EntryCommit:
			committed[e.TxnID] = true
		case EntryAbort:
			committed[e.TxnID] = false
		}
	}

	for _, e := range entries {
		if e.LSN <= checkpointLSN {
			continue
		}
		if e.Type != EntryPageWrite {
			continue
		}
		if !committed[e.TxnID] {
			continue
		}

		page, err := pool.FetchPage(e.PageID)
		if err != nil {
			return fmt.Errorf("fetch page %d: %w", e.PageID, err)
		}
		copy(page.Data[e.Offset:e.Offset+e.Length], e.Data)
		pool.MarkDirty(e.PageID)
		pool.ReleasePage(e.PageID)
	}

	_ = pool.Flush()

	return nil
}

func Checkpoint(log *Log, pool *buffer.Pool) error {
	if err := pool.Flush(); err != nil {
		return fmt.Errorf("flush pool: %w", err)
	}

	if err := pool.Sync(); err != nil {
		return fmt.Errorf("sync storage: %w", err)
	}

	entry := &Entry{
		Type: EntryCheckpoint,
	}
	if err := log.Append(entry); err != nil {
		return fmt.Errorf("append checkpoint: %w", err)
	}

	return log.Sync()
}
