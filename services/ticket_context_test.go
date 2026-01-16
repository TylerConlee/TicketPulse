package services

import (
	"testing"

	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/stretchr/testify/assert"
)

func TestNewTicketContext(t *testing.T) {
	ticket := zendesk.Ticket{
		ID:      12345,
		Subject: "Test Ticket",
	}
	slaInfo := &SLAInfo{}

	ctx := NewTicketContext(ticket, slaInfo, "example")

	assert.Equal(t, ticket, ctx.Ticket)
	assert.Equal(t, slaInfo, ctx.SLAInfo)
	assert.Equal(t, "Unknown Requester", ctx.RequesterName)
	assert.Equal(t, "Unknown Organization", ctx.OrganizationName)
	assert.Equal(t, "https://example.zendesk.com/agent/tickets/12345", ctx.TicketURL)
}

func TestTicketContext_WithRequester(t *testing.T) {
	ticket := zendesk.Ticket{ID: 1}
	ctx := NewTicketContext(ticket, nil, "test")

	ctx = ctx.WithRequester("John Doe")

	assert.Equal(t, "John Doe", ctx.RequesterName)
}

func TestTicketContext_WithOrganization(t *testing.T) {
	ticket := zendesk.Ticket{ID: 1}
	ctx := NewTicketContext(ticket, nil, "test")

	ctx = ctx.WithOrganization("Acme Corp")

	assert.Equal(t, "Acme Corp", ctx.OrganizationName)
}

func TestTicketContext_Chaining(t *testing.T) {
	ticket := zendesk.Ticket{ID: 1}
	ctx := NewTicketContext(ticket, nil, "test").
		WithRequester("Jane Smith").
		WithOrganization("Tech Inc")

	assert.Equal(t, "Jane Smith", ctx.RequesterName)
	assert.Equal(t, "Tech Inc", ctx.OrganizationName)
}

func TestBuildTicketURL(t *testing.T) {
	tests := []struct {
		name      string
		subdomain string
		ticketID  int64
		expected  string
	}{
		{
			name:      "Normal ticket",
			subdomain: "example",
			ticketID:  12345,
			expected:  "https://example.zendesk.com/agent/tickets/12345",
		},
		{
			name:      "Zero ticket ID",
			subdomain: "test",
			ticketID:  0,
			expected:  "https://test.zendesk.com/agent/tickets/0",
		},
		{
			name:      "Empty subdomain",
			subdomain: "",
			ticketID:  100,
			expected:  "",
		},
		{
			name:      "Large ticket ID",
			subdomain: "company",
			ticketID:  999999999,
			expected:  "https://company.zendesk.com/agent/tickets/999999999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTicketURL(tt.subdomain, tt.ticketID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{12345, "12345"},
		{999999999, "999999999"},
		{-1, "-1"},
		{-12345, "-12345"},
	}

	for _, tt := range tests {
		result := formatInt64(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestCollectUniqueRequesterIDs(t *testing.T) {
	tickets := []zendesk.Ticket{
		{ID: 1, RequesterID: 100},
		{ID: 2, RequesterID: 200},
		{ID: 3, RequesterID: 100}, // Duplicate
		{ID: 4, RequesterID: 300},
		{ID: 5, RequesterID: 0},   // Zero - should be excluded
	}

	result := CollectUniqueRequesterIDs(tickets)

	assert.Len(t, result, 3)
	assert.Contains(t, result, int64(100))
	assert.Contains(t, result, int64(200))
	assert.Contains(t, result, int64(300))
	assert.NotContains(t, result, int64(0))
}

func TestCollectUniqueOrganizationIDs(t *testing.T) {
	tickets := []zendesk.Ticket{
		{ID: 1, OrganizationID: 10},
		{ID: 2, OrganizationID: 20},
		{ID: 3, OrganizationID: 10}, // Duplicate
		{ID: 4, OrganizationID: 0},  // Zero - should be excluded
		{ID: 5, OrganizationID: 30},
	}

	result := CollectUniqueOrganizationIDs(tickets)

	assert.Len(t, result, 3)
	assert.Contains(t, result, int64(10))
	assert.Contains(t, result, int64(20))
	assert.Contains(t, result, int64(30))
	assert.NotContains(t, result, int64(0))
}

func TestCollectUniqueRequesterIDs_Empty(t *testing.T) {
	result := CollectUniqueRequesterIDs([]zendesk.Ticket{})
	assert.Empty(t, result)
}

func TestCollectUniqueOrganizationIDs_Empty(t *testing.T) {
	result := CollectUniqueOrganizationIDs([]zendesk.Ticket{})
	assert.Empty(t, result)
}

func TestBuildTicketContextMap(t *testing.T) {
	contexts := []TicketContext{
		{Ticket: zendesk.Ticket{ID: 1}, RequesterName: "User1"},
		{Ticket: zendesk.Ticket{ID: 2}, RequesterName: "User2"},
		{Ticket: zendesk.Ticket{ID: 3}, RequesterName: "User3"},
	}

	contextMap := BuildTicketContextMap(contexts)

	assert.Len(t, contextMap, 3)
	assert.Equal(t, "User1", contextMap[1].RequesterName)
	assert.Equal(t, "User2", contextMap[2].RequesterName)
	assert.Equal(t, "User3", contextMap[3].RequesterName)
}

func TestBuildTicketContextMap_Empty(t *testing.T) {
	contextMap := BuildTicketContextMap([]TicketContext{})
	assert.Empty(t, contextMap)
}

func TestTicketContextMap_Lookup(t *testing.T) {
	contextMap := TicketContextMap{
		100: {Ticket: zendesk.Ticket{ID: 100}, RequesterName: "Test User"},
	}

	ctx, ok := contextMap[100]
	assert.True(t, ok)
	assert.Equal(t, "Test User", ctx.RequesterName)

	_, ok = contextMap[999]
	assert.False(t, ok)
}
