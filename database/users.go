package database

import (
	"context"
	"database/sql"
	"time"
)

// User is an account holder. Email is the case-insensitive identity.
type User struct {
	ID        int64
	Email     string
	Name      string
	CreatedAt time.Time
	GoogleID  string // empty when no Google account is linked
}

// CreateUser inserts a user and returns it with id and created_at populated.
func (s *Store) CreateUser(ctx context.Context, email, name string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO users (email, name) VALUES (?, ?) RETURNING id, email, name, created_at",
		email, name,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	return u, err
}

// UserByEmail looks up a user by email (case-insensitive via column collation).
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, google_id FROM users WHERE email = ?", email))
}

// UserByID looks up a user by id.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, google_id FROM users WHERE id = ?", id))
}

// UpdateUserName sets the display name.
func (s *Store) UpdateUserName(ctx context.Context, id int64, name string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET name = ? WHERE id = ?", name, id)
	return err
}

// DeleteUser removes the user; sessions cascade via the foreign key.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	return err
}

// UserByGoogleID looks up a user by their linked Google account id (sub).
func (s *Store) UserByGoogleID(ctx context.Context, googleID string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, google_id FROM users WHERE google_id = ?", googleID))
}

// SetUserGoogleID links a Google account to the user.
func (s *Store) SetUserGoogleID(ctx context.Context, id int64, googleID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET google_id = ? WHERE id = ?", googleID, id)
	return err
}

// ClearUserGoogleID unlinks the user's Google account.
func (s *Store) ClearUserGoogleID(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET google_id = NULL WHERE id = ?", id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanUser(row rowScanner) (User, error) {
	var u User
	var gid sql.NullString
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt, &gid)
	u.GoogleID = gid.String
	return u, err
}
