// Package testutil provides utilities for testing TicketPulse components.
package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
)

// MockResponse represents a mock API response.
type MockResponse struct {
	StatusCode int
	Body       interface{}
}

// NewMockZendeskServer creates a mock HTTP server that responds to Zendesk API requests.
// The responses map keys are path fragments to match, values are the response data.
func NewMockZendeskServer(responses map[string]MockResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for pathFragment, response := range responses {
			if strings.Contains(r.URL.Path, pathFragment) {
				w.WriteHeader(response.StatusCode)
				if response.Body != nil {
					json.NewEncoder(w).Encode(response.Body)
				}
				return
			}
		}
		// Default: not found
		http.Error(w, "Not found", http.StatusNotFound)
	}))
}

// NewMockZendeskServerSimple creates a mock server with 200 OK responses.
// The responses map keys are path fragments, values are the response bodies.
func NewMockZendeskServerSimple(responses map[string]interface{}) *httptest.Server {
	mockResponses := make(map[string]MockResponse)
	for path, body := range responses {
		mockResponses[path] = MockResponse{StatusCode: http.StatusOK, Body: body}
	}
	return NewMockZendeskServer(mockResponses)
}

// NewMockZendeskServerWithHandler creates a mock server with a custom handler function.
func NewMockZendeskServerWithHandler(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}
