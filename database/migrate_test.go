package database

import (
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestMigrate_AppliesAndIsIdempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	files := fstest.MapFS{
		"migrations/0001_init.sql": {Data: []byte(
			"CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL);")},
	}

	if err := Migrate(db, files); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Table exists.
	if _, err := db.Exec("INSERT INTO widgets (name) VALUES ('a')"); err != nil {
		t.Fatalf("insert into migrated table: %v", err)
	}

	// Tracking row recorded.
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if n != 1 {
		t.Fatalf("schema_migrations count = %d, want 1", n)
	}

	// Idempotent: re-running applies nothing and does not error.
	if err := Migrate(db, files); err != nil {
		t.Fatalf("Migrate (2nd run): %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if n != 1 {
		t.Errorf("after 2nd run count = %d, want 1", n)
	}
}
