package database

import "context"

// IsEventProcessed reports whether a Stripe webhook event id has already been
// recorded as processed.
func (s *Store) IsEventProcessed(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM processed_webhook_events WHERE event_id = ?)",
		eventID).Scan(&exists)
	return exists, err
}

// RecordEvent marks a Stripe webhook event id as processed. Idempotent: a
// repeated record is a no-op (the primary key).
func (s *Store) RecordEvent(ctx context.Context, eventID string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO processed_webhook_events (event_id) VALUES (?) ON CONFLICT(event_id) DO NOTHING",
		eventID)
	return err
}
