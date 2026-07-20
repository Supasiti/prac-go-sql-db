package query

import "reflect"

type Store interface {
	GetRows(table string) []map[string]any
	InsertRows(table string, rows []map[string]any)
	UpdateRows(table string, fn func(map[string]any), where func(map[string]any) bool) int64
	DeleteRows(table string, where func(map[string]any) bool) int64
}

func TableName[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func StructToMap(v any) map[string]any {
	t := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		val = val.Elem()
	}
	m := make(map[string]any)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag, ok := field.Tag.Lookup("db")
		if !ok {
			continue
		}
		m[tag] = val.Field(i).Interface()
	}
	return m
}

func MapToStruct(m map[string]any, v any) {
	t := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		val = val.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag, ok := field.Tag.Lookup("db")
		if !ok {
			continue
		}
		if fv, ok := m[tag]; ok {
			fvVal := reflect.ValueOf(fv)
			val.Field(i).Set(fvVal)
		}
	}
}
