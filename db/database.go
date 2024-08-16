package db

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var Database *sql.DB

func InitDB(filepath string) {
	var err error
	Database, err = sql.Open("sqlite", filepath)
	if err != nil {
		log.Fatal(err)
	}

	createUserTableSQL := `CREATE TABLE IF NOT EXISTS users (
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
);
`
	createTagsTableSQL := `CREATE TABLE IF NOT EXISTS user_tag_alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    slack_channel_id TEXT NOT NULL,
    alert_type TEXT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);`

	createConfigTableSQL := `CREATE TABLE IF NOT EXISTS configuration (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		zendesk_email TEXT,
		value TEXT NOT NULL
	);`

	_, err = Database.Exec(createUserTableSQL)
	if err != nil {
		log.Fatal(err)
	}

	_, err = Database.Exec(createTagsTableSQL)
	if err != nil {
		log.Fatal(err)
	}

	_, err = Database.Exec(createConfigTableSQL)
	if err != nil {
		log.Fatal(err)
	}
}
