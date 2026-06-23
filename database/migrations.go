package database

import (
	"database/sql"
	"embed"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrateUp applies the embedded migrations to db.
func MigrateUp(db *sql.DB) error {
	return Migrate(db, migrationsFS)
}
