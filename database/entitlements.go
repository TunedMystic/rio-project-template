package database

import "context"

// GrantEntitlement records that a user owns a one-time product. Idempotent: a
// repeated grant is a no-op (the unique index on (user_id, product_key)).
func (s *Store) GrantEntitlement(ctx context.Context, userID int64, productKey string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO entitlements (user_id, product_key) VALUES (?, ?) ON CONFLICT(user_id, product_key) DO NOTHING",
		userID, productKey)
	return err
}

// HasEntitlement reports whether the user owns the product.
func (s *Store) HasEntitlement(ctx context.Context, userID int64, productKey string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM entitlements WHERE user_id = ? AND product_key = ?)",
		userID, productKey).Scan(&exists)
	return exists, err
}

// ListEntitlements returns the product keys the user owns, oldest first.
func (s *Store) ListEntitlements(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT product_key FROM entitlements WHERE user_id = ? ORDER BY created_at", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
