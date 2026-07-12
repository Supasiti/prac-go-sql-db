package schema

import (
	"os"
	"testing"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/storage"
)

type SimpleUser struct {
	ID   int64  `db:"id" primary:"true"`
	Name string `db:"name"`
}

type FullUser struct {
	ID    int64  `db:"id" primary:"true" autoincrement:"true"`
	Name  string `db:"name" notnull:"true" unique:"true"`
	Age   int    `db:"age"`
	Email string `db:"email" index:"true"`
}

type BadModel string

func tempEngine(t *testing.T) *storage.LocalFileEngine {
	t.Helper()
	f, err := os.CreateTemp("", "schema-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())

	engine, err := storage.NewLocalFileEngine(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		engine.Close()
		os.Remove(f.Name())
	})
	return engine
}

func TestRegisterSimpleStruct(t *testing.T) {
	engine := tempEngine(t)
	schema, err := NewSchema(engine)
	if err != nil {
		t.Fatal(err)
	}

	if err := schema.Register(SimpleUser{}); err != nil {
		t.Fatal(err)
	}

	ts, err := schema.GetTable("simpleuser")
	if err != nil {
		t.Fatal(err)
	}

	if ts.Name != "simpleuser" {
		t.Errorf("name = %q, want %q", ts.Name, "simpleuser")
	}
	if len(ts.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(ts.Columns))
	}
	if ts.Columns[0].Name != "id" || ts.Columns[1].Name != "name" {
		t.Errorf("columns = [%s, %s], want [id, name]", ts.Columns[0].Name, ts.Columns[1].Name)
	}
}

func TestRegisterWithAllTags(t *testing.T) {
	engine := tempEngine(t)
	schema, err := NewSchema(engine)
	if err != nil {
		t.Fatal(err)
	}

	if err := schema.Register(FullUser{}); err != nil {
		t.Fatal(err)
	}

	ts, err := schema.GetTable("fulluser")
	if err != nil {
		t.Fatal(err)
	}

	if ts.PrimaryKey != "id" {
		t.Errorf("primary key = %q, want %q", ts.PrimaryKey, "id")
	}
	if ts.AutoIncKey != "id" {
		t.Errorf("auto inc key = %q, want %q", ts.AutoIncKey, "id")
	}

	// Find name column
	var nameCol Column
	for _, col := range ts.Columns {
		if col.Name == "name" {
			nameCol = col
			break
		}
	}
	if !nameCol.NotNull {
		t.Error("name column should be notnull")
	}
	if !nameCol.Unique {
		t.Error("name column should be unique")
	}

	// Find email column
	var emailCol Column
	for _, col := range ts.Columns {
		if col.Name == "email" {
			emailCol = col
			break
		}
	}
	if !emailCol.Indexed {
		t.Error("email column should be indexed")
	}

	// Verify indexes created
	if len(ts.Indexes) < 2 {
		t.Errorf("indexes = %d, want >= 2", len(ts.Indexes))
	}
}

func TestRegisterDuplicateTable(t *testing.T) {
	engine := tempEngine(t)
	schema, err := NewSchema(engine)
	if err != nil {
		t.Fatal(err)
	}

	schema.Register(SimpleUser{})
	err = schema.Register(SimpleUser{})
	if err == nil {
		t.Error("expected error for duplicate table")
	}
}

func TestGetTableExisting(t *testing.T) {
	engine := tempEngine(t)
	schema, err := NewSchema(engine)
	if err != nil {
		t.Fatal(err)
	}

	schema.Register(SimpleUser{})

	ts, err := schema.GetTable("simpleuser")
	if err != nil {
		t.Fatal(err)
	}
	if ts.Name != "simpleuser" {
		t.Errorf("name = %q, want %q", ts.Name, "simpleuser")
	}
}

func TestGetTableNonExisting(t *testing.T) {
	engine := tempEngine(t)
	schema, err := NewSchema(engine)
	if err != nil {
		t.Fatal(err)
	}

	_, err = schema.GetTable("nonexistent")
	if err == nil {
		t.Error("expected error for non-existing table")
	}
}

func TestTableNames(t *testing.T) {
	engine := tempEngine(t)
	schema, err := NewSchema(engine)
	if err != nil {
		t.Fatal(err)
	}

	schema.Register(FullUser{})
	schema.Register(SimpleUser{})

	names := schema.TableNames()
	if len(names) != 2 {
		t.Fatalf("table names = %d, want 2", len(names))
	}
	if names[0] != "fulluser" || names[1] != "simpleuser" {
		t.Errorf("names = %v, want [fulluser, simpleuser]", names)
	}
}

func TestPersistence(t *testing.T) {
	engine := tempEngine(t)

	schema1, err := NewSchema(engine)
	if err != nil {
		t.Fatal(err)
	}
	schema1.Register(SimpleUser{})
	schema1.Register(FullUser{})
	engine.Close()

	// Reopen
	engine2, err := storage.NewLocalFileEngine(engine.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer engine2.Close()

	schema2, err := NewSchema(engine2)
	if err != nil {
		t.Fatal(err)
	}

	names := schema2.TableNames()
	if len(names) != 2 {
		t.Fatalf("persisted tables = %d, want 2", len(names))
	}
}

func TestColumnOrdering(t *testing.T) {
	engine := tempEngine(t)
	schema, err := NewSchema(engine)
	if err != nil {
		t.Fatal(err)
	}

	schema.Register(FullUser{})

	ts, _ := schema.GetTable("fulluser")
	if ts.Columns[0].Ordinal != 0 {
		t.Errorf("first column ordinal = %d, want 0", ts.Columns[0].Ordinal)
	}
	if ts.Columns[1].Ordinal != 1 {
		t.Errorf("second column ordinal = %d, want 1", ts.Columns[1].Ordinal)
	}
}

func TestAutoIncrementDetected(t *testing.T) {
	engine := tempEngine(t)
	schema, err := NewSchema(engine)
	if err != nil {
		t.Fatal(err)
	}

	schema.Register(FullUser{})

	ts, _ := schema.GetTable("fulluser")
	if ts.AutoIncKey != "id" {
		t.Errorf("auto inc key = %q, want %q", ts.AutoIncKey, "id")
	}
}
