package query

import (
	"context"
	"fmt"
	"sort"
)

type FindQuery[T any] struct {
	store   Store
	table   string
	cols    []string
	where   func(*T) bool
	orderBy string
	limit   int
	offset  int
}

func NewFindQuery[T any](store Store, table string) *FindQuery[T] {
	return &FindQuery[T]{store: store, table: table}
}

func (q *FindQuery[T]) Select(cols ...string) *FindQuery[T] {
	q.cols = cols
	return q
}

func (q *FindQuery[T]) Where(fn func(*T) bool) *FindQuery[T] {
	q.where = fn
	return q
}

func (q *FindQuery[T]) OrderBy(col string) *FindQuery[T] {
	q.orderBy = col
	return q
}

func (q *FindQuery[T]) Limit(n int) *FindQuery[T] {
	q.limit = n
	return q
}

func (q *FindQuery[T]) Offset(n int) *FindQuery[T] {
	q.offset = n
	return q
}

func (q *FindQuery[T]) Execute(ctx context.Context) ([]T, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	rows := q.store.GetRows(q.table)

	results := make([]T, 0, len(rows))
	for _, row := range rows {
		var item T
		MapToStruct(row, &item)

		if q.where != nil && !q.where(&item) {
			continue
		}
		results = append(results, item)
	}

	if q.orderBy != "" {
		colName := q.orderBy
		sort.Slice(results, func(i, j int) bool {
			ri := StructToMap(results[i])
			rj := StructToMap(results[j])
			vi, _ := ri[colName]
			vj, _ := rj[colName]
			return fmt.Sprintf("%v", vi) < fmt.Sprintf("%v", vj)
		})
	}

	if q.offset > 0 {
		if q.offset >= len(results) {
			return []T{}, nil
		}
		results = results[q.offset:]
	}

	if q.limit > 0 && q.limit < len(results) {
		results = results[:q.limit]
	}

	if len(q.cols) > 0 {
		projected := make([]T, len(results))
		for i, item := range results {
			srcMap := StructToMap(item)
			projectedMap := make(map[string]any)
			for _, c := range q.cols {
				if v, ok := srcMap[c]; ok {
					projectedMap[c] = v
				}
			}
			MapToStruct(projectedMap, &projected[i])
		}
		return projected, nil
	}

	return results, nil
}
