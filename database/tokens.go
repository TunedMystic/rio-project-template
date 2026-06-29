package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// LoginToken is a pending magic-link token (the hash is the primary key).
type LoginToken struct {
	Email     string
	ExpiresAt time.Time
}

func (s *Store) CreateToken(ctx context.Context, tokenHash, email string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO login_tokens (token_hash, email, expires_at) VALUES (?, ?, ?)",
		tokenHash, email, expiresAt)
	return err
}

// ConsumeToken atomically reads and deletes the token so it can be used only
// once. ok is false when no such token exists. The caller must still check
// ExpiresAt (expired tokens are deleted here too).
func (s *Store) ConsumeToken(ctx context.Context, tokenHash string) (LoginToken, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LoginToken{}, false, err
	}
	defer tx.Rollback()

	var tok LoginToken
	err = tx.QueryRowContext(ctx,
		"SELECT email, expires_at FROM login_tokens WHERE token_hash = ?", tokenHash,
	).Scan(&tok.Email, &tok.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return LoginToken{}, false, nil
	}
	if err != nil {
		return LoginToken{}, false, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM login_tokens WHERE token_hash = ?", tokenHash); err != nil {
		return LoginToken{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return LoginToken{}, false, err
	}
	return tok, true, nil
}

func (s *Store) DeleteExpiredTokens(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM login_tokens WHERE expires_at < ?", time.Now())
	return err
}
