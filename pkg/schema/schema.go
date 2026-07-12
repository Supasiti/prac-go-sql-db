package schema

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/storage"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"
)

var (
	ErrTableExists   = errors.New("table already registered")
	ErrTableNotFound = errors.New("table not found")
	ErrNotStruct     = errors.New("argument must be a struct")
)

type Schema struct {
	tables map[string]*TableSchema
	engine storage.StorageEngine
}

func NewSchema(engine storage.StorageEngine) (*Schema, error) {
	s := &Schema{
		tables: make(map[string]*TableSchema),
		engine: engine,
	}

	if err := s.load(); err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}

	return s, nil
}

func (s *Schema) Register(v any) error {
	ts, err := parseStruct(v)
	if err != nil {
		return fmt.Errorf("parse struct: %w", err)
	}

	if _, exists := s.tables[ts.Name]; exists {
		return fmt.Errorf("%w: %s", ErrTableExists, ts.Name)
	}

	s.tables[ts.Name] = ts

	if err := s.save(); err != nil {
		return fmt.Errorf("save schema: %w", err)
	}

	return nil
}

func (s *Schema) GetTable(name string) (*TableSchema, error) {
	ts, ok := s.tables[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTableNotFound, name)
	}
	return ts, nil
}

func (s *Schema) TableNames() []string {
	names := make([]string, 0, len(s.tables))
	for name := range s.tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *Schema) save() error {
	tables := make(map[string]*tableData, len(s.tables))
	for name, ts := range s.tables {
		td := &tableData{
			Name:       ts.Name,
			PrimaryKey: ts.PrimaryKey,
			AutoIncKey: ts.AutoIncKey,
			Indexes:    ts.Indexes,
			Columns:    make([]columnData, len(ts.Columns)),
		}
		for i, col := range ts.Columns {
			td.Columns[i] = columnData{
				Name:     col.Name,
				DataType: col.DataType.String(),
				NotNull:  col.NotNull,
				Unique:   col.Unique,
				Indexed:  col.Indexed,
				Ordinal:  col.Ordinal,
			}
		}
		tables[name] = td
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(tables); err != nil {
		return fmt.Errorf("gob encode: %w", err)
	}

	data := buf.Bytes()
	totalLen := len(data)
	maxPayload := storage.SchemaPages * types.PageSize
	if 4+totalLen > maxPayload {
		return fmt.Errorf("schema too large: %d bytes, max %d", totalLen, maxPayload-4)
	}

	// Write length prefix + data
	payload := make([]byte, 4+totalLen)
	binary.LittleEndian.PutUint32(payload[:4], uint32(totalLen))
	copy(payload[4:], data)

	needed := (len(payload) + types.PageSize - 1) / types.PageSize
	for i := 0; i < needed; i++ {
		page := &types.Page{ID: types.PageID(i)}
		start := i * types.PageSize
		end := start + types.PageSize
		if end > len(payload) {
			end = len(payload)
		}
		copy(page.Data[:], payload[start:end])

		if err := s.engine.WritePage(page); err != nil {
			return fmt.Errorf("write page %d: %w", i, err)
		}
	}

	// Clear remaining schema pages
	for i := needed; i < storage.SchemaPages; i++ {
		page := &types.Page{ID: types.PageID(i)}
		if err := s.engine.WritePage(page); err != nil {
			return fmt.Errorf("clear page %d: %w", i, err)
		}
	}

	return nil
}

func (s *Schema) load() error {
	page, err := s.engine.ReadPage(0)
	if err != nil {
		return nil
	}

	allZero := true
	for _, b := range page.Data {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil
	}

	// Read length prefix
	totalLen := binary.LittleEndian.Uint32(page.Data[:4])
	if totalLen == 0 {
		return nil
	}

	// Read exactly the bytes we need
	needed := (int(totalLen) + 4 + types.PageSize - 1) / types.PageSize
	data := make([]byte, 0, needed*types.PageSize)
	data = append(data, page.Data[:]...)

	for i := 1; i < needed; i++ {
		page, err := s.engine.ReadPage(types.PageID(i))
		if err != nil {
			return fmt.Errorf("read page %d: %w", i, err)
		}
		data = append(data, page.Data[:]...)
	}

	// Trim to actual length (skip 4-byte length prefix)
	data = data[4 : 4+int(totalLen)]

	var rawTables map[string]*tableData
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&rawTables); err != nil {
		return fmt.Errorf("gob decode: %w", err)
	}

	s.tables = make(map[string]*TableSchema, len(rawTables))
	for name, td := range rawTables {
		ts := &TableSchema{
			Name:       td.Name,
			PrimaryKey: td.PrimaryKey,
			AutoIncKey: td.AutoIncKey,
			Indexes:    td.Indexes,
			Columns:    make([]Column, len(td.Columns)),
		}
		for i, cd := range td.Columns {
			ts.Columns[i] = Column{
				Name:     cd.Name,
				DataType: resolveType(cd.DataType),
				NotNull:  cd.NotNull,
				Unique:   cd.Unique,
				Indexed:  cd.Indexed,
				Ordinal:  cd.Ordinal,
			}
		}
		s.tables[name] = ts
	}
	return nil
}

func resolveType(name string) reflect.Type {
	switch name {
	case "string":
		return reflect.TypeOf("")
	case "int":
		return reflect.TypeOf(0)
	case "int64":
		return reflect.TypeOf(int64(0))
	case "int32":
		return reflect.TypeOf(int32(0))
	case "int16":
		return reflect.TypeOf(int16(0))
	case "int8":
		return reflect.TypeOf(int8(0))
	case "uint":
		return reflect.TypeOf(uint(0))
	case "uint64":
		return reflect.TypeOf(uint64(0))
	case "uint32":
		return reflect.TypeOf(uint32(0))
	case "uint16":
		return reflect.TypeOf(uint16(0))
	case "uint8":
		return reflect.TypeOf(uint8(0))
	case "float64":
		return reflect.TypeOf(float64(0))
	case "float32":
		return reflect.TypeOf(float32(0))
	case "bool":
		return reflect.TypeOf(false)
	case "[]byte":
		return reflect.TypeOf([]byte{})
	default:
		return reflect.TypeOf("")
	}
}
