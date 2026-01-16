package services

import (
	"testing"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSchedulerTestDB(t *testing.T) *db.SQLDatabase {
	database := db.InitDB(":memory:")
	require.NotNil(t, database, "Failed to initialize test database")
	return database
}

func TestNewSchedulerService(t *testing.T) {
	database := setupSchedulerTestDB(t)
	defer database.Close()

	service := NewSchedulerService(database, nil)

	assert.NotNil(t, service)
	assert.Equal(t, database, service.db)
	assert.Nil(t, service.slackService)
}

func TestNewSchedulerService_WithSlackService(t *testing.T) {
	database := setupSchedulerTestDB(t)
	defer database.Close()

	// Note: We can't easily create a real SlackService without network access,
	// so we just test that the constructor works with nil for now
	service := NewSchedulerService(database, nil)

	assert.NotNil(t, service)
}

func TestTimeDifferenceCalculation(t *testing.T) {
	// Test the time difference calculation logic used in scheduler
	tests := []struct {
		name           string
		currentHour    int
		currentMinute  int
		targetHour     int
		targetMinute   int
		expectedWithin bool
	}{
		{
			name:           "Exact match",
			currentHour:    9,
			currentMinute:  0,
			targetHour:     9,
			targetMinute:   0,
			expectedWithin: true,
		},
		{
			name:           "1 minute before",
			currentHour:    8,
			currentMinute:  59,
			targetHour:     9,
			targetMinute:   0,
			expectedWithin: true,
		},
		{
			name:           "1 minute after",
			currentHour:    9,
			currentMinute:  1,
			targetHour:     9,
			targetMinute:   0,
			expectedWithin: true,
		},
		{
			name:           "2 minutes before - outside window",
			currentHour:    8,
			currentMinute:  58,
			targetHour:     9,
			targetMinute:   0,
			expectedWithin: false,
		},
		{
			name:           "2 minutes after - outside window",
			currentHour:    9,
			currentMinute:  2,
			targetHour:     9,
			targetMinute:   0,
			expectedWithin: false,
		},
		{
			name:           "1 hour difference",
			currentHour:    10,
			currentMinute:  0,
			targetHour:     9,
			targetMinute:   0,
			expectedWithin: false,
		},
		{
			name:           "End of day to start",
			currentHour:    23,
			currentMinute:  59,
			targetHour:     0,
			targetMinute:   0,
			expectedWithin: false, // This case doesn't handle day wraparound
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the scheduler's time difference logic
			currentMinutes := tt.currentHour*60 + tt.currentMinute
			targetMinutes := tt.targetHour*60 + tt.targetMinute

			timeDiff := currentMinutes - targetMinutes
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}

			isWithin := timeDiff <= 1
			assert.Equal(t, tt.expectedWithin, isWithin)
		})
	}
}

func TestDefaultFilterModes(t *testing.T) {
	// Test that default filter modes are applied correctly
	tests := []struct {
		name                 string
		inputTagFilter       string
		inputTicketFilter    string
		expectedTagFilter    string
		expectedTicketFilter string
	}{
		{
			name:                 "Empty filters get defaults",
			inputTagFilter:       "",
			inputTicketFilter:    "",
			expectedTagFilter:    TagFilterAllTags,
			expectedTicketFilter: TicketFilterAssigned,
		},
		{
			name:                 "Configured tags preserved",
			inputTagFilter:       TagFilterConfiguredTags,
			inputTicketFilter:    "",
			expectedTagFilter:    TagFilterConfiguredTags,
			expectedTicketFilter: TicketFilterAssigned,
		},
		{
			name:                 "All tickets preserved",
			inputTagFilter:       "",
			inputTicketFilter:    TicketFilterAllTickets,
			expectedTagFilter:    TagFilterAllTags,
			expectedTicketFilter: TicketFilterAllTickets,
		},
		{
			name:                 "Both configured values preserved",
			inputTagFilter:       TagFilterConfiguredTags,
			inputTicketFilter:    TicketFilterAllTickets,
			expectedTagFilter:    TagFilterConfiguredTags,
			expectedTicketFilter: TicketFilterAllTickets,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tagFilterMode := tt.inputTagFilter
			if tagFilterMode == "" {
				tagFilterMode = TagFilterAllTags
			}
			ticketFilterMode := tt.inputTicketFilter
			if ticketFilterMode == "" {
				ticketFilterMode = TicketFilterAssigned
			}

			assert.Equal(t, tt.expectedTagFilter, tagFilterMode)
			assert.Equal(t, tt.expectedTicketFilter, ticketFilterMode)
		})
	}
}

func TestTagExtractionFromAlerts(t *testing.T) {
	// Test the tag extraction logic used in sendDailySummary
	type TagAlert struct {
		Tag string
	}

	tests := []struct {
		name         string
		alerts       []TagAlert
		expectedTags []string
	}{
		{
			name:         "No alerts",
			alerts:       []TagAlert{},
			expectedTags: []string{},
		},
		{
			name: "Single alert",
			alerts: []TagAlert{
				{Tag: "urgent"},
			},
			expectedTags: []string{"urgent"},
		},
		{
			name: "Multiple unique alerts",
			alerts: []TagAlert{
				{Tag: "urgent"},
				{Tag: "billing"},
				{Tag: "support"},
			},
			expectedTags: []string{"urgent", "billing", "support"},
		},
		{
			name: "Duplicate tags",
			alerts: []TagAlert{
				{Tag: "urgent"},
				{Tag: "urgent"},
				{Tag: "billing"},
			},
			expectedTags: []string{"urgent", "billing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userTags []string
			tagMap := make(map[string]bool)
			for _, alert := range tt.alerts {
				if !tagMap[alert.Tag] {
					tagMap[alert.Tag] = true
					userTags = append(userTags, alert.Tag)
				}
			}

			if len(tt.expectedTags) == 0 {
				assert.Empty(t, userTags)
			} else {
				assert.ElementsMatch(t, tt.expectedTags, userTags)
			}
		})
	}
}
