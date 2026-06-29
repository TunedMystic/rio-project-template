ALTER TABLE users ADD COLUMN google_id TEXT;

-- Partial unique index: at most one user per Google account, while still
-- allowing many users with no Google link (NULL).
CREATE UNIQUE INDEX idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL;
