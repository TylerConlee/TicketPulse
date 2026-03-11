package services

import (
	"context"
	"testing"
	"time"

	"github.com/TylerConlee/TicketPulse/testutil"
	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/stretchr/testify/assert"
)

func TestNewPollingService(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	ps := NewPollingService(database, nil, nil, nil)

	assert.NotNil(t, ps)
	assert.Equal(t, database, ps.db)
	assert.NotNil(t, ps.pollingState)
	assert.NotNil(t, ps.clock)
}

func TestNewPollingServiceWithClock(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	mockClock := NewMockClock(fixedTime)

	ps := NewPollingServiceWithClock(database, nil, nil, mockClock, nil)

	assert.NotNil(t, ps)
	assert.Equal(t, mockClock, ps.clock)
	// Last poll time should be 5 minutes before the fixed time
	expectedLastPoll := fixedTime.Add(-5 * time.Minute)
	assert.Equal(t, expectedLastPoll, ps.GetLastPollTime())
}

func TestNewPollingStateWithClock(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	mockClock := NewMockClock(fixedTime)

	state := NewPollingStateWithClock(mockClock)

	assert.NotNil(t, state)
	expectedLastPoll := fixedTime.Add(-5 * time.Minute)
	assert.Equal(t, expectedLastPoll, state.lastPollTime)
}

func TestPollingService_GetSetLastPollTime(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	ps := NewPollingService(database, nil, nil, nil)

	newTime := time.Date(2025, 6, 1, 14, 0, 0, 0, time.UTC)
	ps.SetLastPollTime(newTime)

	assert.Equal(t, newTime, ps.GetLastPollTime())
}

func TestPollingResult_Fields(t *testing.T) {
	result := PollingResult{
		SLATicketsCount:       10,
		UpdatedTicketsCount:   5,
		ProcessedTicketsCount: 15,
		Error:                 nil,
	}

	assert.Equal(t, 10, result.SLATicketsCount)
	assert.Equal(t, 5, result.UpdatedTicketsCount)
	assert.Equal(t, 15, result.ProcessedTicketsCount)
	assert.Nil(t, result.Error)
}

func TestPollingService_PollOnce_NoConfiguration(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	ps := NewPollingService(database, nil, nil, nil)

	result := ps.PollOnce(context.Background())

	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "failed to create Zendesk client")
}

func TestDeduplicateTickets_NoDuplicates(t *testing.T) {
	tickets1 := []zendesk.Ticket{{ID: 1}, {ID: 2}}
	tickets2 := []zendesk.Ticket{{ID: 3}, {ID: 4}}

	result := deduplicateTickets(tickets1, tickets2)
	assert.Len(t, result, 4)
}

func TestDeduplicateTickets_WithDuplicates(t *testing.T) {
	tickets1 := []zendesk.Ticket{{ID: 1, Subject: "first"}, {ID: 2}}
	tickets2 := []zendesk.Ticket{{ID: 1, Subject: "duplicate"}, {ID: 3}}

	result := deduplicateTickets(tickets1, tickets2)
	assert.Len(t, result, 3)
	assert.Equal(t, "first", result[0].Subject)
}

func TestDeduplicateTickets_AllDuplicates(t *testing.T) {
	tickets1 := []zendesk.Ticket{{ID: 1}, {ID: 2}}
	tickets2 := []zendesk.Ticket{{ID: 1}, {ID: 2}}

	result := deduplicateTickets(tickets1, tickets2)
	assert.Len(t, result, 2)
}

func TestDeduplicateTickets_EmptySlices(t *testing.T) {
	result := deduplicateTickets([]zendesk.Ticket{}, []zendesk.Ticket{})
	assert.Empty(t, result)
}

func TestDeduplicateTickets_SingleSlice(t *testing.T) {
	tickets := []zendesk.Ticket{{ID: 1}, {ID: 2}, {ID: 1}}
	result := deduplicateTickets(tickets)
	assert.Len(t, result, 2)
}

func TestPollingService_ClockAdvance(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	startTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	mockClock := NewMockClock(startTime)

	ps := NewPollingServiceWithClock(database, nil, nil, mockClock, nil)

	// Initial last poll time should be 5 minutes before start
	assert.Equal(t, startTime.Add(-5*time.Minute), ps.GetLastPollTime())

	// Advance the clock
	mockClock.Advance(10 * time.Minute)

	// Now if we manually set the poll time to "now"
	ps.SetLastPollTime(mockClock.Now())

	// Last poll time should be the new time
	assert.Equal(t, startTime.Add(10*time.Minute), ps.GetLastPollTime())
}
