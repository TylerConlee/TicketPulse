-- +goose Up
CREATE TABLE IF NOT EXISTS skipped_alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    alert_type TEXT NOT NULL,
    skip_reason TEXT NOT NULL,
    metric_type TEXT,
    label TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS acknowledgment_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_log_id INTEGER,
    ticket_id INTEGER NOT NULL,
    slack_user_id TEXT NOT NULL,
    slack_user_name TEXT NOT NULL,
    acknowledged_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(alert_log_id) REFERENCES alert_logs(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS daily_alert_log_sent (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    log_date DATE NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS daily_alert_log_sent;
DROP TABLE IF EXISTS acknowledgment_logs;
DROP TABLE IF EXISTS skipped_alerts;
