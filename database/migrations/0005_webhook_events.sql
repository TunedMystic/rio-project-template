CREATE TABLE processed_webhook_events (
    event_id   TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
