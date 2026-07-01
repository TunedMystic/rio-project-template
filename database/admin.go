package database

import "context"

// ListUsers returns users whose email contains query (case-insensitive; empty
// query matches all), newest first, paginated by limit/offset.
func (s *Store) ListUsers(ctx context.Context, query string, limit, offset int) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE email LIKE '%'||?||'%' "+
			"ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?",
		query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountUsers returns the number of users whose email contains query (empty = all).
func (s *Store) CountUsers(ctx context.Context, query string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE email LIKE '%'||?||'%'", query).Scan(&n)
	return n, err
}

// RevokeEntitlement removes a product entitlement from a user. Removing an
// entitlement the user does not have is a no-op (no error).
func (s *Store) RevokeEntitlement(ctx context.Context, userID int64, productKey string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM entitlements WHERE user_id = ? AND product_key = ?", userID, productKey)
	return err
}
