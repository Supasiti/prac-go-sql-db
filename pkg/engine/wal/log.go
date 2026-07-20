package wal

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

type Log struct {
	file    *os.File
	nextLSN uint64
	mu      sync.Mutex
}

func NewLog(path string) (*Log, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open wal: %w", err)
	}

	log := &Log{file: f}

	entries, err := log.ReadAll()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("read existing wal: %w", err)
	}

	for _, e := range entries {
		if e.LSN >= log.nextLSN {
			log.nextLSN = e.LSN + 1
		}
	}

	return log, nil
}

func (l *Log) Append(entry *Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry.LSN = l.nextLSN
	l.nextLSN++

	data, err := entry.Marshal()
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	if _, err := l.file.Write(data); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	return nil
}

func (l *Log) Sync() error {
	return l.file.Sync()
}

func (l *Log) ReadAll() ([]*Entry, error) {
	if _, err := l.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	var entries []*Entry
	buf := make([]byte, entryHeaderSize)

	for {
		_, err := io.ReadFull(l.file, buf[:entryHeaderSize])
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}

		length := binary.LittleEndian.Uint16(buf[27:29])
		entryBuf := make([]byte, entryHeaderSize+int(length))
		copy(entryBuf[:entryHeaderSize], buf[:entryHeaderSize])

		if length > 0 {
			if _, err := io.ReadFull(l.file, entryBuf[entryHeaderSize:]); err != nil {
				return nil, fmt.Errorf("read data: %w", err)
			}
		}

		entry, err := Unmarshal(entryBuf)
		if err != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func (l *Log) Close() error {
	return l.file.Close()
}
