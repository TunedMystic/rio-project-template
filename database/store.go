package database

import (
	"context"
	"database/sql"
	"time"
)

// Message is a row in the messages table (the demo resource).
type Message struct {
	ID        int64
	Body      string
	CreatedAt time.Time
}

// Store provides data access methods over a *sql.DB using raw SQL.
type Store struct {
	db *sql.DB
}

// NewStore constructs a Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateMessage inserts a new message.
func (s *Store) CreateMessage(ctx context.Context, body string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO messages (body) VALUES (?)", body)
	return err
}

// ListMessages returns all messages, newest first.
func (s *Store) ListMessages(ctx context.Context) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, body, created_at FROM messages ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
