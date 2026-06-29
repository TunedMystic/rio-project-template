ALTER TABLE users ADD COLUMN stripe_customer_id  TEXT;
ALTER TABLE users ADD COLUMN subscription_status TEXT NOT NULL DEFAULT '';  -- '', active, trialing, past_due, canceled
ALTER TABLE users ADD COLUMN current_period_end  TIMESTAMP;

CREATE UNIQUE INDEX idx_users_stripe_customer_id ON users(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;

CREATE TABLE entitlements (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_key TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, product_key)
);
CREATE INDEX idx_entitlements_user_id ON entitlements(user_id);
