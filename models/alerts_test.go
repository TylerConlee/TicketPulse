package models

import (
	"context"
	"testing"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAlertsTestDB(t *testing.T) *db.SQLDatabase {
	database := db.InitDB(":memory:")
	require.NotNil(t, database, "Failed to initialize test database")
	return database
}

func createTestUser(t *testing.T, database *db.SQLDatabase, email string) User {
	err := CreateUser(database, email, "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, email)
	require.NoError(t, err)
	return user
}

func TestCreateSLAAlertCache(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	cacheEntry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "Less than 2 hours remaining",
	}

	err := CreateSLAAlertCache(ctx, database, cacheEntry)
	assert.NoError(t, err)
}

func TestGetSLAAlertCache(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	breachTime := time.Now().Add(2 * time.Hour)
	cacheEntry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   breachTime,
		Label:      "Less than 2 hours remaining",
	}

	err := CreateSLAAlertCache(ctx, database, cacheEntry)
	require.NoError(t, err)

	retrieved, err := GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_reply", "reply_time")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, int64(user.ID), retrieved.UserID)
	assert.Equal(t, int64(12345), retrieved.TicketID)
	assert.Equal(t, "sla_reply", retrieved.AlertType)
	assert.Equal(t, "reply_time", retrieved.MetricType)
	assert.Equal(t, "Less than 2 hours remaining", retrieved.Label)
}

func TestGetSLAAlertCache_NotFound(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	ctx := context.Background()

	retrieved, err := GetSLAAlertCache(ctx, database, 1, 99999, "sla_reply", "reply_time")
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestGetSLAAlertCache_DifferentMetricType(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	// Create cache entry for reply_time
	cacheEntry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "Less than 2 hours remaining",
	}
	err := CreateSLAAlertCache(ctx, database, cacheEntry)
	require.NoError(t, err)

	// Should not find when querying with different metric type
	retrieved, err := GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_reply", "resolution_time")
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestClearSLAAlertCache(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	cacheEntry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "Less than 2 hours remaining",
	}
	err := CreateSLAAlertCache(ctx, database, cacheEntry)
	require.NoError(t, err)

	// Get the created entry
	retrieved, err := GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_reply", "reply_time")
	require.NoError(t, err)

	// Clear the cache entry
	err = ClearSLAAlertCache(ctx, database, retrieved.ID)
	assert.NoError(t, err)

	// Verify deletion
	retrieved, err = GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_reply", "reply_time")
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestClearSLAAlertCacheByTicket(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	// Create multiple cache entries for the same ticket
	entry1 := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "Less than 2 hours remaining",
	}
	entry2 := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_resolution",
		MetricType: "resolution_time",
		BreachAt:   time.Now().Add(4 * time.Hour),
		Label:      "Less than 4 hours remaining",
	}

	err := CreateSLAAlertCache(ctx, database, entry1)
	require.NoError(t, err)
	err = CreateSLAAlertCache(ctx, database, entry2)
	require.NoError(t, err)

	// Clear all entries for the ticket
	err = ClearSLAAlertCacheByTicket(ctx, database, 12345)
	assert.NoError(t, err)

	// Verify both entries are deleted
	retrieved1, err := GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_reply", "reply_time")
	assert.Error(t, err)
	assert.Nil(t, retrieved1)

	retrieved2, err := GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_resolution", "resolution_time")
	assert.Error(t, err)
	assert.Nil(t, retrieved2)
}

func TestCreateAlertLog(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	logEntry := AlertLog{
		UserID:    int64(user.ID),
		TicketID:  12345,
		Tag:       "urgent",
		AlertType: "sla_reply",
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}

	err := CreateAlertLog(ctx, database, logEntry)
	assert.NoError(t, err)
}

func TestCreateDailySummaryLog(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	summaryDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	err := CreateDailySummaryLog(ctx, database, user.ID, summaryDate)
	assert.NoError(t, err)
}

func TestGetDailySummaryLog(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	summaryDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	err := CreateDailySummaryLog(ctx, database, user.ID, summaryDate)
	require.NoError(t, err)

	logEntry, err := GetDailySummaryLog(ctx, database, user.ID, summaryDate)
	assert.NoError(t, err)
	assert.NotNil(t, logEntry)
	assert.Equal(t, int64(user.ID), logEntry.UserID)
}

func TestGetDailySummaryLog_NotFound(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	summaryDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	logEntry, err := GetDailySummaryLog(ctx, database, user.ID, summaryDate)
	assert.Error(t, err)
	assert.Nil(t, logEntry)
}

func TestSLAAlertDeduplication(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	// Create initial cache entry
	cacheEntry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "Less than 2 hours remaining",
	}
	err := CreateSLAAlertCache(ctx, database, cacheEntry)
	require.NoError(t, err)

	// Try to find existing entry - should succeed
	existingEntry, err := GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_reply", "reply_time")
	assert.NoError(t, err)
	assert.NotNil(t, existingEntry)

	// Verify label matches - if same, we would skip sending alert
	assert.Equal(t, "Less than 2 hours remaining", existingEntry.Label)

	// If label changed (e.g., "Less than 1 hour remaining"), we would:
	// 1. Clear the old entry
	err = ClearSLAAlertCache(ctx, database, existingEntry.ID)
	assert.NoError(t, err)

	// 2. Create new entry with updated label
	newCacheEntry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(45 * time.Minute),
		Label:      "Less than 1 hour remaining",
	}
	err = CreateSLAAlertCache(ctx, database, newCacheEntry)
	assert.NoError(t, err)

	// Verify the new entry
	updatedEntry, err := GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_reply", "reply_time")
	assert.NoError(t, err)
	assert.Equal(t, "Less than 1 hour remaining", updatedEntry.Label)
}

func TestSLAAlertCache_SeparateMetricTypes(t *testing.T) {
	database := setupAlertsTestDB(t)
	defer database.Close()

	user := createTestUser(t, database, "test@example.com")
	ctx := context.Background()

	// Create cache entries for both reply_time and resolution_time
	replyEntry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(1 * time.Hour),
		Label:      "Less than 1 hour remaining",
	}
	resolutionEntry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  "sla_resolution",
		MetricType: "resolution_time",
		BreachAt:   time.Now().Add(4 * time.Hour),
		Label:      "Less than 4 hours remaining",
	}

	err := CreateSLAAlertCache(ctx, database, replyEntry)
	require.NoError(t, err)
	err = CreateSLAAlertCache(ctx, database, resolutionEntry)
	require.NoError(t, err)

	// Both should exist independently
	retrievedReply, err := GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_reply", "reply_time")
	assert.NoError(t, err)
	assert.NotNil(t, retrievedReply)
	assert.Equal(t, "Less than 1 hour remaining", retrievedReply.Label)

	retrievedResolution, err := GetSLAAlertCache(ctx, database, user.ID, 12345, "sla_resolution", "resolution_time")
	assert.NoError(t, err)
	assert.NotNil(t, retrievedResolution)
	assert.Equal(t, "Less than 4 hours remaining", retrievedResolution.Label)
}
