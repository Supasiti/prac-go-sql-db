package query

import "context"

type UpdateQuery struct {
	store   Store
	table   string
	changes func(map[string]any)
	where   func(map[string]any) bool
}

func NewUpdateQuery(store Store, table string, fn func(map[string]any)) *UpdateQuery {
	return &UpdateQuery{store: store, table: table, changes: fn}
}

func (q *UpdateQuery) Where(fn func(map[string]any) bool) *UpdateQuery {
	q.where = fn
	return q
}

func (q *UpdateQuery) Execute(ctx context.Context) (int64, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	n := q.store.UpdateRows(q.table, q.changes, q.where)
	return n, nil
}
