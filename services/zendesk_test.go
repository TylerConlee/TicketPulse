package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/stretchr/testify/assert"
)

func TestTagMatches(t *testing.T) {
	tests := []struct {
		name       string
		alertTag   string
		ticketTags []string
		expected   bool
	}{
		{
			name:       "Tag matches exactly",
			alertTag:   "urgent",
			ticketTags: []string{"billing", "urgent", "support"},
			expected:   true,
		},
		{
			name:       "Tag does not match",
			alertTag:   "urgent",
			ticketTags: []string{"billing", "support"},
			expected:   false,
		},
		{
			name:       "Empty ticket tags",
			alertTag:   "urgent",
			ticketTags: []string{},
			expected:   false,
		},
		{
			name:       "Single matching tag",
			alertTag:   "urgent",
			ticketTags: []string{"urgent"},
			expected:   true,
		},
		{
			name:       "Case sensitive - no match",
			alertTag:   "Urgent",
			ticketTags: []string{"urgent"},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tagMatches(tt.alertTag, tt.ticketTags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNewTicket(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		expected  bool
	}{
		{
			name:      "Ticket created just now",
			createdAt: now,
			expected:  true,
		},
		{
			name:      "Ticket created 2 minutes ago",
			createdAt: now.Add(-2 * time.Minute),
			expected:  true,
		},
		{
			name:      "Ticket created 4 minutes ago",
			createdAt: now.Add(-4 * time.Minute),
			expected:  true,
		},
		{
			name:      "Ticket created 6 minutes ago - not new",
			createdAt: now.Add(-6 * time.Minute),
			expected:  false,
		},
		{
			name:      "Ticket created 1 hour ago - not new",
			createdAt: now.Add(-1 * time.Hour),
			expected:  false,
		},
		{
			name:      "Ticket created 1 day ago - not new",
			createdAt: now.Add(-24 * time.Hour),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdAt := tt.createdAt
			ticket := zendesk.Ticket{
				CreatedAt: &createdAt,
			}
			result := isNewTicket(ticket)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsUpdatedTicket(t *testing.T) {
	now := time.Now()
	lastPollTime := now.Add(-5 * time.Minute)

	tests := []struct {
		name         string
		updatedAt    time.Time
		lastPollTime time.Time
		expected     bool
	}{
		{
			name:         "Ticket updated after last poll",
			updatedAt:    now.Add(-2 * time.Minute),
			lastPollTime: lastPollTime,
			expected:     true,
		},
		{
			name:         "Ticket updated at last poll time",
			updatedAt:    lastPollTime,
			lastPollTime: lastPollTime,
			expected:     false,
		},
		{
			name:         "Ticket updated before last poll",
			updatedAt:    now.Add(-10 * time.Minute),
			lastPollTime: lastPollTime,
			expected:     false,
		},
		{
			name:         "Ticket updated just now",
			updatedAt:    now,
			lastPollTime: lastPollTime,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedAt := tt.updatedAt
			ticket := zendesk.Ticket{
				UpdatedAt: &updatedAt,
			}
			result := isUpdatedTicket(ticket, tt.lastPollTime)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSlaConditionMatches_MetricTypes(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		metric        SLAPolicyMetric
		expectedLabel string
		expectedColor string
		expectedType  string
		shouldMatch   bool
	}{
		{
			name: "Reply time metric - first_reply_time",
			metric: SLAPolicyMetric{
				BreachAt: now.Add(2*time.Hour + 30*time.Minute),
				Stage:    "active",
				Metric:   "first_reply_time",
			},
			expectedLabel: "Less than 3 hours remaining",
			expectedColor: "#3498DB",
			expectedType:  MetricTypeReply,
			shouldMatch:   true,
		},
		{
			name: "Reply time metric - reply_time",
			metric: SLAPolicyMetric{
				BreachAt: now.Add(2*time.Hour + 30*time.Minute),
				Stage:    "active",
				Metric:   "reply_time",
			},
			expectedLabel: "Less than 3 hours remaining",
			expectedColor: "#3498DB",
			expectedType:  MetricTypeReply,
			shouldMatch:   true,
		},
		{
			name: "Resolution time metric - resolution_time",
			metric: SLAPolicyMetric{
				BreachAt: now.Add(1*time.Hour + 30*time.Minute),
				Stage:    "active",
				Metric:   "resolution_time",
			},
			expectedLabel: "Less than 2 hours remaining",
			expectedColor: "#F1C40F",
			expectedType:  MetricTypeResolution,
			shouldMatch:   true,
		},
		{
			name: "Resolution time metric - full_resolution_time",
			metric: SLAPolicyMetric{
				BreachAt: now.Add(1*time.Hour + 30*time.Minute),
				Stage:    "active",
				Metric:   "full_resolution_time",
			},
			expectedLabel: "Less than 2 hours remaining",
			expectedColor: "#F1C40F",
			expectedType:  MetricTypeResolution,
			shouldMatch:   true,
		},
		{
			name: "Unknown metric type",
			metric: SLAPolicyMetric{
				BreachAt: now.Add(1 * time.Hour),
				Stage:    "active",
				Metric:   "unknown_metric",
			},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, color, metricType, matches := SlaConditionMatches(tt.metric)

			assert.Equal(t, tt.shouldMatch, matches)
			if matches {
				assert.Equal(t, tt.expectedLabel, label)
				assert.Equal(t, tt.expectedColor, color)
				assert.Equal(t, tt.expectedType, metricType)
			}
		})
	}
}

func TestSlaConditionMatches_TimeThresholds(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		breachAt      time.Time
		expectedLabel string
		expectedColor string
		shouldMatch   bool
	}{
		{
			name:        "More than 3 hours - no alert",
			breachAt:    now.Add(4 * time.Hour),
			shouldMatch: false,
		},
		{
			name:          "Less than 3 hours",
			breachAt:      now.Add(2*time.Hour + 30*time.Minute),
			expectedLabel: "Less than 3 hours remaining",
			expectedColor: "#3498DB",
			shouldMatch:   true,
		},
		{
			name:          "Less than 2 hours",
			breachAt:      now.Add(1*time.Hour + 30*time.Minute),
			expectedLabel: "Less than 2 hours remaining",
			expectedColor: "#F1C40F",
			shouldMatch:   true,
		},
		{
			name:          "Less than 1 hour",
			breachAt:      now.Add(45 * time.Minute),
			expectedLabel: "Less than 1 hour remaining",
			expectedColor: "#FFA500",
			shouldMatch:   true,
		},
		{
			name:          "Less than 30 minutes",
			breachAt:      now.Add(20 * time.Minute),
			expectedLabel: "Less than 30 minutes remaining",
			expectedColor: "#FF8C00",
			shouldMatch:   true,
		},
		{
			name:          "Less than 15 minutes",
			breachAt:      now.Add(10 * time.Minute),
			expectedLabel: "Less than 15 minutes remaining",
			expectedColor: "#FF0000",
			shouldMatch:   true,
		},
		{
			name:          "SLA Breached",
			breachAt:      now.Add(-5 * time.Minute),
			expectedLabel: "BREACHED",
			expectedColor: "#FF0000",
			shouldMatch:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := SLAPolicyMetric{
				BreachAt: tt.breachAt,
				Stage:    "active",
				Metric:   "reply_time",
			}

			label, color, _, matches := SlaConditionMatches(metric)

			assert.Equal(t, tt.shouldMatch, matches)
			if matches {
				assert.Equal(t, tt.expectedLabel, label)
				assert.Equal(t, tt.expectedColor, color)
			}
		})
	}
}

func TestSlaConditionMatches_InactiveStage(t *testing.T) {
	now := time.Now()

	metric := SLAPolicyMetric{
		BreachAt: now.Add(1 * time.Hour),
		Stage:    "inactive",
		Metric:   "reply_time",
	}

	_, _, _, matches := SlaConditionMatches(metric)
	assert.False(t, matches, "Inactive metrics should not match")
}

func TestSlaConditionMatches_CompletedStage(t *testing.T) {
	now := time.Now()

	metric := SLAPolicyMetric{
		BreachAt: now.Add(1 * time.Hour),
		Stage:    "completed",
		Metric:   "reply_time",
	}

	_, _, _, matches := SlaConditionMatches(metric)
	assert.False(t, matches, "Completed metrics should not match")
}

func TestNewPollingState(t *testing.T) {
	before := time.Now().Add(-5 * time.Minute)
	state := NewPollingState()
	after := time.Now().Add(-5 * time.Minute)

	// The lastPollTime should be approximately 5 minutes before now
	assert.True(t, state.lastPollTime.After(before.Add(-time.Second)) || state.lastPollTime.Equal(before.Add(-time.Second)))
	assert.True(t, state.lastPollTime.Before(after.Add(time.Second)) || state.lastPollTime.Equal(after.Add(time.Second)))
}

func TestAlertTypeConstants(t *testing.T) {
	// Verify alert type constants are defined correctly
	assert.Equal(t, "new_ticket", AlertTypeNewTicket)
	assert.Equal(t, "ticket_update", AlertTypeTicketUpdate)
	assert.Equal(t, "sla_deadline", AlertTypeSLABreach)
	assert.Equal(t, "sla_reply", AlertTypeSLAReply)
	assert.Equal(t, "sla_resolution", AlertTypeSLAResolution)
}

func TestMetricTypeConstants(t *testing.T) {
	// Verify metric type constants are defined correctly
	assert.Equal(t, "reply_time", MetricTypeReply)
	assert.Equal(t, "resolution_time", MetricTypeResolution)
}

func TestGetSLALabel_WithSLAData(t *testing.T) {
	slaData := map[int64]SLAInfo{
		100: {PolicyMetrics: []SLAPolicyMetric{
			{Metric: "reply_time", Hours: 2, Minutes: 30},
		}},
	}
	ticket := zendesk.Ticket{}
	ticket.ID = 100

	label := GetSLALabel(ticket, slaData)
	assert.Equal(t, "reply_time - 2 hours 30 minutes remaining", label)
}

func TestGetSLALabel_NoSLAData(t *testing.T) {
	slaData := map[int64]SLAInfo{}
	ticket := zendesk.Ticket{}
	ticket.ID = 999

	label := GetSLALabel(ticket, slaData)
	assert.Equal(t, "No SLA", label)
}

func TestGetSLALabel_EmptyMetrics(t *testing.T) {
	slaData := map[int64]SLAInfo{
		100: {PolicyMetrics: []SLAPolicyMetric{}},
	}
	ticket := zendesk.Ticket{}
	ticket.ID = 100

	label := GetSLALabel(ticket, slaData)
	assert.Equal(t, "No SLA", label)
}

func TestAssignTicket_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.URL.Path, "/tickets/100.json")

		var body struct {
			Ticket zendesk.Ticket `json:"ticket"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, int64(42), body.Ticket.AssigneeID)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ticket": map[string]interface{}{
				"id":          100,
				"assignee_id": 42,
			},
		})
	}))
	defer server.Close()

	client := createTestZendeskClientWithLibrary(server)
	err := client.AssignTicket(100, 42)
	assert.NoError(t, err)
}

func TestAssignTicket_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := createTestZendeskClientWithLibrary(server)
	err := client.AssignTicket(100, 42)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to assign ticket")
}

func TestAddInternalNote_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.URL.Path, "/tickets/200.json")

		var body struct {
			Ticket struct {
				Comment zendesk.TicketComment `json:"comment"`
			} `json:"ticket"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Test note body", body.Ticket.Comment.Body)
		assert.NotNil(t, body.Ticket.Comment.Public)
		assert.False(t, *body.Ticket.Comment.Public)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	client := createTestZendeskClientWithLibrary(server)
	err := client.AddInternalNote(200, "Test note body")
	assert.NoError(t, err)
}

func TestAddInternalNote_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := createTestZendeskClientWithLibrary(server)
	err := client.AddInternalNote(200, "Test note")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to add internal note")
}

// createTestZendeskClientWithLibrary creates a ZendeskClient with a real
// go-zendesk Client that points at the given test server, for testing methods
// that use the library client (AssignTicket, AddInternalNote, etc.).
func createTestZendeskClientWithLibrary(server *httptest.Server) *ZendeskClient {
	zdClient, _ := zendesk.NewClient(server.Client())
	zdClient.SetEndpointURL(server.URL)
	zdClient.SetCredential(zendesk.NewAPITokenCredential("test@example.com", "test-token"))

	return &ZendeskClient{
		client:            zdClient,
		httpClient:        server.Client(),
		Subdomain:         "test",
		Email:             "test@example.com",
		APIToken:          "test-token",
		requesterCache:    make(map[int64]*User),
		organizationCache: make(map[int64]*Organization),
	}
}
