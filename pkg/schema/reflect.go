package schema

import (
	"fmt"
	"reflect"
	"strings"
)

func parseStruct(v any) (*TableSchema, error) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", t.Kind())
	}

	schema := &TableSchema{
		Name:    strings.ToLower(t.Name()),
		Columns: make([]Column, 0, t.NumField()),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		dbTag, ok := field.Tag.Lookup("db")
		if !ok {
			continue
		}

		col := Column{
			Name:     dbTag,
			DataType: field.Type,
			Ordinal:  i,
		}

		opts := parseOptions(field.Tag)
		col.NotNull = opts["notnull"]
		col.Unique = opts["unique"]
		col.Indexed = opts["index"]

		if opts["primary"] {
			schema.PrimaryKey = col.Name
		}
		if opts["autoincrement"] {
			schema.AutoIncKey = col.Name
		}

		schema.Columns = append(schema.Columns, col)
	}

	if schema.PrimaryKey != "" {
		schema.Indexes = append(schema.Indexes, IndexDef{
			Name:    "pk_" + schema.Name,
			Columns: []string{schema.PrimaryKey},
			Unique:  true,
		})
	}

	for _, col := range schema.Columns {
		if col.Indexed && col.Name != schema.PrimaryKey {
			schema.Indexes = append(schema.Indexes, IndexDef{
				Name:    "idx_" + schema.Name + "_" + col.Name,
				Columns: []string{col.Name},
				Unique:  col.Unique,
			})
		}
	}

	return schema, nil
}

func parseOptions(tag reflect.StructTag) map[string]bool {
	opts := make(map[string]bool)
	for _, key := range []string{"primary", "notnull", "unique", "index", "autoincrement"} {
		if tag.Get(key) == "true" {
			opts[key] = true
		}
	}
	return opts
}
