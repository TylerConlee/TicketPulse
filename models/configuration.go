package models

import (
	"database/sql"

	"github.com/TylerConlee/TicketPulse/db"
)

// Database is a global variable to hold the database instance.
var Database db.Database

func init() {
	// Initialize your database connection here.
	var database db.Database
	// Assuming a function db.NewDatabase() initializes the database and returns an instance and an error
	database = db.NewDatabase()

	SetDatabase(database)
}

// SetDatabase sets the global Database variable to the provided database instance.
func SetDatabase(database db.Database) {
	Database = database
}

type Configuration struct {
	Key   string
	Value string
}

// GetConfiguration retrieves a configuration value by key from the database.
// The database interface is passed as a parameter to facilitate testing.
func GetConfiguration(db db.Database, key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return value, nil
}

// SetConfiguration sets a configuration value by key in the database.
// The database interface is passed as a parameter to facilitate testing.
func SetConfiguration(db db.Database, key, value string) error {
	_, err := db.Exec(`
        INSERT INTO configuration (key, value) VALUES (?, ?)
        ON CONFLICT(key) DO UPDATE SET value=excluded.value
    `, key, value)
	return err
}

// GetAllConfigurations retrieves all configuration values from the database.
// The database interface is passed as a parameter to facilitate testing.
func GetAllConfigurations(db db.Database) (map[string]string, error) {
	rows, err := db.Query("SELECT key, value FROM configuration")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	configs := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		configs[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return configs, nil
}
