package engine

import (
	"fmt"
	"sync"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/buffer"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/query"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/storage"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/schema"
)

type DB struct {
	schema *schema.Schema
	pool   *buffer.Pool
	rows   *rowStore
}

type rowStore struct {
	mu     sync.RWMutex
	tables map[string][]map[string]any
}

func newRowStore() *rowStore {
	return &rowStore{tables: make(map[string][]map[string]any)}
}

func (rs *rowStore) GetRows(table string) []map[string]any {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	rows := rs.tables[table]
	out := make([]map[string]any, len(rows))
	copy(out, rows)
	return out
}

func (rs *rowStore) InsertRows(table string, rows []map[string]any) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.tables[table] = append(rs.tables[table], rows...)
}

func (rs *rowStore) UpdateRows(table string, fn func(map[string]any), where func(map[string]any) bool) int64 {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	var count int64
	for _, row := range rs.tables[table] {
		if where != nil && !where(row) {
			continue
		}
		fn(row)
		count++
	}
	return count
}

func (rs *rowStore) DeleteRows(table string, where func(map[string]any) bool) int64 {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rows := rs.tables[table]
	if where == nil {
		n := len(rows)
		rs.tables[table] = nil
		return int64(n)
	}

	filtered := rows[:0]
	var deleted int64
	for _, row := range rows {
		if where(row) {
			deleted++
		} else {
			filtered = append(filtered, row)
		}
	}
	rs.tables[table] = filtered
	return deleted
}

var _ query.Store = (*rowStore)(nil)

func Open(engine storage.StorageEngine) (*DB, error) {
	s, err := schema.NewSchema(engine)
	if err != nil {
		return nil, fmt.Errorf("open schema: %w", err)
	}
	pool := buffer.NewPool(engine, 1024)
	return &DB{
		schema: s,
		pool:   pool,
		rows:   newRowStore(),
	}, nil
}

func (db *DB) Register(v any) error {
	return db.schema.Register(v)
}

func Find[T any](db *DB) *query.FindQuery[T] {
	name := query.TableName[T]()
	return query.NewFindQuery[T](db.rows, name)
}

// Read table name from type
func tableName(v any) string {
	t := fmt.Sprintf("%T", v)
	if t == "" {
		return ""
	}
	if t[0] == '*' {
		t = t[1:]
	}
	for i := len(t) - 1; i >= 0; i-- {
		if t[i] == '.' {
			return t[i+1:]
		}
	}
	return t
}

func (db *DB) Insert(rows ...any) *query.InsertQuery {
	if len(rows) == 0 {
		return query.NewInsertQuery(db.rows, "")
	}
	name := tableName(rows[0])
	return query.NewInsertQuery(db.rows, name, rows...)
}

func (db *DB) Update(table string, fn func(map[string]any)) *query.UpdateQuery {
	return query.NewUpdateQuery(db.rows, table, fn)
}

func Delete[T any](db *DB) *query.DeleteQuery[T] {
	name := query.TableName[T]()
	return query.NewDeleteQuery[T](db.rows, name)
}
