package models

import (
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"log"

	"github.com/TylerConlee/TicketPulse/db"
)

func init() {
	// Register the Role type with gob
	gob.Register(Role(""))
}

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

// serializeNotificationTags converts the NotificationTags slice to a JSON string
func (u *User) serializeNotificationTags() (sql.NullString, error) {
	if len(u.NotificationTags) == 0 {
		return sql.NullString{Valid: false}, nil
	}
	jsonData, err := json.Marshal(u.NotificationTags)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(jsonData), Valid: true}, nil
}

// deserializeNotificationTags converts a JSON string to a NotificationTags slice
func (u *User) deserializeNotificationTags(data sql.NullString) error {
	if !data.Valid || data.String == "" {
		u.NotificationTags = []string{}
		return nil
	}
	return json.Unmarshal([]byte(data.String), &u.NotificationTags)
}

// CreateUser adds a new user to the database
func CreateUser(email, name string, role Role, dailySummary bool) error {
	tags, err := json.Marshal([]string{})
	if err != nil {
		return err
	}

	_, err = db.Database.Exec(`INSERT INTO users (email, name, role, daily_summary, notification_tags) VALUES (?, ?, ?, ?, ?)`,
		email, name, role, dailySummary, tags, false)
	return err
}

// UpdateUser updates a user's information in the database
func UpdateUser(user User) error {
	tags, err := user.serializeNotificationTags()
	if err != nil {
		return err
	}

	_, err = db.Database.Exec(
		`UPDATE users SET name = ?, role = ?, daily_summary = ?, notification_tags = ? WHERE id = ?`,
		user.Name, user.Role, user.DailySummary, tags, user.ID,
	)
	return err
}

// GetUserByEmail retrieves a user by their email
func GetUserByEmail(email string) (User, error) {
	row := db.Database.QueryRow(`SELECT id, email, name, role, sso_provider, notification_tags, daily_summary FROM users WHERE email = ?`, email)
	var user User
	var tags sql.NullString

	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.SSOProvider, &tags, &user.DailySummary)
	if err != nil {
		return user, err
	}

	err = user.deserializeNotificationTags(tags)
	return user, err
}

// GetUserByID retrieves a user by their ID
func GetUserByID(id int) (User, error) {
	row := db.Database.QueryRow(`SELECT id, email, name, role, sso_provider, notification_tags, daily_summary FROM users WHERE id = ?`, id)
	var user User
	var tags sql.NullString

	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.SSOProvider, &tags, &user.DailySummary)
	if err != nil {
		return user, err
	}

	err = user.deserializeNotificationTags(tags)
	return user, err
}

// GetAllUsers retrieves all users from the database
func GetAllUsers() ([]User, error) {
	rows, err := db.Database.Query("SELECT id, email, name, role, daily_summary FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err = rows.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.DailySummary)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func IsFirstUser() bool {
	row := db.Database.QueryRow("SELECT COUNT(*) FROM users")
	var count int
	err := row.Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	return count == 0
}
