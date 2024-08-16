package models

import (
	"database/sql"
	"encoding/gob"
	"log"
	"time"

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
	ID           int
	Email        string
	Name         string
	Role         Role
	DailySummary bool
	SelectedTags []TagAlert     // New field for storing tag-specific alerts
	SummaryTime  sql.NullTime   // The preferred time for the daily summary
	SlackUserID  sql.NullString // The user's Slack ID for direct messages
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type TagAlert struct {
	ID             int
	UserID         int
	Tag            string
	SlackChannelID string
	AlertType      string
	User           User // Add User field to associate with the alert
}

// CreateUser adds a new user to the database
func CreateUser(email, name string, role Role, dailySummary bool) error {

	_, err := db.Database.Exec(`INSERT INTO users (email, name, role, daily_summary ) VALUES (?, ?, ?, ?)`,
		email, name, role, dailySummary)
	return err
}

// UpdateUser updates a user's information in the database
func UpdateUser(user User) error {

	_, err := db.Database.Exec(
		`UPDATE users SET name = ?, role = ?, daily_summary = ?, WHERE id = ?`,
		user.Name, user.Role, user.DailySummary, user.ID,
	)
	return err
}

// GetUserByEmail retrieves a user by their email
func GetUserByEmail(email string) (User, error) {
	var user User
	row := db.Database.QueryRow("SELECT id, email, name, role, daily_summary FROM users WHERE LOWER(email) = LOWER(?)", email)
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.DailySummary)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return a special error to indicate that the user was not found
			return user, nil
		}
		return user, err
	}
	return user, nil
}

// GetUserByID retrieves a user by their ID
func GetUserByID(id int) (User, error) {
	row := db.Database.QueryRow(`SELECT id, email, name, role, daily_summary, summary_time, slack_user_id FROM users WHERE id = ?`, id)
	var user User

	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.DailySummary, &user.SummaryTime, &user.SlackUserID)
	if err != nil {
		return user, err
	}

	// Convert sql.NullString to regular string or handle NULL case
	slackUserID := ""
	if user.SlackUserID.Valid {
		slackUserID = user.SlackUserID.String
	}
	user.SlackUserID = sql.NullString{String: slackUserID, Valid: user.SlackUserID.Valid}

	summaryTime := time.Time{}
	if user.SummaryTime.Valid {
		summaryTime = user.SummaryTime.Time
	}
	user.SummaryTime = sql.NullTime{Time: summaryTime, Valid: user.SummaryTime.Valid}

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

func GetFirstUserID() (int, error) {
	var id int
	err := db.Database.QueryRow("SELECT id FROM users ORDER BY id ASC LIMIT 1").Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil // No users in the database
		}
		return 0, err
	}
	return id, nil
}

func DeleteUserByID(userID int) error {
	_, err := db.Database.Exec("DELETE FROM users WHERE id = ?", userID)
	return err
}

func GetUserCount() (int, error) {
	var count int
	err := db.Database.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CreateTagAlert adds a new tag alert configuration for a user
func CreateTagAlert(userID int, tag, slackChannelID, alertType string) error {
	_, err := db.Database.Exec(`INSERT INTO user_tag_alerts (user_id, tag, slack_channel_id, alert_type) VALUES (?, ?, ?, ?)`,
		userID, tag, slackChannelID, alertType)
	return err
}

// GetTagAlertsByUser retrieves all tag alerts for a specific user
func GetTagAlertsByUser(userID int) ([]TagAlert, error) {
	rows, err := db.Database.Query(`SELECT id, user_id, tag, slack_channel_id, alert_type FROM user_tag_alerts WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []TagAlert
	for rows.Next() {
		var alert TagAlert
		err = rows.Scan(&alert.ID, &alert.UserID, &alert.Tag, &alert.SlackChannelID, &alert.AlertType)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

// DeleteTagAlert removes a specific tag alert configuration
func DeleteTagAlert(alertID int) error {
	_, err := db.Database.Exec(`DELETE FROM user_tag_alerts WHERE id = ?`, alertID)
	return err
}
func GetAllTagAlerts() ([]TagAlert, error) {
	rows, err := db.Database.Query(`
		SELECT 
			uta.id, uta.tag, uta.slack_channel_id, uta.alert_type, 
			u.id, u.name, u.email 
		FROM 
			user_tag_alerts uta 
		INNER JOIN 
			users u ON uta.user_id = u.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []TagAlert
	for rows.Next() {
		var alert TagAlert
		var user User
		err = rows.Scan(&alert.ID, &alert.Tag, &alert.SlackChannelID, &alert.AlertType, &user.ID, &user.Name, &user.Email)
		if err != nil {
			return nil, err
		}
		alert.User = user // Now this assignment works
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

// UpdateDailySummarySettings updates the user's daily summary settings.
func (u *User) UpdateDailySummarySettings(dailySummary bool, summaryTime time.Time) error {
	_, err := db.Database.Exec(`UPDATE users SET daily_summary = ?, summary_time = ? WHERE id = ?`, dailySummary, summaryTime, u.ID)
	return err
}

// GetUsersWithDailySummaryEnabled returns a list of users who have enabled the daily summary.
func GetUsersWithDailySummaryEnabled(db *sql.DB) ([]User, error) {
	rows, err := db.Query(`SELECT id, name, email, role, daily_summary, summary_time, slack_user_id FROM users WHERE daily_summary = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.DailySummary, &user.SummaryTime, &user.SlackUserID)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
