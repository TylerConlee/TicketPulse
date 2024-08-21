package models

import (
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
func CreateSLAAlertCache(cacheEntry SLAAlertCache) error {
	query := `
        INSERT INTO sla_alert_cache (user_id, ticket_id, alert_type, breach_at)
        VALUES ($1, $2, $3, $4)
        RETURNING id
    `
	err := db.Database.QueryRow(query, cacheEntry.UserID, cacheEntry.TicketID, cacheEntry.AlertType, cacheEntry.BreachAt).Scan(&cacheEntry.ID)
	if err != nil {
		return fmt.Errorf("failed to create SLA alert cache entry: %w", err)
	}

	return nil
}

// GetSLAAlertCache retrieves an SLA alert cache entry by user, ticket, and alert type.
func GetSLAAlertCache(userID, ticketID int64, alertType string) (SLAAlertCache, bool) {
	var cacheEntry SLAAlertCache
	query := `
        SELECT id, breach_at FROM sla_alert_cache
        WHERE user_id = $1 AND ticket_id = $2 AND alert_type = $3
    `
	err := db.Database.Get(&cacheEntry, query, userID, ticketID, alertType)
	return cacheEntry, err == nil
}

// ClearSLAAlertCache deletes an SLA alert cache entry by its ID.
func ClearSLAAlertCache(cacheID int64) error {
	query := `DELETE FROM sla_alert_cache WHERE id = $1`
	_, err := db.Database.Exec(query, cacheID)
	return err
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
func CreateAlertLog(logEntry AlertLog) error {
	query := `
        INSERT INTO alert_logs (user_id, ticket_id, tag, alert_type, timestamp)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
    `
	err := db.Database.QueryRow(query, logEntry.UserID, logEntry.TicketID, logEntry.Tag, logEntry.AlertType, logEntry.Timestamp).Scan(&logEntry.ID)
	if err != nil {
		return fmt.Errorf("failed to create alert log: %w", err)
	}
	return nil
}
