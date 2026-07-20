package engine

import (
	"context"
	"os"
	"testing"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/storage"
)

type User struct {
	ID   int    `db:"id" primary:"true"`
	Name string `db:"name" notnull:"true"`
	Age  int    `db:"age"`
}

func tempDB(t *testing.T) *DB {
	t.Helper()
	f, err := os.CreateTemp("", "query-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())

	eng, err := storage.NewLocalFileEngine(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(eng)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Register(User{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.pool.Flush()
		eng.Close()
		os.Remove(f.Name())
	})
	return db
}

func TestInsertAndFindAll(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	err := db.Insert(&User{ID: 1, Name: "Alice", Age: 30}).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Insert(&User{ID: 2, Name: "Bob", Age: 25}).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}

	users, err := Find[User](db).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2", len(users))
	}
}

func TestFindSelect(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.Insert(&User{ID: 1, Name: "Alice", Age: 30}).Execute(ctx)

	users, err := Find[User](db).Select("name").Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}
	if users[0].Name != "Alice" {
		t.Errorf("name = %q, want Alice", users[0].Name)
	}
	if users[0].Age != 0 {
		t.Errorf("age = %d, want 0 (not selected)", users[0].Age)
	}
}

func TestFindWhere(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.Insert(&User{ID: 1, Name: "Alice", Age: 30}).Execute(ctx)
	db.Insert(&User{ID: 2, Name: "Bob", Age: 17}).Execute(ctx)

	users, err := Find[User](db).Where(func(u *User) bool { return u.Age >= 18 }).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}
	if users[0].Name != "Alice" {
		t.Errorf("name = %q, want Alice", users[0].Name)
	}
}

func TestFindOrderBy(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.Insert(&User{ID: 1, Name: "Bob", Age: 25}).Execute(ctx)
	db.Insert(&User{ID: 2, Name: "Alice", Age: 30}).Execute(ctx)

	users, err := Find[User](db).OrderBy("name").Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2", len(users))
	}
	if users[0].Name != "Alice" {
		t.Errorf("first = %q, want Alice", users[0].Name)
	}
	if users[1].Name != "Bob" {
		t.Errorf("second = %q, want Bob", users[1].Name)
	}
}

func TestFindLimitOffset(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.Insert(&User{ID: 1, Name: "Alice", Age: 30}).Execute(ctx)
	db.Insert(&User{ID: 2, Name: "Bob", Age: 25}).Execute(ctx)
	db.Insert(&User{ID: 3, Name: "Carol", Age: 20}).Execute(ctx)

	users, err := Find[User](db).OrderBy("name").Limit(1).Offset(1).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}
	if users[0].Name != "Bob" {
		t.Errorf("name = %q, want Bob", users[0].Name)
	}
}

func TestInsertMultiple(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	err := db.Insert(
		&User{ID: 1, Name: "Alice", Age: 30},
		&User{ID: 2, Name: "Bob", Age: 25},
	).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}

	users, err := Find[User](db).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2", len(users))
	}
}

func TestUpdateWithWhere(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.Insert(&User{ID: 1, Name: "Alice", Age: 30}).Execute(ctx)
	db.Insert(&User{ID: 2, Name: "Bob", Age: 25}).Execute(ctx)

	n, err := db.Update("User", func(row map[string]any) {
		row["age"] = row["age"].(int) + 1
	}).Where(func(row map[string]any) bool {
		return row["name"] == "Alice"
	}).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows affected = %d, want 1", n)
	}

	users, err := Find[User](db).OrderBy("name").Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if users[0].Age != 31 {
		t.Errorf("Alice age = %d, want 31", users[0].Age)
	}
	if users[1].Age != 25 {
		t.Errorf("Bob age = %d, want 25", users[1].Age)
	}
}

func TestDeleteWithWhere(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.Insert(&User{ID: 1, Name: "Alice", Age: 30}).Execute(ctx)
	db.Insert(&User{ID: 2, Name: "Bob", Age: 17}).Execute(ctx)

	n, err := Delete[User](db).Where(func(u *User) bool { return u.Age < 18 }).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows deleted = %d, want 1", n)
	}

	users, err := Find[User](db).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}
	if users[0].Name != "Alice" {
		t.Errorf("remaining = %q, want Alice", users[0].Name)
	}
}

func TestDeleteAll(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.Insert(&User{ID: 1, Name: "Alice", Age: 30}).Execute(ctx)
	db.Insert(&User{ID: 2, Name: "Bob", Age: 25}).Execute(ctx)

	n, err := Delete[User](db).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows deleted = %d, want 2", n)
	}

	users, err := Find[User](db).Execute(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("got %d users, want 0", len(users))
	}
}
