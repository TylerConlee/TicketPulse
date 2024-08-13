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
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"email" TEXT UNIQUE,
		"name" TEXT,
		"role" TEXT,
		"sso_provider" TEXT,
		"notification_tags" TEXT,
		"daily_summary" BOOLEAN
	);`
	_, err = Database.Exec(createUserTableSQL)
	if err != nil {
		log.Fatal(err)
	}
}
