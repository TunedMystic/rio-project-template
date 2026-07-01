package database

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; registers "sqlite"
)

// Open opens a SQLite database at path with sane pragmas for a web app.
func Open(path string) (*sql.DB, error) {
	// Ensure the parent directory exists (e.g. dev's ./data on a fresh clone);
	// SQLite creates the file but not missing directories.
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	// All pragmas go in the DSN so the driver applies them to every new
	// connection. journal_mode=WAL persists in the file, but foreign_keys and
	// synchronous are per-connection — setting them here (rather than a one-off
	// Exec) keeps them correct even if the pool ever opens a fresh connection.
	dsn := path + "?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(on)" +
		"&_pragma=synchronous(normal)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // simplest correct default for SQLite writes
	return db, nil
}
