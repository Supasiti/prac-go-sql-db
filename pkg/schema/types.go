package schema

import "reflect"

type TableSchema struct {
	Name       string
	Columns    []Column
	PrimaryKey string
	AutoIncKey string
	Indexes    []IndexDef
}

type Column struct {
	Name     string
	DataType reflect.Type
	NotNull  bool
	Unique   bool
	Indexed  bool
	Ordinal  int
}

type IndexDef struct {
	Name    string
	Columns []string
	Unique  bool
}

type columnData struct {
	Name     string
	DataType string
	NotNull  bool
	Unique   bool
	Indexed  bool
	Ordinal  int
}

type tableData struct {
	Name       string
	Columns    []columnData
	PrimaryKey string
	AutoIncKey string
	Indexes    []IndexDef
}
