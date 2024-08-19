package models

import (
	"fmt"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
)

type AlertLog struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	TicketID  int64     `db:"ticket_id"`
	Tag       string    `db:"tag"`
	AlertType string    `db:"alert_type"`
	Timestamp time.Time `db:"timestamp"`
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
