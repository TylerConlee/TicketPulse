package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/services"
	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/stretchr/testify/assert"
)

// MockDB for mocking the database connection
type MockDB struct {
	db.Database
	sqlmock.Sqlmock
}

func NewMockDB(t *testing.T) *MockDB {
	_, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}
	return &MockDB{Sqlmock: mock, Database: db.NewDatabase()}
}

// TestNewZendeskClient tests the NewZendeskClient function
func TestNewZendeskClient(t *testing.T) {
	mockDB := NewMockDB(t)

	// Mocking configuration retrieval
	mockDB.ExpectQuery("^SELECT value FROM configuration WHERE key = (.+)$").
		WithArgs("zendesk_subdomain").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("test_subdomain"))
	mockDB.ExpectQuery("^SELECT value FROM configuration WHERE key = (.+)$").
		WithArgs("zendesk_email").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("test_email"))
	mockDB.ExpectQuery("^SELECT value FROM configuration WHERE key = (.+)$").
		WithArgs("zendesk_api_key").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("test_api_key"))

	client, err := services.NewZendeskClient(mockDB)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "test_subdomain", client.Subdomain)
	assert.Equal(t, "test_email", client.Email)
	assert.Equal(t, "test_api_key", client.APIToken)

	if err := mockDB.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// TestGetRequesterByID tests the GetRequesterByID function
func TestGetRequesterByID(t *testing.T) {
	mockDB := NewMockDB(t)
	client, err := services.NewZendeskClient(mockDB)
	assert.NoError(t, err)

	// Mocking the HTTP request
	userID := int64(12345)
	expectedUser := services.User{
		ID:   userID,
		Name: "Test User",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/api/v2/users/%d.json", userID), r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": expectedUser,
		})
	}))
	defer server.Close()

	user, err := client.GetRequesterByID(userID)
	assert.NoError(t, err)
	assert.Equal(t, &expectedUser, user)
}

// TestGetOrganizationByID tests the GetOrganizationByID function
func TestGetOrganizationByID(t *testing.T) {
	mockDB := NewMockDB(t)
	client, err := services.NewZendeskClient(mockDB)
	assert.NoError(t, err)

	// Mocking the HTTP request
	organizationID := int64(54321)
	expectedOrganization := services.Organization{
		ID:   organizationID,
		Name: "Test Organization",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/api/v2/organizations/%d.json", organizationID), r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"organization": expectedOrganization,
		})
	}))
	defer server.Close()

	org, err := client.GetOrganizationByID(organizationID)
	assert.NoError(t, err)
	assert.Equal(t, &expectedOrganization, org)
}

// TestStartZendeskPolling tests the StartZendeskPolling function
func TestStartZendeskPolling(t *testing.T) {
	mockDB := NewMockDB(t)
	sseServer := middlewares.NewSSEServer()
	slackService := &services.SlackService{}
	ctx, cancel := context.WithCancel(context.Background())

	// Mock NewZendeskClient to return a mock ZendeskClient
	zendeskClient := &services.ZendeskClient{
		DB: mockDB,
	}
	zendeskClient.SetClient(&zendesk.Client{}) // Use a mock or empty client as needed
	NewZendeskClient = func(db db.Database) (*services.ZendeskClient, error) {
		return zendeskClient, nil
	}

	go func() {
		services.StartZendeskPolling(ctx, mockDB, sseServer, slackService)
	}()

	// Simulate time passage and then cancel the context to stop polling
	time.Sleep(2 * time.Second)
	cancel()

	if err := mockDB.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// TestProcessTickets tests the processTickets function
func TestProcessTickets(t *testing.T) {
	mockDB := NewMockDB(t)
	ctx := context.Background()
	sseServer := middlewares.NewSSEServer()
	slackService := &services.SlackService{}
	tickets := []zendesk.Ticket{
		{ID: 1, Subject: "Test Ticket 1", Tags: []string{"urgent"}},
		{ID: 2, Subject: "Test Ticket 2", Tags: []string{"support"}},
	}
	slaData := map[int64]services.SLAInfo{
		1: {
			PolicyMetrics: []services.SLAPolicyMetric{
				{BreachAt: time.Now().Add(10 * time.Minute), Stage: "active", Metric: "response_time"},
			},
		},
	}

	mockDB.ExpectQuery("^SELECT (.+) FROM user_tag_alerts$").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "tag", "slack_channel_id", "alert_type"}).
			AddRow(1, 1, "urgent", "C123456", services.AlertTypeNewTicket))

	mockDB.ExpectQuery("^SELECT (.+) FROM sla_alert_cache WHERE user_id = (.+) AND ticket_id = (.+) AND alert_type = (.+)$").
		WithArgs(1, 1, services.AlertTypeSLABreach).
		WillReturnError(errors.New("no rows found"))

	ProcessTickets(ctx, mockDB, tickets, slaData, sseServer, slackService)

	if err := mockDB.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
