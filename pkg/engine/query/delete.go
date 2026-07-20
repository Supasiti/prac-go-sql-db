package query

import "context"

type DeleteQuery[T any] struct {
	store Store
	table string
	where func(*T) bool
}

func NewDeleteQuery[T any](store Store, table string) *DeleteQuery[T] {
	return &DeleteQuery[T]{store: store, table: table}
}

func (q *DeleteQuery[T]) Where(fn func(*T) bool) *DeleteQuery[T] {
	q.where = fn
	return q
}

func (q *DeleteQuery[T]) Execute(ctx context.Context) (int64, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	var whereFn func(map[string]any) bool
	if q.where != nil {
		whereFn = func(row map[string]any) bool {
			var item T
			MapToStruct(row, &item)
			return q.where(&item)
		}
	}

	n := q.store.DeleteRows(q.table, whereFn)
	return n, nil
}
