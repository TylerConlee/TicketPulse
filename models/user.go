package models

import (
	"log"

	"github.com/tylerconlee/ticketpulse/db"
)

type Role string

const (
	AdminRole Role = "admin"
	AgentRole Role = "agent"
)

type User struct {
	ID               int
	Email            string
	Name             string
	Role             Role
	NotificationTags []string
	DailySummary     bool
	SSOProvider      string // e.g., "google"
}

// CreateUser adds a new user to the database
func CreateUser(email, name string, role Role, provider string) {
	insertSQL := `INSERT INTO users (email, name, role, sso_provider) VALUES (?, ?, ?, ?)`
	_, err := db.Database.Exec(insertSQL, email, name, role, provider)
	if err != nil {
		log.Fatal(err)
	}
}

// GetUserByEmail retrieves a user by their email
func GetUserByEmail(email string) (User, error) {
	row := db.Database.QueryRow("SELECT id, email, name, role, sso_provider FROM users WHERE email = ?", email)
	var user User
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.SSOProvider)
	return user, err
}

// Additional CRUD operations for User here...
