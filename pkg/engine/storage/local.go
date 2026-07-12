package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"
)

var (
	ErrFileClosed    = errors.New("engine closed")
	ErrPageNotFound  = errors.New("page not allocated")
	ErrBadMagic      = errors.New("invalid file header")
	ErrCorruptedPage = errors.New("page data corrupted")
)

var magic = [4]byte{'G', 'S', 'Q', 'L'}

const headerSize = types.PageSize

type header struct {
	Magic         [4]byte
	PageCount     uint64
	SchemaPageCount uint64
	Version       uint32
	_             [headerSize - 4 - 8 - 8 - 4]byte // padding
}

type LocalFileEngine struct {
	path   string
	file   *os.File
	header header
	closed bool
}

func NewLocalFileEngine(path string) (*LocalFileEngine, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	engine := &LocalFileEngine{path: path, file: f}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat file: %w", err)
	}

	if stat.Size() == 0 {
		engine.header = header{Magic: magic, Version: 1}
		if err := engine.writeHeader(); err != nil {
			f.Close()
			return nil, fmt.Errorf("write initial header: %w", err)
		}
	} else {
		if err := engine.readHeader(); err != nil {
			f.Close()
			return nil, fmt.Errorf("read header: %w", err)
		}
		if engine.header.Magic != magic {
			f.Close()
			return nil, ErrBadMagic
		}
	}

	return engine, nil
}

func (e *LocalFileEngine) ReadPage(id types.PageID) (*types.Page, error) {
	if e.closed {
		return nil, ErrFileClosed
	}
	if uint64(id) >= e.header.PageCount {
		return nil, ErrPageNotFound
	}

	offset := int64((uint64(id) + 1)) * types.PageSize
	page := &types.Page{ID: id}

	if _, err := e.file.ReadAt(page.Data[:], offset); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read page %d: %w", id, ErrCorruptedPage)
		}
		return nil, fmt.Errorf("read page %d: %w", id, err)
	}

	return page, nil
}

func (e *LocalFileEngine) WritePage(page *types.Page) error {
	if e.closed {
		return ErrFileClosed
	}
	if uint64(page.ID) >= e.header.PageCount {
		return ErrPageNotFound
	}

	offset := int64((uint64(page.ID) + 1)) * types.PageSize
	if _, err := e.file.WriteAt(page.Data[:], offset); err != nil {
		return fmt.Errorf("write page %d: %w", page.ID, err)
	}

	return nil
}

func (e *LocalFileEngine) AllocatePage() (types.PageID, error) {
	if e.closed {
		return 0, ErrFileClosed
	}

	id := types.PageID(e.header.PageCount)
	e.header.PageCount++

	if err := e.writeHeader(); err != nil {
		return 0, fmt.Errorf("write header: %w", err)
	}

	return id, nil
}

func (e *LocalFileEngine) AllocateSchemaPage() (types.PageID, error) {
	if e.closed {
		return 0, ErrFileClosed
	}

	id := types.PageID(e.header.SchemaPageCount)
	if uint64(id) < e.header.PageCount {
		return 0, fmt.Errorf("schema page %d already allocated as data page", id)
	}

	e.header.SchemaPageCount++
	e.header.PageCount++

	if err := e.writeHeader(); err != nil {
		return 0, fmt.Errorf("write header: %w", err)
	}

	return id, nil
}

func (e *LocalFileEngine) SetSchemaPageCount(count uint64) error {
	if e.closed {
		return ErrFileClosed
	}
	e.header.SchemaPageCount = count
	return e.writeHeader()
}

func (e *LocalFileEngine) SchemaPageCount() uint64 {
	return e.header.SchemaPageCount
}

func (e *LocalFileEngine) Sync() error {
	if e.closed {
		return ErrFileClosed
	}
	return e.file.Sync()
}

func (e *LocalFileEngine) Path() string {
	return e.path
}

func (e *LocalFileEngine) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true

	if err := e.writeHeader(); err != nil {
		e.file.Close()
		return fmt.Errorf("write header on close: %w", err)
	}
	if err := e.Sync(); err != nil {
		e.file.Close()
		return fmt.Errorf("sync on close: %w", err)
	}
	return e.file.Close()
}

func (e *LocalFileEngine) readHeader() error {
	var buf [headerSize]byte
	if _, err := e.file.ReadAt(buf[:], 0); err != nil {
		return err
	}

	e.header.Magic = [4]byte{buf[0], buf[1], buf[2], buf[3]}
	e.header.PageCount = binary.LittleEndian.Uint64(buf[4:12])
	e.header.SchemaPageCount = binary.LittleEndian.Uint64(buf[12:20])
	e.header.Version = binary.LittleEndian.Uint32(buf[20:24])
	return nil
}

func (e *LocalFileEngine) writeHeader() error {
	var buf [headerSize]byte
	copy(buf[0:4], e.header.Magic[:])
	binary.LittleEndian.PutUint64(buf[4:12], e.header.PageCount)
	binary.LittleEndian.PutUint64(buf[12:20], e.header.SchemaPageCount)
	binary.LittleEndian.PutUint32(buf[20:24], e.header.Version)

	_, err := e.file.WriteAt(buf[:], 0)
	return err
}
