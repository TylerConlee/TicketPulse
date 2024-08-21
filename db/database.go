package db

import (
	"log"

	"github.com/jmoiron/sqlx"

	_ "modernc.org/sqlite"
)

var Database *sqlx.DB

func InitDB(filepath string) {
	var err error
	Database, err = sqlx.Open("sqlite", filepath)
	if err != nil {
		log.Fatal(err)
	}

	// Create the users table
	createUserTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE,
		name TEXT,
		role TEXT,
		daily_summary BOOLEAN,
		selected_tags TEXT,
		summary_time DATETIME,
		slack_user_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Create the user_tag_alerts table
	createTagsTableSQL := `
	CREATE TABLE IF NOT EXISTS user_tag_alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		tag TEXT NOT NULL,
		slack_channel_id TEXT NOT NULL,
		alert_type TEXT NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	// Create the configuration table
	createConfigTableSQL := `
	CREATE TABLE IF NOT EXISTS configuration (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		value TEXT NOT NULL
	);`

	// Create the alert_logs table
	createAlertLogsTableSQL := `
	CREATE TABLE IF NOT EXISTS alert_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		user_id INTEGER NOT NULL,   
		ticket_id INTEGER NOT NULL,
		tag TEXT NOT NULL,  
		alert_type TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	createCacheTableSQL := `CREATE TABLE IF NOT EXISTS sla_alert_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT, -- Use INTEGER for AUTOINCREMENT
    	user_id INT NOT NULL,
    	ticket_id INT NOT NULL,
    	alert_type VARCHAR(255) NOT NULL,
    	breach_at TIMESTAMP NOT NULL,
    	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, 
    	UNIQUE(user_id, ticket_id, alert_type),
    	FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	// Execute the SQL statements in sequence
	tables := []string{
		createUserTableSQL,
		createTagsTableSQL,
		createConfigTableSQL,
		createAlertLogsTableSQL,
		createCacheTableSQL,
	}

	for _, table := range tables {
		_, err := Database.Exec(table)
		if err != nil {
			log.Fatalf("Error creating table: %v", err)
		}
	}
}

// GetDB returns the initialized database connection
func GetDB() *sqlx.DB {
	return Database
}
