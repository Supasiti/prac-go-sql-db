package query

import "context"

type InsertQuery struct {
	store Store
	table string
	rows  []any
}

func NewInsertQuery(store Store, table string, rows ...any) *InsertQuery {
	return &InsertQuery{store: store, table: table, rows: rows}
}

func (q *InsertQuery) Execute(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	maps := make([]map[string]any, len(q.rows))
	for i, row := range q.rows {
		maps[i] = StructToMap(row)
	}

	q.store.InsertRows(q.table, maps)
	return nil
}
