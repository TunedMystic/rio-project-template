package database

import (
	"database/sql"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; registers "sqlite"
)

// Open opens a SQLite database at path with sane pragmas for a web app.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	db.SetMaxOpenConns(1) // simplest correct default for SQLite writes
	return db, nil
}
