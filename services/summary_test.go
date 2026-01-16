package services

import (
	"testing"
	"time"

	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/stretchr/testify/assert"
)

func TestCalculatePreviousWorkDayEnd(t *testing.T) {
	tests := []struct {
		name         string
		workDayStart time.Time
		workDayEnd   time.Time
		timezone     string
		expectError  bool
	}{
		{
			name:         "Valid timezone - UTC",
			workDayStart: time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC),
			workDayEnd:   time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
			timezone:     "UTC",
			expectError:  false,
		},
		{
			name:         "Valid timezone - America/New_York",
			workDayStart: time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC),
			workDayEnd:   time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
			timezone:     "America/New_York",
			expectError:  false,
		},
		{
			name:         "Valid timezone - Europe/London",
			workDayStart: time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC),
			workDayEnd:   time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
			timezone:     "Europe/London",
			expectError:  false,
		},
		{
			name:         "Invalid timezone",
			workDayStart: time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC),
			workDayEnd:   time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
			timezone:     "Invalid/Timezone",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := calculatePreviousWorkDayEnd(tt.workDayStart, tt.workDayEnd, tt.timezone)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFilterTicketsByTags(t *testing.T) {
	tickets := []zendesk.Ticket{
		{ID: 1, Tags: []string{"billing", "urgent", "support"}},
		{ID: 2, Tags: []string{"sales", "priority"}},
		{ID: 3, Tags: []string{"billing"}},
		{ID: 4, Tags: []string{"support", "technical"}},
		{ID: 5, Tags: []string{}},
	}

	tests := []struct {
		name        string
		userTags    []string
		expectedIDs []int64
	}{
		{
			name:        "Single tag filter",
			userTags:    []string{"billing"},
			expectedIDs: []int64{1, 3},
		},
		{
			name:        "Multiple tags filter",
			userTags:    []string{"billing", "sales"},
			expectedIDs: []int64{1, 2, 3},
		},
		{
			name:        "No matching tags",
			userTags:    []string{"nonexistent"},
			expectedIDs: []int64{},
		},
		{
			name:        "Empty user tags",
			userTags:    []string{},
			expectedIDs: []int64{},
		},
		{
			name:        "All tickets match with support tag",
			userTags:    []string{"support"},
			expectedIDs: []int64{1, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterTicketsByTags(tickets, tt.userTags)

			var resultIDs []int64
			for _, ticket := range result {
				resultIDs = append(resultIDs, ticket.ID)
			}

			if len(tt.expectedIDs) == 0 {
				assert.Empty(t, resultIDs)
			} else {
				assert.ElementsMatch(t, tt.expectedIDs, resultIDs)
			}
		})
	}
}

func TestFilterUnreadTickets(t *testing.T) {
	// Create tickets with different Via sources
	// Note: The Via struct in zendesk.Ticket has an anonymous Source struct
	ticket1 := zendesk.Ticket{ID: 1}
	ticket1.Via = &zendesk.Via{Channel: "email"}
	ticket1.Via.Source.From = map[string]interface{}{"address": "customer@example.com"}

	ticket2 := zendesk.Ticket{ID: 2}
	ticket2.Via = &zendesk.Via{Channel: "email"}
	ticket2.Via.Source.From = map[string]interface{}{"address": "agent@example.com"}

	ticket3 := zendesk.Ticket{ID: 3}
	ticket3.Via = &zendesk.Via{Channel: "email"}
	ticket3.Via.Source.From = map[string]interface{}{"address": "customer@example.com"}

	ticket4 := zendesk.Ticket{ID: 4}
	ticket4.Via = &zendesk.Via{Channel: "email"}
	ticket4.Via.Source.From = map[string]interface{}{} // No address

	tickets := []zendesk.Ticket{ticket1, ticket2, ticket3, ticket4}

	tests := []struct {
		name        string
		userEmail   string
		expectedIDs []int64
	}{
		{
			name:        "Filter out agent's own tickets",
			userEmail:   "agent@example.com",
			expectedIDs: []int64{1, 3, 4},
		},
		{
			name:        "Filter out customer tickets",
			userEmail:   "customer@example.com",
			expectedIDs: []int64{2, 4},
		},
		{
			name:        "No matching emails - keep all",
			userEmail:   "other@example.com",
			expectedIDs: []int64{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterUnreadTickets(tickets, tt.userEmail)

			var resultIDs []int64
			for _, ticket := range result {
				resultIDs = append(resultIDs, ticket.ID)
			}

			assert.ElementsMatch(t, tt.expectedIDs, resultIDs)
		})
	}
}

func TestFilterTicketsWithActiveSLA(t *testing.T) {
	now := time.Now()

	tickets := []zendesk.Ticket{
		{ID: 1},
		{ID: 2},
		{ID: 3},
		{ID: 4},
	}

	slaData := map[int64]SLAInfo{
		1: {
			PolicyMetrics: []SLAPolicyMetric{
				{BreachAt: now.Add(1 * time.Hour), Stage: "active", Metric: "reply_time"},
			},
		},
		2: {
			PolicyMetrics: []SLAPolicyMetric{
				{BreachAt: now.Add(-1 * time.Hour), Stage: "completed", Metric: "reply_time"},
			},
		},
		3: {
			PolicyMetrics: []SLAPolicyMetric{
				{BreachAt: now.Add(2 * time.Hour), Stage: "active", Metric: "resolution_time"},
				{BreachAt: now.Add(-30 * time.Minute), Stage: "completed", Metric: "reply_time"},
			},
		},
		// Ticket 4 has no SLA data
	}

	tests := []struct {
		name        string
		tickets     []zendesk.Ticket
		slaData     map[int64]SLAInfo
		expectedIDs []int64
	}{
		{
			name:        "Filter tickets with active SLA",
			tickets:     tickets,
			slaData:     slaData,
			expectedIDs: []int64{1, 3},
		},
		{
			name:        "Empty SLA data",
			tickets:     tickets,
			slaData:     map[int64]SLAInfo{},
			expectedIDs: []int64{},
		},
		{
			name:        "Empty tickets",
			tickets:     []zendesk.Ticket{},
			slaData:     slaData,
			expectedIDs: []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterTicketsWithActiveSLA(tt.tickets, tt.slaData)

			var resultIDs []int64
			for _, ticket := range result {
				resultIDs = append(resultIDs, ticket.ID)
			}

			if len(tt.expectedIDs) == 0 {
				assert.Empty(t, resultIDs)
			} else {
				assert.ElementsMatch(t, tt.expectedIDs, resultIDs)
			}
		})
	}
}

func TestSummaryFilterConstants(t *testing.T) {
	// Verify filter constants are defined correctly
	assert.Equal(t, "all_tags", TagFilterAllTags)
	assert.Equal(t, "configured_tags", TagFilterConfiguredTags)
	assert.Equal(t, "all_tickets", TicketFilterAllTickets)
	assert.Equal(t, "assigned_tickets", TicketFilterAssigned)
}

func TestCompileSummaryMessage(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		userName       string
		unreadTickets  []zendesk.Ticket
		openTicketsSLA []zendesk.Ticket
		csatRatings    []SatisfactionRating
		since          time.Time
		expectContains []string
	}{
		{
			name:           "Basic summary with tickets",
			userName:       "Test User",
			unreadTickets:  []zendesk.Ticket{{ID: 1, Subject: "Test Ticket"}},
			openTicketsSLA: []zendesk.Ticket{{ID: 2, Subject: "SLA Ticket"}},
			csatRatings:    []SatisfactionRating{},
			since:          now.Add(-24 * time.Hour),
			expectContains: []string{"Test User", "1"},
		},
		{
			name:           "Empty summary",
			userName:       "Empty User",
			unreadTickets:  []zendesk.Ticket{},
			openTicketsSLA: []zendesk.Ticket{},
			csatRatings:    []SatisfactionRating{},
			since:          now.Add(-24 * time.Hour),
			expectContains: []string{"Empty User"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compileSummaryMessage(tt.userName, tt.unreadTickets, tt.openTicketsSLA, tt.csatRatings, tt.since)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}
