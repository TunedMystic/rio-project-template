package database

import (
	"context"
	"database/sql"
	"time"
)

// User is an account holder. Email is the case-insensitive identity.
type User struct {
	ID                 int64
	Email              string
	Name               string
	CreatedAt          time.Time
	StripeCustomerID   string    // empty when no Stripe customer yet
	SubscriptionStatus string    // '', active, trialing, past_due, canceled
	CurrentPeriodEnd   time.Time // zero when no subscription
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
		"SELECT id, email, name, created_at, stripe_customer_id, subscription_status, current_period_end FROM users WHERE email = ?", email))
}

// UserByID looks up a user by id.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, stripe_customer_id, subscription_status, current_period_end FROM users WHERE id = ?", id))
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

type rowScanner interface {
	Scan(dest ...any) error
}

// SetStripeCustomerID links a Stripe customer to the user.
func (s *Store) SetStripeCustomerID(ctx context.Context, id int64, customerID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET stripe_customer_id = ? WHERE id = ?", customerID, id)
	return err
}

// UserByStripeCustomerID looks up a user by their Stripe customer id.
func (s *Store) UserByStripeCustomerID(ctx context.Context, customerID string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, stripe_customer_id, subscription_status, current_period_end FROM users WHERE stripe_customer_id = ?", customerID))
}

// UpdateSubscription sets the subscription status + period end for the user with
// the given Stripe customer id.
func (s *Store) UpdateSubscription(ctx context.Context, customerID, status string, periodEnd time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET subscription_status = ?, current_period_end = ? WHERE stripe_customer_id = ?",
		status, periodEnd, customerID)
	return err
}

func (s *Store) scanUser(row rowScanner) (User, error) {
	var u User
	var cust sql.NullString
	var pend sql.NullTime
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt, &cust, &u.SubscriptionStatus, &pend)
	u.StripeCustomerID = cust.String
	u.CurrentPeriodEnd = pend.Time
	return u, err
}
