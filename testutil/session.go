// Package testutil provides utilities for testing TicketPulse components.
package testutil

import (
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/sessions"
)

// SessionStore is a test session store for handler tests.
var SessionStore = sessions.NewCookieStore([]byte("test-session-key"))

// CreateAuthenticatedRequest creates an HTTP request with session authentication.
func CreateAuthenticatedRequest(method, path string, userID int, email string, role string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	session, _ := SessionStore.Get(req, "session-name")
	session.Values["user_id"] = userID
	session.Values["user_email"] = email
	session.Values["role"] = role
	return req
}

// CreateUnauthenticatedRequest creates an HTTP request without session authentication.
func CreateUnauthenticatedRequest(method, path string) *http.Request {
	return httptest.NewRequest(method, path, nil)
}

// SaveSessionToResponse saves the session to a response so cookies are set.
func SaveSessionToResponse(req *http.Request, w http.ResponseWriter, userID int, email string, role string) error {
	session, _ := SessionStore.Get(req, "session-name")
	session.Values["user_id"] = userID
	session.Values["user_email"] = email
	session.Values["role"] = role
	return session.Save(req, w)
}
