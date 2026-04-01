package models

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Tag Cache Tests ---

func TestClearAndReplaceCachedTags_Empty(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := ClearAndReplaceCachedTags(database, []string{})
	assert.NoError(t, err)

	tags, err := GetCachedTags(database)
	assert.NoError(t, err)
	assert.Empty(t, tags)
}

func TestClearAndReplaceCachedTags_SingleTag(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := ClearAndReplaceCachedTags(database, []string{"billing"})
	assert.NoError(t, err)

	tags, err := GetCachedTags(database)
	assert.NoError(t, err)
	assert.Equal(t, []string{"billing"}, tags)
}

func TestClearAndReplaceCachedTags_MultipleTags(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := ClearAndReplaceCachedTags(database, []string{"urgent", "billing", "support"})
	assert.NoError(t, err)

	tags, err := GetCachedTags(database)
	assert.NoError(t, err)
	assert.Len(t, tags, 3)
	assert.Equal(t, []string{"billing", "support", "urgent"}, tags)
}

func TestClearAndReplaceCachedTags_ReplacesExisting(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := ClearAndReplaceCachedTags(database, []string{"old_tag1", "old_tag2"})
	require.NoError(t, err)

	tags, err := GetCachedTags(database)
	require.NoError(t, err)
	assert.Len(t, tags, 2)

	err = ClearAndReplaceCachedTags(database, []string{"new_tag1", "new_tag2", "new_tag3"})
	require.NoError(t, err)

	tags, err = GetCachedTags(database)
	assert.NoError(t, err)
	assert.Len(t, tags, 3)
	assert.Equal(t, []string{"new_tag1", "new_tag2", "new_tag3"}, tags)
}

func TestClearAndReplaceCachedTags_ClearsWhenEmptyAfterData(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := ClearAndReplaceCachedTags(database, []string{"tag1", "tag2"})
	require.NoError(t, err)

	err = ClearAndReplaceCachedTags(database, []string{})
	assert.NoError(t, err)

	tags, err := GetCachedTags(database)
	assert.NoError(t, err)
	assert.Empty(t, tags)
}

func TestGetCachedTags_Empty(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	tags, err := GetCachedTags(database)
	assert.NoError(t, err)
	assert.Empty(t, tags)
}

func TestGetCachedTags_OrderedAlphabetically(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := ClearAndReplaceCachedTags(database, []string{"zebra", "alpha", "mike", "bravo"})
	require.NoError(t, err)

	tags, err := GetCachedTags(database)
	assert.NoError(t, err)
	assert.Equal(t, []string{"alpha", "bravo", "mike", "zebra"}, tags)
}

// --- UpdateTagAlert Tests ---

func TestUpdateTagAlert(t *testing.T) {
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
	alertID := alerts[0].ID

	err = UpdateTagAlert(database, alertID, "billing", "C222", "billing-team", "sla_reply")
	assert.NoError(t, err)

	updated, err := GetTagAlertByID(database, alertID)
	assert.NoError(t, err)
	assert.Equal(t, "billing", updated.Tag)
	assert.Equal(t, "C222", updated.SlackChannelID)
	assert.Equal(t, "billing-team", updated.SlackChannelName)
	assert.Equal(t, "sla_reply", updated.AlertType)
}

func TestUpdateTagAlert_NonExistent(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := UpdateTagAlert(database, 99999, "tag", "chan", "name", "type")
	assert.NoError(t, err)
}

// --- DeleteTagAlertsByIDs Tests ---

func TestDeleteTagAlertsByIDs_DeleteMultiple(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = CreateTagAlert(database, user.ID, "tag1", "C111", "general", "new_ticket")
	require.NoError(t, err)
	err = CreateTagAlert(database, user.ID, "tag2", "C111", "general", "sla_reply")
	require.NoError(t, err)
	err = CreateTagAlert(database, user.ID, "tag3", "C222", "billing", "ticket_update")
	require.NoError(t, err)

	alerts, err := GetTagAlertsByUser(database, user.ID)
	require.NoError(t, err)
	require.Len(t, alerts, 3)

	err = DeleteTagAlertsByIDs(database, user.ID, []int{alerts[0].ID, alerts[1].ID})
	assert.NoError(t, err)

	remaining, err := GetTagAlertsByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, "tag3", remaining[0].Tag)
}

func TestDeleteTagAlertsByIDs_ScopedToUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "user1@example.com", "User 1", AdminRole, false)
	require.NoError(t, err)
	err = CreateUser(database, "user2@example.com", "User 2", AgentRole, false)
	require.NoError(t, err)

	user1, err := GetUserByEmail(database, "user1@example.com")
	require.NoError(t, err)
	user2, err := GetUserByEmail(database, "user2@example.com")
	require.NoError(t, err)

	err = CreateTagAlert(database, user1.ID, "tag1", "C111", "general", "new_ticket")
	require.NoError(t, err)
	err = CreateTagAlert(database, user2.ID, "tag2", "C222", "billing", "new_ticket")
	require.NoError(t, err)

	user2Alerts, err := GetTagAlertsByUser(database, user2.ID)
	require.NoError(t, err)
	require.Len(t, user2Alerts, 1)

	// User1 tries to delete User2's alert — should not work
	err = DeleteTagAlertsByIDs(database, user1.ID, []int{user2Alerts[0].ID})
	assert.NoError(t, err)

	// User2's alert should still exist
	remaining, err := GetTagAlertsByUser(database, user2.ID)
	assert.NoError(t, err)
	assert.Len(t, remaining, 1)
}

func TestDeleteTagAlertsByIDs_EmptyList(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := DeleteTagAlertsByIDs(database, 1, []int{})
	assert.NoError(t, err)
}

// --- GetTagAlertCountByUser Tests ---

func TestGetTagAlertCountByUser_Zero(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	count, err := GetTagAlertCountByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestGetTagAlertCountByUser_WithAlerts(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = CreateTagAlert(database, user.ID, "tag1", "C111", "general", "new_ticket")
	require.NoError(t, err)
	err = CreateTagAlert(database, user.ID, "tag2", "C222", "billing", "sla_reply")
	require.NoError(t, err)

	count, err := GetTagAlertCountByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestGetTagAlertCountByUser_IsolatedPerUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "user1@example.com", "User 1", AdminRole, false)
	require.NoError(t, err)
	err = CreateUser(database, "user2@example.com", "User 2", AgentRole, false)
	require.NoError(t, err)

	user1, err := GetUserByEmail(database, "user1@example.com")
	require.NoError(t, err)
	user2, err := GetUserByEmail(database, "user2@example.com")
	require.NoError(t, err)

	err = CreateTagAlert(database, user1.ID, "tag1", "C111", "general", "new_ticket")
	require.NoError(t, err)
	err = CreateTagAlert(database, user1.ID, "tag2", "C222", "billing", "sla_reply")
	require.NoError(t, err)
	err = CreateTagAlert(database, user2.ID, "tag3", "C333", "support", "ticket_update")
	require.NoError(t, err)

	count1, err := GetTagAlertCountByUser(database, user1.ID)
	assert.NoError(t, err)
	assert.Equal(t, 2, count1)

	count2, err := GetTagAlertCountByUser(database, user2.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, count2)
}

// --- Alert History Tests ---

func createTestUserWithAlertLogs(t *testing.T, database interface {
	Query(query string, args ...interface{}) (interface{ Close() error; Next() bool; Scan(dest ...interface{}) error }, error)
	QueryRow(query string, args ...interface{}) interface{ Scan(dest ...interface{}) error }
	Exec(query string, args ...interface{}) (interface{ RowsAffected() (int64, error) }, error)
}) {
	// Helper is not used; see actual tests below
}

func TestGetAlertHistoryStats_Empty(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	stats, err := GetAlertHistoryStats(database, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, 0, stats.TotalAlerts)
	assert.Equal(t, 0, stats.AlertsToday)
	assert.Empty(t, stats.TopTags)
	assert.Empty(t, stats.TopTypes)
}

func TestGetAlertHistoryStats_WithData(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	today := time.Now().Format("2006-01-02") + " 10:00:00"
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02") + " 10:00:00"

	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 100, Tag: "urgent", AlertType: "new_ticket", Timestamp: today})
	require.NoError(t, err)
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 101, Tag: "urgent", AlertType: "sla_reply", Timestamp: today})
	require.NoError(t, err)
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 102, Tag: "billing", AlertType: "new_ticket", Timestamp: yesterday})
	require.NoError(t, err)

	stats, err := GetAlertHistoryStats(database, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, 3, stats.TotalAlerts)
	assert.Equal(t, 2, stats.AlertsToday)
	assert.NotEmpty(t, stats.TopTags)
	assert.Equal(t, "urgent", stats.TopTags[0].Tag)
	assert.Equal(t, 2, stats.TopTags[0].Count)
	assert.NotEmpty(t, stats.TopTypes)
	assert.Equal(t, "new_ticket", stats.TopTypes[0].AlertType)
	assert.Equal(t, 2, stats.TopTypes[0].Count)
}

func TestGetAlertHistoryStats_IsolatedPerUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "user1@example.com", "User 1", AdminRole, false)
	require.NoError(t, err)
	err = CreateUser(database, "user2@example.com", "User 2", AgentRole, false)
	require.NoError(t, err)

	user1, err := GetUserByEmail(database, "user1@example.com")
	require.NoError(t, err)
	user2, err := GetUserByEmail(database, "user2@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	ts := time.Now().Format("2006-01-02") + " 12:00:00"

	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user1.ID), TicketID: 100, Tag: "urgent", AlertType: "new_ticket", Timestamp: ts})
	require.NoError(t, err)
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user2.ID), TicketID: 200, Tag: "billing", AlertType: "sla_reply", Timestamp: ts})
	require.NoError(t, err)

	stats1, _ := GetAlertHistoryStats(database, user1.ID)
	assert.Equal(t, 1, stats1.TotalAlerts)

	stats2, _ := GetAlertHistoryStats(database, user2.ID)
	assert.Equal(t, 1, stats2.TotalAlerts)
}

// --- GetAlertHistoryPaginated Tests ---

func TestGetAlertHistoryPaginated_Empty(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	logs, total, err := GetAlertHistoryPaginated(database, user.ID, 25, 0)
	assert.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, logs)
}

func TestGetAlertHistoryPaginated_WithData(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		ts := time.Now().Add(time.Duration(-i) * time.Hour).Format("2006-01-02 15:04:05")
		err = CreateAlertLog(ctx, database, AlertLog{
			UserID:    int64(user.ID),
			TicketID:  int64(100 + i),
			Tag:       "tag",
			AlertType: "new_ticket",
			Timestamp: ts,
		})
		require.NoError(t, err)
	}

	logs, total, err := GetAlertHistoryPaginated(database, user.ID, 25, 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, logs, 5)
}

func TestGetAlertHistoryPaginated_Pagination(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		ts := time.Now().Add(time.Duration(-i) * time.Hour).Format("2006-01-02 15:04:05")
		err = CreateAlertLog(ctx, database, AlertLog{
			UserID:    int64(user.ID),
			TicketID:  int64(100 + i),
			Tag:       "tag",
			AlertType: "new_ticket",
			Timestamp: ts,
		})
		require.NoError(t, err)
	}

	// First page
	page1, total, err := GetAlertHistoryPaginated(database, user.ID, 3, 0)
	assert.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Len(t, page1, 3)

	// Second page
	page2, total, err := GetAlertHistoryPaginated(database, user.ID, 3, 3)
	assert.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Len(t, page2, 3)

	// Pages should have different entries
	assert.NotEqual(t, page1[0].TicketID, page2[0].TicketID)

	// Beyond data
	pageBeyond, total, err := GetAlertHistoryPaginated(database, user.ID, 3, 100)
	assert.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Empty(t, pageBeyond)
}

// --- GetMostRecentAlertLogByTicketID Tests ---

func TestGetMostRecentAlertLogByTicketID_Found(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 500, Tag: "urgent", AlertType: "new_ticket", Timestamp: "2025-01-01 10:00:00"})
	require.NoError(t, err)
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 500, Tag: "urgent", AlertType: "sla_reply", Timestamp: "2025-01-02 10:00:00"})
	require.NoError(t, err)
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 500, Tag: "urgent", AlertType: "sla_resolution", Timestamp: "2025-01-03 10:00:00"})
	require.NoError(t, err)

	alertLog, err := GetMostRecentAlertLogByTicketID(database, 500)
	assert.NoError(t, err)
	require.NotNil(t, alertLog)
	assert.Equal(t, int64(500), alertLog.TicketID)
	assert.Equal(t, "sla_resolution", alertLog.AlertType)
	assert.Contains(t, alertLog.Timestamp, "2025-01-03")
}

func TestGetMostRecentAlertLogByTicketID_NotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	alertLog, err := GetMostRecentAlertLogByTicketID(database, 99999)
	assert.Error(t, err)
	assert.Nil(t, alertLog)
}

func TestGetMostRecentAlertLogByTicketID_SingleEntry(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 600, Tag: "billing", AlertType: "ticket_update", Timestamp: "2025-06-15 14:30:00"})
	require.NoError(t, err)

	alertLog, err := GetMostRecentAlertLogByTicketID(database, 600)
	assert.NoError(t, err)
	require.NotNil(t, alertLog)
	assert.Equal(t, "ticket_update", alertLog.AlertType)
	assert.Equal(t, "billing", alertLog.Tag)
}

func TestGetMostRecentAlertLogByTicketID_DifferentTickets(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 100, Tag: "tag1", AlertType: "new_ticket", Timestamp: "2025-01-01 10:00:00"})
	require.NoError(t, err)
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 200, Tag: "tag2", AlertType: "sla_reply", Timestamp: "2025-01-02 10:00:00"})
	require.NoError(t, err)

	alertLog, err := GetMostRecentAlertLogByTicketID(database, 100)
	assert.NoError(t, err)
	require.NotNil(t, alertLog)
	assert.Equal(t, int64(100), alertLog.TicketID)
	assert.Equal(t, "new_ticket", alertLog.AlertType)
}

// --- CreateAlertLog Tests ---

func TestCreateAlertLog_Success(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	entry := AlertLog{
		UserID:    int64(user.ID),
		TicketID:  123,
		Tag:       "urgent",
		AlertType: "new_ticket",
		Timestamp: "2025-03-09 12:00:00",
	}
	err = CreateAlertLog(ctx, database, entry)
	assert.NoError(t, err)

	logs, total, err := GetAlertHistoryPaginated(database, user.ID, 10, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, logs, 1)
	assert.Equal(t, int64(123), logs[0].TicketID)
	assert.Equal(t, "urgent", logs[0].Tag)
	assert.Equal(t, "new_ticket", logs[0].AlertType)
}

// --- SLA Alert Cache CRUD Tests ---

func TestSLAAlertCache_CreateAndGet(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	entry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   100,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "Less than 3 hours remaining",
	}
	err = CreateSLAAlertCache(ctx, database, entry)
	assert.NoError(t, err)

	cached, err := GetSLAAlertCache(ctx, database, user.ID, 100, "sla_reply", "reply_time")
	assert.NoError(t, err)
	require.NotNil(t, cached)
	assert.Equal(t, "Less than 3 hours remaining", cached.Label)
	assert.Equal(t, int64(100), cached.TicketID)
}

func TestSLAAlertCache_GetNotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	ctx := context.Background()
	cached, err := GetSLAAlertCache(ctx, database, 1, 999, "sla_reply", "reply_time")
	assert.Error(t, err)
	assert.Nil(t, cached)
}

func TestSLAAlertCache_ClearByID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	entry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   100,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "test",
	}
	err = CreateSLAAlertCache(ctx, database, entry)
	require.NoError(t, err)

	cached, err := GetSLAAlertCache(ctx, database, user.ID, 100, "sla_reply", "reply_time")
	require.NoError(t, err)
	require.NotNil(t, cached)

	err = ClearSLAAlertCache(ctx, database, cached.ID)
	assert.NoError(t, err)

	cached2, err := GetSLAAlertCache(ctx, database, user.ID, 100, "sla_reply", "reply_time")
	assert.Error(t, err)
	assert.Nil(t, cached2)
}

func TestSLAAlertCache_ClearByTicket(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	entries := []SLAAlertCache{
		{UserID: int64(user.ID), TicketID: 200, AlertType: "sla_reply", MetricType: "reply_time", BreachAt: time.Now().Add(2 * time.Hour), Label: "test reply"},
		{UserID: int64(user.ID), TicketID: 200, AlertType: "sla_resolution", MetricType: "resolution_time", BreachAt: time.Now().Add(4 * time.Hour), Label: "test resolution"},
	}
	for _, entry := range entries {
		err = CreateSLAAlertCache(ctx, database, entry)
		require.NoError(t, err)
	}

	err = ClearSLAAlertCacheByTicket(ctx, database, 200)
	assert.NoError(t, err)

	cached, err := GetSLAAlertCache(ctx, database, user.ID, 200, "sla_reply", "reply_time")
	assert.Error(t, err)
	assert.Nil(t, cached)
}

// --- DailySummaryLog Tests ---

func TestDailySummaryLog_CreateAndGet(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	summaryDate := time.Date(2025, 3, 9, 0, 0, 0, 0, time.UTC)

	err = CreateDailySummaryLog(ctx, database, user.ID, summaryDate)
	assert.NoError(t, err)

	logEntry, err := GetDailySummaryLog(ctx, database, user.ID, summaryDate)
	assert.NoError(t, err)
	require.NotNil(t, logEntry)
	assert.Equal(t, int64(user.ID), logEntry.UserID)
}

func TestDailySummaryLog_GetNotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	ctx := context.Background()
	logEntry, err := GetDailySummaryLog(ctx, database, 999, time.Now())
	assert.Error(t, err)
	assert.Nil(t, logEntry)
}

// --- ClearExpiredSLAAlertCache Tests ---

func TestClearExpiredSLAAlertCache_NoEntries(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	ctx := context.Background()
	err := ClearExpiredSLAAlertCache(ctx, database)
	assert.NoError(t, err)
}

func TestClearExpiredSLAAlertCache_OnlyOldEntriesRemoved(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()

	recentEntry := SLAAlertCache{
		UserID:     int64(user.ID),
		TicketID:   100,
		AlertType:  "sla_reply",
		MetricType: "reply_time",
		BreachAt:   time.Now().Add(2 * time.Hour),
		Label:      "recent",
	}
	err = CreateSLAAlertCache(ctx, database, recentEntry)
	require.NoError(t, err)

	err = ClearExpiredSLAAlertCache(ctx, database)
	assert.NoError(t, err)

	cached, err := GetSLAAlertCache(ctx, database, user.ID, 100, "sla_reply", "reply_time")
	assert.NoError(t, err)
	assert.NotNil(t, cached)
}

// --- SkippedAlert Tests ---

func TestCreateSkippedAlert_Success(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	entry := SkippedAlert{
		TicketID:   100,
		UserID:     int64(user.ID),
		Tag:        "billing",
		AlertType:  "new_ticket",
		SkipReason: SkipReasonNewTicketOutsideWindow,
	}
	err = CreateSkippedAlert(ctx, database, entry)
	assert.NoError(t, err)
}

func TestCreateSkippedAlert_WithMetricAndLabel(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	entry := SkippedAlert{
		TicketID:   200,
		UserID:     int64(user.ID),
		Tag:        "urgent",
		AlertType:  "sla_reply",
		SkipReason: SkipReasonSLADuplicate,
		MetricType: "reply_time",
		Label:      "Less than 1 hour remaining",
	}
	err = CreateSkippedAlert(ctx, database, entry)
	assert.NoError(t, err)
}

// --- AcknowledgmentLog Tests ---

func TestCreateAcknowledgmentLog_Success(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	err = CreateAlertLog(ctx, database, AlertLog{
		UserID:    int64(user.ID),
		TicketID:  300,
		Tag:       "billing",
		AlertType: "new_ticket",
		Timestamp: "2025-06-15 10:00:00",
	})
	require.NoError(t, err)

	alertLog, err := GetMostRecentAlertLogByTicketID(database, 300)
	require.NoError(t, err)

	entry := AcknowledgmentLog{
		AlertLogID:    &alertLog.ID,
		TicketID:      300,
		SlackUserID:   "U12345",
		SlackUserName: "testuser",
	}
	err = CreateAcknowledgmentLog(ctx, database, entry)
	assert.NoError(t, err)
}

func TestCreateAcknowledgmentLog_NilAlertLogID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	ctx := context.Background()
	entry := AcknowledgmentLog{
		AlertLogID:    nil,
		TicketID:      400,
		SlackUserID:   "U67890",
		SlackUserName: "anotheruser",
	}
	err := CreateAcknowledgmentLog(ctx, database, entry)
	assert.NoError(t, err)
}

// --- DailyAlertLogSent Tests ---

func TestDailyAlertLogSent_CreateAndGet(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	ctx := context.Background()
	dateStr := "2025-06-15"

	_, err := GetDailyAlertLogSent(ctx, database, dateStr)
	assert.Error(t, err)

	err = CreateDailyAlertLogSent(ctx, database, dateStr)
	assert.NoError(t, err)

	entry, err := GetDailyAlertLogSent(ctx, database, dateStr)
	assert.NoError(t, err)
	require.NotNil(t, entry)
	assert.Contains(t, entry.LogDate, dateStr)
}

func TestDailyAlertLogSent_DuplicateDate(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	ctx := context.Background()
	dateStr := "2025-06-15"

	err := CreateDailyAlertLogSent(ctx, database, dateStr)
	require.NoError(t, err)

	err = CreateDailyAlertLogSent(ctx, database, dateStr)
	assert.Error(t, err)
}

// --- DailyAlertSummary Tests ---

func TestGetDailyAlertSummary_Empty(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	ctx := context.Background()
	summary, err := GetDailyAlertSummary(ctx, database, "2025-06-15")
	assert.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, 0, summary.TotalAlerts)
	assert.Equal(t, 0, summary.PreviousDay)
	assert.Empty(t, summary.ByType)
	assert.Empty(t, summary.ByChannel)
	assert.Empty(t, summary.ByTag)
	assert.Empty(t, summary.TopTickets)
	assert.Empty(t, summary.Skipped)
	assert.Equal(t, 0, summary.Acknowledgments.TotalAcknowledged)
}

func TestGetDailyAlertSummary_WithAlertData(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	date := "2025-06-15"

	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 100, Tag: "urgent", AlertType: "new_ticket", Timestamp: date + " 10:00:00"})
	require.NoError(t, err)
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 101, Tag: "urgent", AlertType: "sla_reply", Timestamp: date + " 11:00:00"})
	require.NoError(t, err)
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 100, Tag: "billing", AlertType: "new_ticket", Timestamp: date + " 12:00:00"})
	require.NoError(t, err)

	summary, err := GetDailyAlertSummary(ctx, database, date)
	assert.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, 3, summary.TotalAlerts)
	assert.NotEmpty(t, summary.ByType)
	assert.NotEmpty(t, summary.ByTag)
	assert.NotEmpty(t, summary.TopTickets)
}

func TestGetDailyAlertSummary_WithSkippedAlerts(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()

	err = CreateSkippedAlert(ctx, database, SkippedAlert{
		TicketID:   100,
		UserID:     int64(user.ID),
		Tag:        "billing",
		AlertType:  "new_ticket",
		SkipReason: SkipReasonNewTicketOutsideWindow,
	})
	require.NoError(t, err)
	err = CreateSkippedAlert(ctx, database, SkippedAlert{
		TicketID:   200,
		UserID:     int64(user.ID),
		Tag:        "urgent",
		AlertType:  "sla_reply",
		SkipReason: SkipReasonSLADuplicate,
		MetricType: "reply_time",
		Label:      "Less than 1h remaining",
	})
	require.NoError(t, err)

	date := time.Now().Format("2006-01-02")
	summary, err := GetDailyAlertSummary(ctx, database, date)
	assert.NoError(t, err)
	require.NotNil(t, summary)
	assert.Len(t, summary.Skipped, 2)
}

func TestGetDailyAlertSummary_PreviousDayTrend(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()

	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 100, Tag: "urgent", AlertType: "new_ticket", Timestamp: "2025-06-14 10:00:00"})
	require.NoError(t, err)
	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 101, Tag: "urgent", AlertType: "new_ticket", Timestamp: "2025-06-14 11:00:00"})
	require.NoError(t, err)

	err = CreateAlertLog(ctx, database, AlertLog{UserID: int64(user.ID), TicketID: 200, Tag: "billing", AlertType: "sla_reply", Timestamp: "2025-06-15 10:00:00"})
	require.NoError(t, err)

	summary, err := GetDailyAlertSummary(ctx, database, "2025-06-15")
	assert.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, 1, summary.TotalAlerts)
	assert.Equal(t, 2, summary.PreviousDay)
}

// --- Cleanup Tests ---

func TestClearOldSkippedAlerts(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	err = CreateSkippedAlert(ctx, database, SkippedAlert{
		TicketID:   100,
		UserID:     int64(user.ID),
		Tag:        "billing",
		AlertType:  "new_ticket",
		SkipReason: SkipReasonNewTicketOutsideWindow,
	})
	require.NoError(t, err)

	err = ClearOldSkippedAlerts(ctx, database, 30)
	assert.NoError(t, err)
}

func TestClearOldAcknowledgmentLogs(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	ctx := context.Background()
	err := CreateAcknowledgmentLog(ctx, database, AcknowledgmentLog{
		TicketID:      100,
		SlackUserID:   "U123",
		SlackUserName: "testuser",
	})
	require.NoError(t, err)

	err = ClearOldAcknowledgmentLogs(ctx, database, 30)
	assert.NoError(t, err)
}

func TestGetAlertHistoryPaginated_OrderedByTimestampDesc(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	err := CreateUser(database, "test@example.com", "Test User", AdminRole, false)
	require.NoError(t, err)
	user, err := GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	timestamps := []string{
		"2025-01-01 10:00:00",
		"2025-01-03 10:00:00",
		"2025-01-02 10:00:00",
	}
	for i, ts := range timestamps {
		err = CreateAlertLog(ctx, database, AlertLog{
			UserID:    int64(user.ID),
			TicketID:  int64(100 + i),
			Tag:       "tag",
			AlertType: "new_ticket",
			Timestamp: ts,
		})
		require.NoError(t, err)
	}

	logs, _, err := GetAlertHistoryPaginated(database, user.ID, 25, 0)
	assert.NoError(t, err)
	require.Len(t, logs, 3)
	assert.Contains(t, logs[0].Timestamp, "2025-01-03")
	assert.Contains(t, logs[1].Timestamp, "2025-01-02")
	assert.Contains(t, logs[2].Timestamp, "2025-01-01")
}
