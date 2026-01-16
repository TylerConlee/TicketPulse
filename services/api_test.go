package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestZendeskClient creates a ZendeskClient for testing with a custom HTTP client.
func createTestZendeskClient(server *httptest.Server) *ZendeskClient {
	// Create a custom HTTP client that redirects requests to the test server
	customClient := server.Client()

	return &ZendeskClient{
		httpClient: customClient,
		Subdomain:  "test",
		Email:      "test@example.com",
		APIToken:   "test-token",
	}
}

func TestGetRequesterByID(t *testing.T) {
	tests := []struct {
		name           string
		userID         int64
		serverResponse interface{}
		statusCode     int
		expectError    bool
		expectedName   string
	}{
		{
			name:   "Successful user retrieval",
			userID: 12345,
			serverResponse: map[string]interface{}{
				"user": map[string]interface{}{
					"id":   12345,
					"name": "Test User",
					"role": "end-user",
				},
			},
			statusCode:   http.StatusOK,
			expectError:  false,
			expectedName: "Test User",
		},
		{
			name:   "User with admin role",
			userID: 67890,
			serverResponse: map[string]interface{}{
				"user": map[string]interface{}{
					"id":   67890,
					"name": "Admin User",
					"role": "admin",
				},
			},
			statusCode:   http.StatusOK,
			expectError:  false,
			expectedName: "Admin User",
		},
		{
			name:           "User not found",
			userID:         99999,
			serverResponse: nil,
			statusCode:     http.StatusNotFound,
			expectError:    true,
		},
		{
			name:           "Server error",
			userID:         11111,
			serverResponse: nil,
			statusCode:     http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request path contains users endpoint
				assert.Contains(t, r.URL.Path, fmt.Sprintf("/users/%d", tt.userID))

				// Verify basic auth is set
				username, password, ok := r.BasicAuth()
				assert.True(t, ok)
				assert.Equal(t, "test@example.com/token", username)
				assert.Equal(t, "test-token", password)

				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != nil {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Create a client that redirects to our test server
			client := createTestZendeskClientWithBaseURL(server)

			user, err := client.GetRequesterByID(tt.userID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedName, user.Name)
			}
		})
	}
}

func TestGetOrganizationByID(t *testing.T) {
	tests := []struct {
		name           string
		orgID          int64
		serverResponse interface{}
		statusCode     int
		expectError    bool
		expectedName   string
	}{
		{
			name:  "Successful organization retrieval",
			orgID: 12345,
			serverResponse: map[string]interface{}{
				"organization": map[string]interface{}{
					"id":   12345,
					"name": "Test Organization",
				},
			},
			statusCode:   http.StatusOK,
			expectError:  false,
			expectedName: "Test Organization",
		},
		{
			name:           "Organization not found",
			orgID:          99999,
			serverResponse: nil,
			statusCode:     http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, fmt.Sprintf("/organizations/%d", tt.orgID))

				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != nil {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			client := createTestZendeskClientWithBaseURL(server)
			org, err := client.GetOrganizationByID(tt.orgID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedName, org.Name)
			}
		})
	}
}

func TestGetUserByEmail(t *testing.T) {
	tests := []struct {
		name           string
		email          string
		serverResponse interface{}
		statusCode     int
		expectError    bool
		expectedUserID int64
	}{
		{
			name:  "Successful user lookup",
			email: "user@example.com",
			serverResponse: map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": int64(12345), "name": "Test User", "email": "user@example.com"},
				},
			},
			statusCode:     http.StatusOK,
			expectError:    false,
			expectedUserID: 12345,
		},
		{
			name:  "User not found - empty results",
			email: "nonexistent@example.com",
			serverResponse: map[string]interface{}{
				"results": []map[string]interface{}{},
			},
			statusCode:  http.StatusOK,
			expectError: true,
		},
		{
			name:           "Server error",
			email:          "error@example.com",
			serverResponse: nil,
			statusCode:     http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, "/search")
				assert.Contains(t, r.URL.RawQuery, "email")

				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != nil {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			client := createTestZendeskClientWithBaseURL(server)
			user, err := client.GetUserByEmail(tt.email)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUserID, user.ID)
			}
		})
	}
}

func TestGetCSATRatingsForUser(t *testing.T) {
	tests := []struct {
		name            string
		userID          int64
		serverResponse  interface{}
		statusCode      int
		expectError     bool
		expectedRatings int
	}{
		{
			name:   "Successful CSAT ratings retrieval",
			userID: 12345,
			serverResponse: map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": 1, "score": "good", "ticket_id": 100},
					{"id": 2, "score": "bad", "ticket_id": 101},
				},
			},
			statusCode:      http.StatusOK,
			expectError:     false,
			expectedRatings: 2,
		},
		{
			name:   "No ratings found",
			userID: 99999,
			serverResponse: map[string]interface{}{
				"results": []map[string]interface{}{},
			},
			statusCode:      http.StatusOK,
			expectError:     false,
			expectedRatings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, "/search")

				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != nil {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			client := createTestZendeskClientWithBaseURL(server)
			ratings, err := client.GetCSATRatingsForUser(tt.userID, time.Now().Add(-24*time.Hour))

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, ratings, tt.expectedRatings)
			}
		})
	}
}

func TestNewZendeskClientForTesting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":   12345,
				"name": "Test User",
			},
		})
	}))
	defer server.Close()

	client := NewZendeskClientForTesting("test", "test@example.com", "token", server.Client())

	assert.NotNil(t, client)
	assert.Equal(t, "test", client.Subdomain)
	assert.Equal(t, "test@example.com", client.Email)
	assert.Equal(t, "token", client.APIToken)
	assert.NotNil(t, client.httpClient)
}

// createTestZendeskClientWithBaseURL creates a client that uses the test server.
// Since we can't easily override the base URL in the ZendeskClient, we use a
// custom transport that rewrites the URL to point to our test server.
func createTestZendeskClientWithBaseURL(server *httptest.Server) *ZendeskClient {
	// Create a custom transport that rewrites URLs to the test server
	transport := &testTransport{
		server:    server,
		transport: http.DefaultTransport,
	}

	return &ZendeskClient{
		httpClient: &http.Client{Transport: transport},
		Subdomain:  "test",
		Email:      "test@example.com",
		APIToken:   "test-token",
	}
}

// testTransport rewrites requests to point to the test server
type testTransport struct {
	server    *httptest.Server
	transport http.RoundTripper
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point to our test server, but keep the path
	newURL := t.server.URL + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}

	newReq, err := http.NewRequest(req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header

	return t.transport.RoundTrip(newReq)
}
