package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHandlersTestDB(t *testing.T) *db.SQLDatabase {
	database := db.InitDB(":memory:")
	require.NotNil(t, database, "Failed to initialize test database")
	return database
}

func init() {
	sessionKey := []byte("test-session-key-that-is-32-byte")
	csrfKey := []byte("test-csrf-key-that-is-32-bytes!")
	InitAuth(sessionKey, csrfKey, "http://localhost:8080")
}

func TestNewAppHandler(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()

	handler := NewAppHandler(database, nil, nil)

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
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("protected content"))
	})

	wrappedHandler := AuthMiddleware(protectedHandler)

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

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

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{"valid simple email", "user@example.com", true},
		{"valid with subdomain", "user@mail.example.com", true},
		{"valid with plus", "user+tag@example.com", true},
		{"valid with dots", "first.last@example.com", true},
		{"empty string", "", false},
		{"no at sign", "userexample.com", false},
		{"no domain", "user@", false},
		{"no local part", "@example.com", false},
		{"spaces", "user @example.com", false},
		{"long email over 254 chars", string(make([]byte, 250)) + "@a.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogoutHandler(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	rec := httptest.NewRecorder()
	tmpReq := httptest.NewRequest("POST", "/logout", nil)
	session, _ := store.Get(tmpReq, "session-name")
	session.Values["user_id"] = 1
	require.NoError(t, session.Save(tmpReq, rec))

	req := httptest.NewRequest("POST", "/logout", nil)
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()

	handler.LogoutHandler(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestLogoutHandler_RejectsGET(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	req := httptest.NewRequest("GET", "/logout", nil)
	w := httptest.NewRecorder()

	handler.LogoutHandler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestIsHTTPS(t *testing.T) {
	result := IsHTTPS()
	assert.IsType(t, true, result)
}

func TestBaseURL(t *testing.T) {
	result := BaseURL()
	assert.NotEmpty(t, result)
	assert.Equal(t, "http://localhost:8080", result)
}

func TestValidateWorkDaysJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid weekdays", `["Monday","Tuesday","Wednesday","Thursday","Friday"]`, true},
		{"valid single day", `["Monday"]`, true},
		{"valid all days", `["Monday","Tuesday","Wednesday","Thursday","Friday","Saturday","Sunday"]`, true},
		{"invalid day name", `["Monday","Funday"]`, false},
		{"too many days", `["Monday","Tuesday","Wednesday","Thursday","Friday","Saturday","Sunday","Extra"]`, false},
		{"empty array", `[]`, true},
		{"invalid json", `not json`, false},
		{"number in array", `[1, 2, 3]`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, validateWorkDaysJSON(tt.input))
		})
	}
}

func TestGetCurrentUser_NoSession(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	req := httptest.NewRequest("GET", "/", nil)
	user := handler.getCurrentUser(req)

	assert.Equal(t, 0, user.ID)
	assert.Empty(t, user.Email)
}

func TestGetCurrentUser_ValidSession(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	dbUser, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	handler := NewAppHandler(database, nil, nil)

	req := createAuthenticatedRequest(t, "GET", "/", dbUser.ID, dbUser.Email, string(dbUser.Role))
	user := handler.getCurrentUser(req)

	assert.Equal(t, dbUser.ID, user.ID)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"status": "ok"}
	writeJSON(w, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

func TestWriteJSON_Error(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"error": "not found"}
	writeJSON(w, http.StatusNotFound, data)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"not found"`)
}

func TestAuthMiddleware_WithValidSession(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	var capturedUserID int
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := GetUserIDFromContext(r.Context())
		if ok {
			capturedUserID = uid
		}
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := AuthMiddleware(protectedHandler)
	req := createAuthenticatedRequest(t, "GET", "/protected", user.ID, user.Email, string(user.Role))
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, user.ID, capturedUserID)
}

func TestAdminMiddleware_NonAdminRole(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()

	err := models.CreateUser(database, "agent@example.com", "Agent", models.AgentRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "agent@example.com")
	require.NoError(t, err)

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := AdminMiddleware(adminHandler)
	req := createAuthenticatedRequest(t, "GET", "/admin", user.ID, user.Email, string(user.Role))
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminMiddleware_AdminRole(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()

	err := models.CreateUser(database, "admin@example.com", "Admin", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "admin@example.com")
	require.NoError(t, err)

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := AdminMiddleware(adminHandler)
	req := createAuthenticatedRequest(t, "GET", "/admin", user.ID, user.Email, string(user.Role))
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
