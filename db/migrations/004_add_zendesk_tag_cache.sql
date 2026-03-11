-- +goose Up
CREATE TABLE IF NOT EXISTS zendesk_tag_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag TEXT NOT NULL UNIQUE,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS zendesk_tag_cache;
