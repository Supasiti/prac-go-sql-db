package wal

import (
	"encoding/binary"
	"errors"
	"fmt"

	"hash/crc32"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"
)

var (
	ErrChecksumMismatch = errors.New("wal: checksum mismatch")
	ErrEntryTooShort    = errors.New("wal: entry data too short")
)

type EntryType uint8

const (
	EntryBegin      EntryType = iota // Start of transaction
	EntryCommit                      // Commit transaction
	EntryAbort                       // Abort transaction
	EntryPageWrite                   // Page modification
	EntryCheckpoint                  // Checkpoint marker
)

const entryHeaderSize = 33 // 8+8+1+8+2+2+4

type Entry struct {
	LSN      uint64
	TxnID    uint64
	Type     EntryType
	PageID   types.PageID
	Offset   uint16
	Length   uint16
	Data     []byte
	Checksum uint32
}

func (e *Entry) ComputeChecksum() uint32 {
	size := entryHeaderSize - 4 + len(e.Data)
	buf := make([]byte, size)

	binary.LittleEndian.PutUint64(buf[0:8], e.LSN)
	binary.LittleEndian.PutUint64(buf[8:16], e.TxnID)
	buf[16] = byte(e.Type)
	binary.LittleEndian.PutUint64(buf[17:25], uint64(e.PageID))
	binary.LittleEndian.PutUint16(buf[25:27], e.Offset)
	binary.LittleEndian.PutUint16(buf[27:29], e.Length)
	copy(buf[29:], e.Data)

	return crc32.ChecksumIEEE(buf)
}

func (e *Entry) VerifyChecksum() bool {
	return e.Checksum == e.ComputeChecksum()
}

func (e *Entry) Marshal() ([]byte, error) {
	e.Checksum = e.ComputeChecksum()

	size := entryHeaderSize + len(e.Data)
	buf := make([]byte, size)

	binary.LittleEndian.PutUint64(buf[0:8], e.LSN)
	binary.LittleEndian.PutUint64(buf[8:16], e.TxnID)
	buf[16] = byte(e.Type)
	binary.LittleEndian.PutUint64(buf[17:25], uint64(e.PageID))
	binary.LittleEndian.PutUint16(buf[25:27], e.Offset)
	binary.LittleEndian.PutUint16(buf[27:29], e.Length)
	copy(buf[29:], e.Data)
	binary.LittleEndian.PutUint32(buf[size-4:], e.Checksum)

	return buf, nil
}

func Unmarshal(data []byte) (*Entry, error) {
	if len(data) < entryHeaderSize {
		return nil, fmt.Errorf("%w: got %d bytes, need at least %d", ErrEntryTooShort, len(data), entryHeaderSize)
	}

	e := &Entry{
		LSN:      binary.LittleEndian.Uint64(data[0:8]),
		TxnID:    binary.LittleEndian.Uint64(data[8:16]),
		Type:     EntryType(data[16]),
		PageID:   types.PageID(binary.LittleEndian.Uint64(data[17:25])),
		Offset:   binary.LittleEndian.Uint16(data[25:27]),
		Length:   binary.LittleEndian.Uint16(data[27:29]),
		Checksum: binary.LittleEndian.Uint32(data[len(data)-4:]),
	}

	if e.Length > 0 {
		e.Data = make([]byte, e.Length)
		copy(e.Data, data[29:29+e.Length])
	}

	return e, nil
}
