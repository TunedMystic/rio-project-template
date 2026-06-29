package database

import (
	"context"
	"time"
)

// Session is a server-side login session. ID is sha256(cookie token).
type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
	UserAgent string
	IP        string
}

func (s *Store) CreateSession(ctx context.Context, id string, userID int64, expiresAt time.Time, userAgent, ip string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (id, user_id, expires_at, user_agent, ip) VALUES (?, ?, ?, ?, ?)",
		id, userID, expiresAt, userAgent, ip)
	return err
}

func (s *Store) SessionByID(ctx context.Context, id string) (Session, error) {
	var sess Session
	err := s.db.QueryRowContext(ctx,
		"SELECT id, user_id, expires_at, created_at, user_agent, ip FROM sessions WHERE id = ?", id,
	).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt, &sess.UserAgent, &sess.IP)
	return sess, err
}

func (s *Store) ListUserSessions(ctx context.Context, userID int64) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, user_id, expires_at, created_at, user_agent, ip FROM sessions WHERE user_id = ? ORDER BY created_at DESC",
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt, &sess.UserAgent, &sess.IP); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	return err
}

// DeleteUserSessions deletes all of a user's sessions except exceptID (pass ""
// to delete all).
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64, exceptID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE user_id = ? AND id != ?", userID, exceptID)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", time.Now())
	return err
}
