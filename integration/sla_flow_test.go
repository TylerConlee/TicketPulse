package integration

import (
	"context"
	"testing"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIntegrationTestDB(t *testing.T) *db.SQLDatabase {
	database := db.InitDB(":memory:")
	require.NotNil(t, database, "Failed to initialize test database")
	return database
}

func createIntegrationTestUser(t *testing.T, database *db.SQLDatabase, email string) models.User {
	err := models.CreateUser(database, email, "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, email)
	require.NoError(t, err)
	return user
}

// TestSLAAlertCacheDeduplication tests the SLA alert deduplication workflow
func TestSLAAlertCacheDeduplication(t *testing.T) {
	database := setupIntegrationTestDB(t)
	defer database.Close()

	user := createIntegrationTestUser(t, database, "test@example.com")
	ctx := context.Background()

	// Create an initial SLA alert cache entry
	cacheEntry := models.SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  services.AlertTypeSLAReply,
		MetricType: services.MetricTypeReply,
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "Less than 2 hours remaining",
	}

	err := models.CreateSLAAlertCache(ctx, database, cacheEntry)
	require.NoError(t, err)

	// Try to retrieve the same alert - should find it (deduplication)
	existingEntry, err := models.GetSLAAlertCache(ctx, database, user.ID, 12345, services.AlertTypeSLAReply, services.MetricTypeReply)
	assert.NoError(t, err)
	assert.NotNil(t, existingEntry)
	assert.Equal(t, "Less than 2 hours remaining", existingEntry.Label)

	// If label matches, we should skip sending another alert (deduplication logic)
	shouldSkip := existingEntry.Label == "Less than 2 hours remaining"
	assert.True(t, shouldSkip, "Should skip sending alert when label matches")
}

// TestSLAAlertCacheLabelChange tests that alerts are sent when SLA label changes
func TestSLAAlertCacheLabelChange(t *testing.T) {
	database := setupIntegrationTestDB(t)
	defer database.Close()

	user := createIntegrationTestUser(t, database, "test@example.com")
	ctx := context.Background()

	// Create an initial SLA alert cache entry with "Less than 2 hours remaining"
	cacheEntry := models.SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  services.AlertTypeSLAReply,
		MetricType: services.MetricTypeReply,
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "Less than 2 hours remaining",
	}

	err := models.CreateSLAAlertCache(ctx, database, cacheEntry)
	require.NoError(t, err)

	// Now check with a new label (SLA progressed)
	newLabel := "Less than 1 hour remaining"
	existingEntry, err := models.GetSLAAlertCache(ctx, database, user.ID, 12345, services.AlertTypeSLAReply, services.MetricTypeReply)
	require.NoError(t, err)

	// Labels don't match, so we should clear old cache and send new alert
	shouldSendNewAlert := existingEntry.Label != newLabel
	assert.True(t, shouldSendNewAlert, "Should send new alert when label changes")

	// Clear the old cache entry
	err = models.ClearSLAAlertCache(ctx, database, existingEntry.ID)
	assert.NoError(t, err)

	// Create new cache entry with updated label
	newCacheEntry := models.SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   12345,
		AlertType:  services.AlertTypeSLAReply,
		MetricType: services.MetricTypeReply,
		BreachAt:   time.Now().Add(45 * time.Minute),
		Label:      newLabel,
	}

	err = models.CreateSLAAlertCache(ctx, database, newCacheEntry)
	assert.NoError(t, err)

	// Verify the new entry
	updatedEntry, err := models.GetSLAAlertCache(ctx, database, user.ID, 12345, services.AlertTypeSLAReply, services.MetricTypeReply)
	assert.NoError(t, err)
	assert.Equal(t, newLabel, updatedEntry.Label)
}

// TestSLAAlertSeparateMetricTypes tests that reply and resolution SLAs are tracked separately
func TestSLAAlertSeparateMetricTypes(t *testing.T) {
	database := setupIntegrationTestDB(t)
	defer database.Close()

	user := createIntegrationTestUser(t, database, "test@example.com")
	ctx := context.Background()

	ticketID := int64(12345)

	// Create SLA alert for reply time
	replyEntry := models.SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   ticketID,
		AlertType:  services.AlertTypeSLAReply,
		MetricType: services.MetricTypeReply,
		BreachAt:   time.Now().Add(1 * time.Hour),
		Label:      "Less than 1 hour remaining",
	}

	// Create SLA alert for resolution time
	resolutionEntry := models.SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   ticketID,
		AlertType:  services.AlertTypeSLAResolution,
		MetricType: services.MetricTypeResolution,
		BreachAt:   time.Now().Add(4 * time.Hour),
		Label:      "Less than 4 hours remaining",
	}

	err := models.CreateSLAAlertCache(ctx, database, replyEntry)
	require.NoError(t, err)
	err = models.CreateSLAAlertCache(ctx, database, resolutionEntry)
	require.NoError(t, err)

	// Verify both entries exist independently
	replyCache, err := models.GetSLAAlertCache(ctx, database, user.ID, int(ticketID), services.AlertTypeSLAReply, services.MetricTypeReply)
	assert.NoError(t, err)
	assert.Equal(t, "Less than 1 hour remaining", replyCache.Label)

	resolutionCache, err := models.GetSLAAlertCache(ctx, database, user.ID, int(ticketID), services.AlertTypeSLAResolution, services.MetricTypeResolution)
	assert.NoError(t, err)
	assert.Equal(t, "Less than 4 hours remaining", resolutionCache.Label)

	// Clear only the reply cache
	err = models.ClearSLAAlertCache(ctx, database, replyCache.ID)
	assert.NoError(t, err)

	// Reply should be gone, but resolution should still exist
	_, err = models.GetSLAAlertCache(ctx, database, user.ID, int(ticketID), services.AlertTypeSLAReply, services.MetricTypeReply)
	assert.Error(t, err, "Reply cache should be cleared")

	resolutionCacheStillExists, err := models.GetSLAAlertCache(ctx, database, user.ID, int(ticketID), services.AlertTypeSLAResolution, services.MetricTypeResolution)
	assert.NoError(t, err)
	assert.NotNil(t, resolutionCacheStillExists)
}

// TestSlaConditionMatchesIntegration tests the SlaConditionMatches function with various scenarios
func TestSlaConditionMatchesIntegration(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name         string
		metric       services.SLAPolicyMetric
		expectMatch  bool
		expectLabel  string
		expectMetric string
	}{
		{
			name: "Reply time approaching breach - 2.5 hours",
			metric: services.SLAPolicyMetric{
				BreachAt: now.Add(2*time.Hour + 30*time.Minute),
				Stage:    "active",
				Metric:   "first_reply_time",
			},
			expectMatch:  true,
			expectLabel:  "Less than 3 hours remaining",
			expectMetric: services.MetricTypeReply,
		},
		{
			name: "Resolution time approaching breach - 20 minutes",
			metric: services.SLAPolicyMetric{
				BreachAt: now.Add(20 * time.Minute),
				Stage:    "active",
				Metric:   "full_resolution_time",
			},
			expectMatch:  true,
			expectLabel:  "Less than 30 minutes remaining",
			expectMetric: services.MetricTypeResolution,
		},
		{
			name: "Reply time breached",
			metric: services.SLAPolicyMetric{
				BreachAt: now.Add(-5 * time.Minute),
				Stage:    "active",
				Metric:   "reply_time",
			},
			expectMatch:  true,
			expectLabel:  "BREACHED",
			expectMetric: services.MetricTypeReply,
		},
		{
			name: "Inactive metric - should not match",
			metric: services.SLAPolicyMetric{
				BreachAt: now.Add(1 * time.Hour),
				Stage:    "inactive",
				Metric:   "reply_time",
			},
			expectMatch: false,
		},
		{
			name: "More than 3 hours remaining - no alert",
			metric: services.SLAPolicyMetric{
				BreachAt: now.Add(5 * time.Hour),
				Stage:    "active",
				Metric:   "resolution_time",
			},
			expectMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			label, _, metricType, matches := services.SlaConditionMatches(tc.metric)

			assert.Equal(t, tc.expectMatch, matches)
			if matches {
				assert.Equal(t, tc.expectLabel, label)
				assert.Equal(t, tc.expectMetric, metricType)
			}
		})
	}
}

// TestAlertLogCreation tests the alert logging functionality
func TestAlertLogCreation(t *testing.T) {
	database := setupIntegrationTestDB(t)
	defer database.Close()

	user := createIntegrationTestUser(t, database, "test@example.com")
	ctx := context.Background()

	// Create an alert log entry
	alertLog := models.AlertLog{
		UserID:    int64(user.ID),
		TicketID:  12345,
		Tag:       "urgent",
		AlertType: services.AlertTypeSLAReply,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}

	err := models.CreateAlertLog(ctx, database, alertLog)
	assert.NoError(t, err)
}

// TestTagAlertWithUserAssociation tests tag alerts are properly associated with users
func TestTagAlertWithUserAssociation(t *testing.T) {
	database := setupIntegrationTestDB(t)
	defer database.Close()

	user1 := createIntegrationTestUser(t, database, "user1@example.com")
	user2 := createIntegrationTestUser(t, database, "user2@example.com")

	// Create tag alerts for each user
	err := models.CreateTagAlert(database, user1.ID, "billing", "C12345", services.AlertTypeSLAReply)
	require.NoError(t, err)
	err = models.CreateTagAlert(database, user1.ID, "urgent", "C12345", services.AlertTypeSLAResolution)
	require.NoError(t, err)
	err = models.CreateTagAlert(database, user2.ID, "support", "C67890", services.AlertTypeNewTicket)
	require.NoError(t, err)

	// Verify user1's alerts
	user1Alerts, err := models.GetTagAlertsByUser(database, user1.ID)
	assert.NoError(t, err)
	assert.Len(t, user1Alerts, 2)

	// Verify user2's alerts
	user2Alerts, err := models.GetTagAlertsByUser(database, user2.ID)
	assert.NoError(t, err)
	assert.Len(t, user2Alerts, 1)

	// Verify GetAllTagAlerts returns all with user info
	allAlerts, err := models.GetAllTagAlerts(database)
	assert.NoError(t, err)
	assert.Len(t, allAlerts, 3)

	// Verify user info is populated
	for _, alert := range allAlerts {
		assert.NotEmpty(t, alert.User.Email)
		assert.NotEmpty(t, alert.User.Name)
	}
}
