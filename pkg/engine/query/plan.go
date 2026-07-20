package query

type PlanType int

const (
	PlanScan PlanType = iota
	PlanFilter
	PlanSort
	PlanLimit
	PlanInsert
	PlanUpdate
	PlanDelete
)

type QueryPlan struct {
	Type    PlanType
	Table   string
	Columns []string
	Where   any
	OrderBy string
	Limit   int
	Offset  int
	Rows    []any
	Updates func(map[string]any)
}
