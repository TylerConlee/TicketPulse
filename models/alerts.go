package models

import (
	"context"
	"fmt"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
)

type SLAAlertCache struct {
	ID         int64     `db:"id"`
	UserID     int64     `db:"user_id"`
	TicketID   int64     `db:"ticket_id"`
	AlertType  string    `db:"alert_type"`
	MetricType string    `db:"metric_type"`
	BreachAt   time.Time `db:"breach_at"`
	CreatedAt  time.Time `db:"created_at"`
	Label      string    `db:"label"`
}

// CreateSLAAlertCache inserts a new entry into the sla_alert_cache table.
func CreateSLAAlertCache(ctx context.Context, db db.Database, cacheEntry SLAAlertCache) error {
	query := `
        INSERT INTO sla_alert_cache (user_id, ticket_id, alert_type, metric_type, breach_at, label)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id
    `
	err := db.QueryRowContext(ctx, query, cacheEntry.UserID, cacheEntry.TicketID, cacheEntry.AlertType, cacheEntry.MetricType, cacheEntry.BreachAt, cacheEntry.Label).Scan(&cacheEntry.ID)
	if err != nil {
		return fmt.Errorf("failed to create SLA alert cache entry: %w", err)
	}

	return nil
}

// GetSLAAlertCache retrieves an SLA alert cache entry by user, ticket, alert type, and metric type.
func GetSLAAlertCache(ctx context.Context, db db.Database, userID, ticketID int, alertType, metricType string) (*SLAAlertCache, error) {
	var cacheEntry SLAAlertCache
	query := `SELECT id, user_id, ticket_id, alert_type, metric_type, breach_at, created_at, label FROM sla_alert_cache WHERE user_id = $1 AND ticket_id = $2 AND alert_type = $3 AND metric_type = $4`
	err := db.QueryRowContext(ctx, query, userID, ticketID, alertType, metricType).Scan(&cacheEntry.ID, &cacheEntry.UserID, &cacheEntry.TicketID, &cacheEntry.AlertType, &cacheEntry.MetricType, &cacheEntry.BreachAt, &cacheEntry.CreatedAt, &cacheEntry.Label)
	if err != nil {
		return nil, err
	}
	return &cacheEntry, nil
}

// ClearSLAAlertCache deletes an SLA alert cache entry by its ID.
func ClearSLAAlertCache(ctx context.Context, db db.Database, cacheID int64) error {
	query := `DELETE FROM sla_alert_cache WHERE id = $1`
	_, err := db.ExecContext(ctx, query, cacheID)
	if err != nil {
		return fmt.Errorf("failed to clear SLA alert cache entry: %w", err)
	}
	return nil
}

// ClearSLAAlertCacheByTicket clears all cache entries for a specific ticket (useful when SLA is resolved)
func ClearSLAAlertCacheByTicket(ctx context.Context, db db.Database, ticketID int64) error {
	query := `DELETE FROM sla_alert_cache WHERE ticket_id = $1`
	_, err := db.ExecContext(ctx, query, ticketID)
	if err != nil {
		return fmt.Errorf("failed to clear SLA alert cache entries for ticket: %w", err)
	}
	return nil
}

// DailySummaryLog represents a log entry for sent daily summaries
type DailySummaryLog struct {
	ID          int64     `db:"id"`
	UserID      int64     `db:"user_id"`
	SummaryDate time.Time `db:"summary_date"`
	CreatedAt   time.Time `db:"created_at"`
}

// CreateDailySummaryLog inserts a new daily summary log entry
func CreateDailySummaryLog(ctx context.Context, db db.Database, userID int, summaryDate time.Time) error {
	query := `
		INSERT INTO daily_summary_log (user_id, summary_date)
		VALUES ($1, $2)
		RETURNING id
	`
	var id int64
	err := db.QueryRowContext(ctx, query, userID, summaryDate.Format("2006-01-02")).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to create daily summary log entry: %w", err)
	}
	return nil
}

// GetDailySummaryLog retrieves a daily summary log entry for a user and date
func GetDailySummaryLog(ctx context.Context, db db.Database, userID int, summaryDate time.Time) (*DailySummaryLog, error) {
	var logEntry DailySummaryLog
	query := `SELECT id, user_id, summary_date, created_at FROM daily_summary_log WHERE user_id = $1 AND summary_date = $2`
	err := db.QueryRowContext(ctx, query, userID, summaryDate.Format("2006-01-02")).Scan(&logEntry.ID, &logEntry.UserID, &logEntry.SummaryDate, &logEntry.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &logEntry, nil
}

// ClearExpiredSLAAlertCache clears SLA alert cache entries that are old (created more than 24 hours ago).
// This keeps recent breached SLA entries to prevent duplicate alerts, while cleaning up stale data.
func ClearExpiredSLAAlertCache(ctx context.Context, db db.Database) error {
	// Only clear entries older than 24 hours to prevent duplicate alerts for breached SLAs
	query := `DELETE FROM sla_alert_cache WHERE created_at < datetime('now', '-24 hours')`
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to clear expired SLA alert cache entries: %w", err)
	}
	return nil
}

type AlertLog struct {
	ID        int64  `db:"id"`
	UserID    int64  `db:"user_id"`
	TicketID  int64  `db:"ticket_id"`
	Tag       string `db:"tag"`
	AlertType string `db:"alert_type"`
	Timestamp string `db:"timestamp"`
}

// CreateAlertLog inserts a new alert log entry into the database.
func CreateAlertLog(ctx context.Context, db db.Database, logEntry AlertLog) error {
	query := `
		INSERT INTO alert_logs (user_id, ticket_id, tag, alert_type, timestamp)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err := db.QueryRowContext(ctx, query, logEntry.UserID, logEntry.TicketID, logEntry.Tag, logEntry.AlertType, logEntry.Timestamp).Scan(&logEntry.ID)
	if err != nil {
		return fmt.Errorf("failed to create alert log: %w", err)
	}
	return nil
}
