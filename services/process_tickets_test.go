package services

import (
	"context"
	"testing"
	"time"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/testutil"
	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTagIndex(t *testing.T) {
	alerts := []models.TagAlert{
		{ID: 1, Tag: "urgent", AlertType: AlertTypeNewTicket, User: models.User{ID: 1, Email: "a@b.com"}},
		{ID: 2, Tag: "urgent", AlertType: AlertTypeSLAReply, User: models.User{ID: 2, Email: "c@d.com"}},
		{ID: 3, Tag: "billing", AlertType: AlertTypeNewTicket, User: models.User{ID: 1, Email: "a@b.com"}},
	}

	index := buildTagIndex(alerts)

	assert.Len(t, index, 2)
	assert.Len(t, index["urgent"], 2)
	assert.Len(t, index["billing"], 1)
}

func TestMatchAlertsForTicket(t *testing.T) {
	index := map[string][]models.TagAlert{
		"urgent":  {{ID: 1, Tag: "urgent"}},
		"billing": {{ID: 2, Tag: "billing"}, {ID: 3, Tag: "billing"}},
	}

	t.Run("matching tags", func(t *testing.T) {
		ticket := zendesk.Ticket{Tags: []string{"urgent", "billing"}}
		matched := matchAlertsForTicket(ticket, index)
		assert.Len(t, matched, 3)
	})

	t.Run("no matching tags", func(t *testing.T) {
		ticket := zendesk.Ticket{Tags: []string{"support", "general"}}
		matched := matchAlertsForTicket(ticket, index)
		assert.Empty(t, matched)
	})

	t.Run("empty ticket tags", func(t *testing.T) {
		ticket := zendesk.Ticket{Tags: []string{}}
		matched := matchAlertsForTicket(ticket, index)
		assert.Empty(t, matched)
	})
}

func TestMetricTypeToAlertType(t *testing.T) {
	assert.Equal(t, AlertTypeSLAReply, metricTypeToAlertType(MetricTypeReply))
	assert.Equal(t, AlertTypeSLAResolution, metricTypeToAlertType(MetricTypeResolution))
	assert.Equal(t, "", metricTypeToAlertType("unknown"))
}

func TestIsDuplicateSLAAlert(t *testing.T) {
	database := testutil.SetupTestDB()
	defer database.Close()

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	alert := models.TagAlert{User: user, Tag: "urgent"}
	ticket := zendesk.Ticket{}
	ticket.ID = 100

	t.Run("no existing cache entry", func(t *testing.T) {
		isDup := isDuplicateSLAAlert(ctx, database, alert, ticket, AlertTypeSLAReply, MetricTypeReply, "Less than 3 hours remaining")
		assert.False(t, isDup)
	})

	t.Run("same label is duplicate", func(t *testing.T) {
		entry := models.SLAAlertCache{
			UserID:     int64(user.ID),
			TicketID:   100,
			AlertType:  AlertTypeSLAReply,
			MetricType: MetricTypeReply,
			BreachAt:   time.Now().Add(2 * time.Hour),
			Label:      "Less than 3 hours remaining",
		}
		err := models.CreateSLAAlertCache(ctx, database, entry)
		require.NoError(t, err)

		isDup := isDuplicateSLAAlert(ctx, database, alert, ticket, AlertTypeSLAReply, MetricTypeReply, "Less than 3 hours remaining")
		assert.True(t, isDup)
	})

	t.Run("different label clears old and is not duplicate", func(t *testing.T) {
		isDup := isDuplicateSLAAlert(ctx, database, alert, ticket, AlertTypeSLAReply, MetricTypeReply, "Less than 2 hours remaining")
		assert.False(t, isDup)
	})
}

func TestLogAlert(t *testing.T) {
	alert := models.TagAlert{Tag: "urgent"}
	ticket := zendesk.Ticket{Subject: "Test ticket"}
	ticket.ID = 123

	assert.NotPanics(t, func() {
		logAlert(alert, ticket, AlertTypeNewTicket)
	})
}

func TestLogAlert_EmptyFields(t *testing.T) {
	alert := models.TagAlert{Tag: ""}
	ticket := zendesk.Ticket{Subject: ""}
	ticket.ID = 0

	assert.NotPanics(t, func() {
		logAlert(alert, ticket, "")
	})
}

func TestCacheSLAAlert(t *testing.T) {
	database := testutil.SetupTestDB()
	defer database.Close()

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	ctx := context.Background()
	alert := models.TagAlert{User: user, Tag: "urgent"}
	ticket := zendesk.Ticket{}
	ticket.ID = 300
	breachAt := time.Now().Add(2 * time.Hour)

	cacheSLAAlert(ctx, database, alert, ticket, AlertTypeSLAReply, MetricTypeReply, breachAt, "Less than 3 hours remaining")

	cached, err := models.GetSLAAlertCache(ctx, database, user.ID, 300, AlertTypeSLAReply, MetricTypeReply)
	require.NoError(t, err)
	assert.Equal(t, "Less than 3 hours remaining", cached.Label)
	assert.Equal(t, MetricTypeReply, cached.MetricType)
}

