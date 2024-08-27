package models

import (
	"context"
	"fmt"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
)

type SLAAlertCache struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	TicketID  int64     `db:"ticket_id"`
	AlertType string    `db:"alert_type"`
	BreachAt  time.Time `db:"breach_at"`
	CreatedAt time.Time `db:"created_at"`
}

// CreateSLAAlertCache inserts a new entry into the sla_alert_cache table.
func CreateSLAAlertCache(ctx context.Context, db db.Database, cacheEntry SLAAlertCache) error {
	query := `
        INSERT INTO sla_alert_cache (user_id, ticket_id, alert_type, breach_at)
        VALUES ($1, $2, $3, $4)
        RETURNING id
    `
	err := db.QueryRowContext(ctx, query, cacheEntry.UserID, cacheEntry.TicketID, cacheEntry.AlertType, cacheEntry.BreachAt).Scan(&cacheEntry.ID)
	if err != nil {
		return fmt.Errorf("failed to create SLA alert cache entry: %w", err)
	}

	return nil
}

// GetSLAAlertCache retrieves an SLA alert cache entry by user, ticket, and alert type.
func GetSLAAlertCache(ctx context.Context, db db.Database, userID, ticketID int, alertType string) (*SLAAlertCache, error) {
	var cacheEntry SLAAlertCache
	query := `SELECT id, user_id, ticket_id, alert_type, breach_at, created_at FROM sla_alert_cache WHERE user_id = $1 AND ticket_id = $2 AND alert_type = $3`
	err := db.QueryRowContext(ctx, query, userID, ticketID, alertType).Scan(&cacheEntry.ID, &cacheEntry.UserID, &cacheEntry.TicketID, &cacheEntry.AlertType, &cacheEntry.BreachAt, &cacheEntry.CreatedAt)
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
