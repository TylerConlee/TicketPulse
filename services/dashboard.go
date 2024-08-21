package services

import (
	"log"

	"github.com/jmoiron/sqlx"
)

type DashboardService struct {
	db *sqlx.DB
}

func NewDashboardService(db *sqlx.DB) *DashboardService {
	return &DashboardService{db: db}
}

type AlertStats struct {
	Date       string `db:"date"`
	AlertType  string `db:"alert_type"`
	Tag        string `db:"tag"`
	AlertCount int    `db:"alert_count"`
}

func (ds *DashboardService) GetAlertStatsForUser(userID int) ([]AlertStats, error) {
	log.Println("Getting alert stats for user", userID)
	query := `
		SELECT 
			DATE(timestamp) AS date, 
			alert_type, 
			tag, 
			COUNT(*) AS alert_count
		FROM 
			alert_logs
		WHERE 
			user_id = $1
		GROUP BY 
			DATE(timestamp), alert_type, tag
		ORDER BY 
			DATE(timestamp) ASC;

    `
	var stats []AlertStats
	err := ds.db.Select(&stats, query, userID)
	log.Println("Got alert stats for user", stats, err)
	return stats, err
}
