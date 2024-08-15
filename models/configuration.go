package models

import (
	"database/sql"

	"github.com/TylerConlee/TicketPulse/db"
)

type Configuration struct {
	Key   string
	Value string
}

func GetConfiguration(key string) (string, error) {
	var value string
	err := db.Database.QueryRow("SELECT value FROM configuration WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return value, nil
}

func SetConfiguration(key, value string) error {
	_, err := db.Database.Exec(`
		INSERT INTO configuration (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, key, value)
	return err
}

func GetAllConfigurations() (map[string]string, error) {
	rows, err := db.Database.Query("SELECT key, value FROM configuration")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	configs := make(map[string]string)
	for rows.Next() {
		var config Configuration
		if err := rows.Scan(&config.Key, &config.Value); err != nil {
			return nil, err
		}
		configs[config.Key] = config.Value
	}
	return configs, nil
}
