package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/services/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHandlersTestDB(t *testing.T) *db.SQLDatabase {
	database := db.InitDB(":memory:")
	require.NotNil(t, database, "Failed to initialize test database")
	return database
}

func TestNewAppHandler(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()

	handler := NewAppHandler(database)

	assert.NotNil(t, handler)
	assert.Equal(t, database, handler.DB)
}

func TestNewAppHandlerWithServices(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()

	mockSlack := new(mocks.MockSlackService)
	mockZendesk := new(mocks.MockZendeskClient)

	handler := NewAppHandlerWithServices(database, mockSlack, mockZendesk)

	assert.NotNil(t, handler)
	assert.Equal(t, database, handler.DB)
	assert.Equal(t, mockSlack, handler.SlackService)
	assert.Equal(t, mockZendesk, handler.Zendesk)
}

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name           string
		contextSetup   func() context.Context
		expectedUserID int
		expectedOk     bool
	}{
		{
			name: "Valid user ID in context",
			contextSetup: func() context.Context {
				return context.WithValue(context.Background(), userIDKey, 123)
			},
			expectedUserID: 123,
			expectedOk:     true,
		},
		{
			name: "No user ID in context",
			contextSetup: func() context.Context {
				return context.Background()
			},
			expectedUserID: 0,
			expectedOk:     false,
		},
		{
			name: "Wrong type in context",
			contextSetup: func() context.Context {
				return context.WithValue(context.Background(), userIDKey, "not-an-int")
			},
			expectedUserID: 0,
			expectedOk:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.contextSetup()
			userID, ok := GetUserIDFromContext(ctx)

			assert.Equal(t, tt.expectedOk, ok)
			if ok {
				assert.Equal(t, tt.expectedUserID, userID)
			}
		})
	}
}

// Note: LogoutHandler and other session-dependent tests require
// proper environment setup with SESSION_KEY. These are better tested
// in integration tests with the full environment.

func TestAuthMiddleware_NoSession(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()

	handler := NewAppHandler(database)

	// Create a handler that should be protected
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("protected content"))
	})

	// Wrap with auth middleware
	wrappedHandler := handler.AuthMiddleware(protectedHandler)

	// Request without session
	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Should redirect to login
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestPackageLevelAuthMiddleware_NoSession(t *testing.T) {
	// Test the package-level AuthMiddleware function
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("protected content"))
	})

	// Wrap with package-level auth middleware
	wrappedHandler := AuthMiddleware(protectedHandler)

	// Request without session
	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Should redirect to login
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestPackageLevelAdminMiddleware_NoSession(t *testing.T) {
	// Test the package-level AdminMiddleware function
	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("admin content"))
	})

	// Wrap with package-level admin middleware
	wrappedHandler := AdminMiddleware(adminHandler)

	// Request without session
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Should redirect to login (no role in session)
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestGetUserIDFromContextHelper(t *testing.T) {
	// Test that context works correctly
	ctx := context.WithValue(context.Background(), userIDKey, 42)
	userID, ok := GetUserIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, 42, userID)
}

func TestContextKeyNotSet(t *testing.T) {
	// Test when context doesn't have user ID
	ctx := context.Background()
	userID, ok := GetUserIDFromContext(ctx)
	assert.False(t, ok)
	assert.Equal(t, 0, userID)
}
