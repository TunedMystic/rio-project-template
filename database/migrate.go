package database

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
)

// Migrate applies every migrations/*.sql file in files that has not yet been
// applied, in filename order, each inside a transaction. It is forward-only
// and idempotent. Driver- and embed-agnostic (stdlib only) so it can be lifted
// into a shared library unchanged.
func Migrate(db *sql.DB, files fs.FS) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	paths, err := fs.Glob(files, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(paths)

	for _, path := range paths {
		var exists bool
		if err := db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = ?)", path,
		).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		stmt, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(stmt)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", path, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (name) VALUES (?)", path); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
