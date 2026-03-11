package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *db.SQLDatabase {
	database := db.InitDB(":memory:")
	require.NotNil(t, database, "Failed to initialize test database")
	return database
}

func TestCreateUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	assert.NoError(t, err)

	// Verify user was created
	user, err := GetUserByEmail(database, "test@example.com")
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, AdminRole, user.Role)
	assert.False(t, user.DailySummary)
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	assert.NoError(t, err)

	// Attempt to create duplicate
	err = CreateUser(database, "test@example.com", "Another User", AgentRole, true)
	assert.Error(t, err)
}

func TestGetUserByEmail(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create user first
	err := CreateUser(database, "test@example.com", "Test User", AgentRole, true)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, AgentRole, user.Role)
	assert.True(t, user.DailySummary)
}

func TestGetUserByEmail_CaseInsensitive(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "Test@Example.COM", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	assert.NoError(t, err)
	assert.Equal(t, "Test@Example.COM", user.Email)
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	_, err := GetUserByEmail(database, "nonexistent@example.com")
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestGetUserByID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, true)
	require.NoError(t, err)

	// Get the created user to find their ID
	createdUser, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	user, err := GetUserByID(database, createdUser.ID)
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
}

func TestGetUserByID_NotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	_, err := GetUserByID(database, 99999)
	assert.Error(t, err)
}

func TestGetAllUsers(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create multiple users
	err := CreateUser(database, "user1@example.com", "User 1", AdminRole, false)
	require.NoError(t, err)
	err = CreateUser(database, "user2@example.com", "User 2", AgentRole, true)
	require.NoError(t, err)

	users, err := GetAllUsers(database)
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestGetAllUsers_Empty(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	users, err := GetAllUsers(database)
	assert.NoError(t, err)
	assert.Empty(t, users)
}

func TestDeleteUserByID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = DeleteUserByID(database, user.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = GetUserByEmail(database, "test@example.com")
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestIsFirstUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Should be true when no users exist
	assert.True(t, IsFirstUser(database))

	// Create a user
	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	// Should be false after creating a user
	assert.False(t, IsFirstUser(database))
}

func TestGetUserCount(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	count, err := GetUserCount(database)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	err = CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	count, err = GetUserCount(database)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestUpdateSlackUserID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	err = UpdateSlackUserID(database, "test@example.com", "U12345678")
	assert.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	assert.NoError(t, err)
	assert.True(t, user.SlackUserID.Valid)
	assert.Equal(t, "U12345678", user.SlackUserID.String)
}

func TestUpdateDailySummarySettings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	summaryTime := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	err = user.UpdateDailySummarySettings(database, true, summaryTime)
	assert.NoError(t, err)

	// Retrieve and verify
	updatedUser, err := GetUserByID(database, user.ID)
	assert.NoError(t, err)
	assert.True(t, updatedUser.DailySummary)
	assert.True(t, updatedUser.SummaryTime.Valid)
}

func TestGetUsersWithDailySummaryEnabled(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create users with different daily summary settings
	err := CreateUser(database, "user1@example.com", "User 1", AdminRole, true)
	require.NoError(t, err)
	err = CreateUser(database, "user2@example.com", "User 2", AgentRole, false)
	require.NoError(t, err)
	err = CreateUser(database, "user3@example.com", "User 3", AgentRole, true)
	require.NoError(t, err)

	users, err := GetUsersWithDailySummaryEnabled(database)
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUpdateWorkDaySettings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	startTime := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)
	err = user.UpdateWorkDaySettings(database, startTime, endTime, "America/New_York", "Monday,Tuesday,Wednesday,Thursday,Friday")
	assert.NoError(t, err)

	// Verify in-memory update
	assert.True(t, user.WorkDayStartTime.Valid)
	assert.True(t, user.WorkDayEndTime.Valid)
	assert.True(t, user.Timezone.Valid)
	assert.Equal(t, "America/New_York", user.Timezone.String)
	assert.Equal(t, "Monday,Tuesday,Wednesday,Thursday,Friday", user.WorkDays)
}

func TestUpdateSummaryFilterSettings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = user.UpdateSummaryFilterSettings(database, "configured_tags", "assigned_tickets")
	assert.NoError(t, err)

	// Verify in-memory update
	assert.Equal(t, "configured_tags", user.SummaryTagFilter)
	assert.Equal(t, "assigned_tickets", user.SummaryTicketFilter)
}

// Tag Alert Tests

func TestCreateTagAlert(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create user first
	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = CreateTagAlert(database, user.ID, "urgent", "C12345678", "general", "sla_reply")
	assert.NoError(t, err)

	alerts, err := GetTagAlertsByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "urgent", alerts[0].Tag)
	assert.Equal(t, "C12345678", alerts[0].SlackChannelID)
	assert.Equal(t, "general", alerts[0].SlackChannelName)
	assert.Equal(t, "sla_reply", alerts[0].AlertType)
}

func TestGetTagAlertsByUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = CreateTagAlert(database, user.ID, "urgent", "C12345678", "general", "sla_reply")
	require.NoError(t, err)
	err = CreateTagAlert(database, user.ID, "billing", "C12345678", "general", "new_ticket")
	require.NoError(t, err)

	alerts, err := GetTagAlertsByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Len(t, alerts, 2)
}

func TestGetTagAlertsByUser_Empty(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	alerts, err := GetTagAlertsByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Empty(t, alerts)
}

func TestDeleteTagAlert(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = CreateTagAlert(database, user.ID, "urgent", "C12345678", "general", "sla_reply")
	require.NoError(t, err)

	alerts, err := GetTagAlertsByUser(database, user.ID)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	err = DeleteTagAlert(database, alerts[0].ID)
	assert.NoError(t, err)

	// Verify deletion
	alerts, err = GetTagAlertsByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Empty(t, alerts)
}

func TestGetAllTagAlerts(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create two users with alerts
	err := CreateUser(database, "user1@example.com", "User 1", AdminRole, false)
	require.NoError(t, err)
	err = CreateUser(database, "user2@example.com", "User 2", AgentRole, false)
	require.NoError(t, err)

	user1, err := GetUserByEmail(database, "user1@example.com")
	require.NoError(t, err)
	user2, err := GetUserByEmail(database, "user2@example.com")
	require.NoError(t, err)

	err = CreateTagAlert(database, user1.ID, "urgent", "C12345678", "general", "sla_reply")
	require.NoError(t, err)
	err = CreateTagAlert(database, user2.ID, "billing", "C87654321", "billing-team", "new_ticket")
	require.NoError(t, err)

	alerts, err := GetAllTagAlerts(database)
	assert.NoError(t, err)
	assert.Len(t, alerts, 2)

	// Verify that User is populated
	for _, alert := range alerts {
		assert.NotEmpty(t, alert.User.Name)
		assert.NotEmpty(t, alert.User.Email)
	}
}

func TestGetAllTagAlerts_Empty(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	alerts, err := GetAllTagAlerts(database)
	assert.NoError(t, err)
	assert.Empty(t, alerts)
}

// --- UpdateUser Tests ---

func TestUpdateUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Old Name", AgentRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	user.Name = "New Name"
	user.Role = AdminRole
	user.DailySummary = true
	err = UpdateUser(database, user)
	assert.NoError(t, err)

	updated, err := GetUserByID(database, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, AdminRole, updated.Role)
	assert.True(t, updated.DailySummary)
}

// --- UpdateSlackUserID Edge Cases ---

func TestUpdateSlackUserID_NonExistentUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := UpdateSlackUserID(database, "nonexistent@example.com", "U12345")
	assert.NoError(t, err)
}

func TestUpdateSlackUserID_Overwrite(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	err = UpdateSlackUserID(database, "test@example.com", "U_FIRST")
	require.NoError(t, err)

	err = UpdateSlackUserID(database, "test@example.com", "U_SECOND")
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	assert.NoError(t, err)
	assert.Equal(t, "U_SECOND", user.SlackUserID.String)
}

// --- GetUserByID Full Field Coverage ---

func TestGetUserByID_WithWorkDaySettings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)

	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	startTime := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)
	err = user.UpdateWorkDaySettings(database, startTime, endTime, "America/Chicago", "Monday,Tuesday,Wednesday")
	require.NoError(t, err)

	err = user.UpdateSummaryFilterSettings(database, "configured_tags", "assigned_tickets")
	require.NoError(t, err)

	err = UpdateSlackUserID(database, "test@example.com", "U_TEST123")
	require.NoError(t, err)

	fetched, err := GetUserByID(database, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", fetched.Email)
	assert.True(t, fetched.WorkDayStartTime.Valid)
	assert.True(t, fetched.WorkDayEndTime.Valid)
	assert.True(t, fetched.Timezone.Valid)
	assert.Equal(t, "America/Chicago", fetched.Timezone.String)
	assert.Equal(t, "Monday,Tuesday,Wednesday", fetched.WorkDays)
	assert.Equal(t, "configured_tags", fetched.SummaryTagFilter)
	assert.Equal(t, "assigned_tickets", fetched.SummaryTicketFilter)
	assert.True(t, fetched.SlackUserID.Valid)
	assert.Equal(t, "U_TEST123", fetched.SlackUserID.String)
}

// --- ValidRole Tests ---

func TestValidRole(t *testing.T) {
	assert.True(t, ValidRole(AdminRole))
	assert.True(t, ValidRole(AgentRole))
	assert.False(t, ValidRole(Role("superadmin")))
	assert.False(t, ValidRole(Role("")))
}

// --- GetAllUsersPaginated Tests ---

func TestGetAllUsersPaginated(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	for i := 0; i < 5; i++ {
		err := CreateUser(database, fmt.Sprintf("user%d@example.com", i), fmt.Sprintf("User %d", i), AgentRole, false)
		require.NoError(t, err)
	}

	page1, err := GetAllUsersPaginated(database, 2, 0)
	assert.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := GetAllUsersPaginated(database, 2, 2)
	assert.NoError(t, err)
	assert.Len(t, page2, 2)

	page3, err := GetAllUsersPaginated(database, 2, 4)
	assert.NoError(t, err)
	assert.Len(t, page3, 1)

	pageBeyond, err := GetAllUsersPaginated(database, 2, 10)
	assert.NoError(t, err)
	assert.Empty(t, pageBeyond)
}

// --- GetFirstUserID Tests ---

func TestGetFirstUserID_NoUsers(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	id, err := GetFirstUserID(database)
	assert.NoError(t, err)
	assert.Equal(t, 0, id)
}

func TestGetFirstUserID_WithUsers(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "first@example.com", "First", AdminRole, false)
	require.NoError(t, err)
	err = CreateUser(database, "second@example.com", "Second", AgentRole, false)
	require.NoError(t, err)

	id, err := GetFirstUserID(database)
	assert.NoError(t, err)
	assert.Greater(t, id, 0)
}

// --- GetAllTagAlertsPaginated Tests ---

func TestGetAllTagAlertsPaginated(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		err = CreateTagAlert(database, user.ID, fmt.Sprintf("tag%d", i), "C111", "general", "new_ticket")
		require.NoError(t, err)
	}

	page1, err := GetAllTagAlertsPaginated(database, 2, 0)
	assert.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.NotEmpty(t, page1[0].User.Name)

	page2, err := GetAllTagAlertsPaginated(database, 2, 2)
	assert.NoError(t, err)
	assert.Len(t, page2, 2)

	page3, err := GetAllTagAlertsPaginated(database, 10, 4)
	assert.NoError(t, err)
	assert.Len(t, page3, 1)
}

// --- GetTagAlertByID Tests ---

func TestGetTagAlertByID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = CreateTagAlert(database, user.ID, "urgent", "C111", "general", "new_ticket")
	require.NoError(t, err)

	alerts, err := GetTagAlertsByUser(database, user.ID)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert, err := GetTagAlertByID(database, alerts[0].ID)
	assert.NoError(t, err)
	assert.Equal(t, "urgent", alert.Tag)
	assert.Equal(t, "C111", alert.SlackChannelID)
}

func TestGetTagAlertByID_NotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	_, err := GetTagAlertByID(database, 99999)
	assert.Error(t, err)
}
