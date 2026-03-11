package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createAuthenticatedRequest builds a request with a valid session cookie.
func createAuthenticatedRequest(t *testing.T, method, path string, userID int, email, role string) *http.Request {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = userID
	session.Values["user_email"] = email
	session.Values["role"] = role
	require.NoError(t, session.Save(req, rec))

	authedReq := httptest.NewRequest(method, path, nil)
	for _, cookie := range rec.Result().Cookies() {
		authedReq.AddCookie(cookie)
	}
	return authedReq
}

// createAuthenticatedPostRequest builds a POST request with form data and a valid session.
func createAuthenticatedPostRequest(t *testing.T, path string, userID int, email, role string, formData url.Values) *http.Request {
	t.Helper()

	rec := httptest.NewRecorder()
	tmpReq := httptest.NewRequest("POST", path, nil)
	session, _ := store.Get(tmpReq, "session-name")
	session.Values["user_id"] = userID
	session.Values["user_email"] = email
	session.Values["role"] = role
	require.NoError(t, session.Save(tmpReq, rec))

	req := httptest.NewRequest("POST", path, strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	return req
}

// --- groupAlertsByChannel Tests ---

func TestGroupAlertsByChannel_Empty(t *testing.T) {
	groups := groupAlertsByChannel(nil)
	assert.Empty(t, groups)
}

func TestGroupAlertsByChannel_SingleChannel(t *testing.T) {
	alerts := []models.TagAlert{
		{ID: 1, SlackChannelID: "C111", SlackChannelName: "general", Tag: "urgent"},
		{ID: 2, SlackChannelID: "C111", SlackChannelName: "general", Tag: "billing"},
	}

	groups := groupAlertsByChannel(alerts)

	assert.Len(t, groups, 1)
	assert.Equal(t, "C111", groups[0].ChannelID)
	assert.Equal(t, "general", groups[0].ChannelName)
	assert.Len(t, groups[0].Alerts, 2)
}

func TestGroupAlertsByChannel_MultipleChannels(t *testing.T) {
	alerts := []models.TagAlert{
		{ID: 1, SlackChannelID: "C111", SlackChannelName: "general", Tag: "urgent"},
		{ID: 2, SlackChannelID: "C222", SlackChannelName: "billing-team", Tag: "billing"},
		{ID: 3, SlackChannelID: "C111", SlackChannelName: "general", Tag: "support"},
	}

	groups := groupAlertsByChannel(alerts)

	assert.Len(t, groups, 2)
	assert.Equal(t, "general", groups[0].ChannelName)
	assert.Len(t, groups[0].Alerts, 2)
	assert.Equal(t, "billing-team", groups[1].ChannelName)
	assert.Len(t, groups[1].Alerts, 1)
}

func TestGroupAlertsByChannel_PreservesInsertionOrder(t *testing.T) {
	alerts := []models.TagAlert{
		{ID: 1, SlackChannelID: "C333", SlackChannelName: "gamma"},
		{ID: 2, SlackChannelID: "C111", SlackChannelName: "alpha"},
		{ID: 3, SlackChannelID: "C222", SlackChannelName: "beta"},
	}

	groups := groupAlertsByChannel(alerts)

	assert.Len(t, groups, 3)
	assert.Equal(t, "C333", groups[0].ChannelID)
	assert.Equal(t, "C111", groups[1].ChannelID)
	assert.Equal(t, "C222", groups[2].ChannelID)
}

func TestGroupAlertsByChannel_UnknownChannelFallsBackToID(t *testing.T) {
	alerts := []models.TagAlert{
		{ID: 1, SlackChannelID: "C999"},
	}

	groups := groupAlertsByChannel(alerts)

	assert.Len(t, groups, 1)
	assert.Equal(t, "C999", groups[0].ChannelName)
}

// --- buildChannelList Tests ---

func TestBuildChannelList_NilService(t *testing.T) {
	result := buildChannelList(nil)
	assert.Empty(t, result)
}

// --- AlertsAddHandler Tests ---

func TestAlertsAddHandler_NoSession(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	req := httptest.NewRequest("POST", "/alerts/add", nil)
	w := httptest.NewRecorder()

	handler.AlertsAddHandler(w, req, nil)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestAlertsAddHandler_MissingFields(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	tests := []struct {
		name string
		form url.Values
	}{
		{"missing tag", url.Values{"slack_channel": {"C111"}, "alert_type": {"new_ticket"}}},
		{"missing channel", url.Values{"tag": {"urgent"}, "alert_type": {"new_ticket"}}},
		{"missing alert_type", url.Values{"tag": {"urgent"}, "slack_channel": {"C111"}}},
		{"all empty", url.Values{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createAuthenticatedPostRequest(t, "/alerts/add", user.ID, user.Email, string(user.Role), tt.form)
			w := httptest.NewRecorder()

			handler.AlertsAddHandler(w, req, nil)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestAlertsAddHandler_Success(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	form := url.Values{
		"tag":           {"urgent"},
		"slack_channel": {"C111"},
		"alert_type":    {"new_ticket", "sla_reply"},
	}
	req := createAuthenticatedPostRequest(t, "/alerts/add", user.ID, user.Email, string(user.Role), form)
	w := httptest.NewRecorder()

	handler.AlertsAddHandler(w, req, nil)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/alerts", w.Header().Get("Location"))

	alerts, err := models.GetTagAlertsByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Len(t, alerts, 2)
}

// --- AlertsUpdateHandler Tests ---

func TestAlertsUpdateHandler_NoSession(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	req := httptest.NewRequest("POST", "/alerts/update/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	handler.AlertsUpdateHandler(w, req, nil)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestAlertsUpdateHandler_InvalidID(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	form := url.Values{"tag": {"t"}, "slack_channel": {"c"}, "alert_type": {"new_ticket"}}
	req := createAuthenticatedPostRequest(t, "/alerts/update/abc", user.ID, user.Email, string(user.Role), form)
	req = mux.SetURLVars(req, map[string]string{"id": "abc"})
	w := httptest.NewRecorder()

	handler.AlertsUpdateHandler(w, req, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAlertsUpdateHandler_NotFound(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	form := url.Values{"tag": {"t"}, "slack_channel": {"c"}, "alert_type": {"new_ticket"}}
	req := createAuthenticatedPostRequest(t, "/alerts/update/99999", user.ID, user.Email, string(user.Role), form)
	req = mux.SetURLVars(req, map[string]string{"id": "99999"})
	w := httptest.NewRecorder()

	handler.AlertsUpdateHandler(w, req, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAlertsUpdateHandler_Forbidden(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "owner@example.com", "Owner", models.AdminRole, false)
	require.NoError(t, err)
	err = models.CreateUser(database, "other@example.com", "Other", models.AgentRole, false)
	require.NoError(t, err)

	owner, err := models.GetUserByEmail(database, "owner@example.com")
	require.NoError(t, err)
	other, err := models.GetUserByEmail(database, "other@example.com")
	require.NoError(t, err)

	err = models.CreateTagAlert(database, owner.ID, "tag", "C111", "general", "new_ticket")
	require.NoError(t, err)
	alerts, err := models.GetTagAlertsByUser(database, owner.ID)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	idStr := fmt.Sprintf("%d", alerts[0].ID)
	form := url.Values{"tag": {"new"}, "slack_channel": {"C222"}, "alert_type": {"sla_reply"}}
	req := createAuthenticatedPostRequest(t, "/alerts/update/"+idStr, other.ID, other.Email, string(other.Role), form)
	req = mux.SetURLVars(req, map[string]string{"id": idStr})
	w := httptest.NewRecorder()

	handler.AlertsUpdateHandler(w, req, nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAlertsUpdateHandler_Success(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = models.CreateTagAlert(database, user.ID, "old_tag", "C111", "general", "new_ticket")
	require.NoError(t, err)
	alerts, err := models.GetTagAlertsByUser(database, user.ID)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	form := url.Values{"tag": {"new_tag"}, "slack_channel": {"C222"}, "alert_type": {"sla_reply"}}
	req := createAuthenticatedPostRequest(t, "/alerts/update/1", user.ID, user.Email, string(user.Role), form)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	handler.AlertsUpdateHandler(w, req, nil)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/alerts", w.Header().Get("Location"))

	updated, err := models.GetTagAlertByID(database, alerts[0].ID)
	assert.NoError(t, err)
	assert.Equal(t, "new_tag", updated.Tag)
	assert.Equal(t, "C222", updated.SlackChannelID)
	assert.Equal(t, "sla_reply", updated.AlertType)
}

// --- AlertsDeleteHandler Tests ---

func TestAlertsDeleteHandler_NoSession(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	req := httptest.NewRequest("POST", "/alerts/delete/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	handler.AlertsDeleteHandler(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestAlertsDeleteHandler_InvalidID(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	req := createAuthenticatedRequest(t, "POST", "/alerts/delete/abc", user.ID, user.Email, string(user.Role))
	req = mux.SetURLVars(req, map[string]string{"id": "abc"})
	w := httptest.NewRecorder()

	handler.AlertsDeleteHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAlertsDeleteHandler_Forbidden(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "owner@example.com", "Owner", models.AdminRole, false)
	require.NoError(t, err)
	err = models.CreateUser(database, "other@example.com", "Other", models.AgentRole, false)
	require.NoError(t, err)

	owner, err := models.GetUserByEmail(database, "owner@example.com")
	require.NoError(t, err)
	other, err := models.GetUserByEmail(database, "other@example.com")
	require.NoError(t, err)

	err = models.CreateTagAlert(database, owner.ID, "tag", "C111", "general", "new_ticket")
	require.NoError(t, err)

	req := createAuthenticatedRequest(t, "POST", "/alerts/delete/1", other.ID, other.Email, string(other.Role))
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	handler.AlertsDeleteHandler(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAlertsDeleteHandler_Success(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = models.CreateTagAlert(database, user.ID, "tag", "C111", "general", "new_ticket")
	require.NoError(t, err)

	alerts, err := models.GetTagAlertsByUser(database, user.ID)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	req := createAuthenticatedRequest(t, "POST", "/alerts/delete/1", user.ID, user.Email, string(user.Role))
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	handler.AlertsDeleteHandler(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/alerts", w.Header().Get("Location"))

	remaining, err := models.GetTagAlertsByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Empty(t, remaining)
}

// --- AlertsBulkDeleteHandler Tests ---

func TestAlertsBulkDeleteHandler_NoSession(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	req := httptest.NewRequest("POST", "/alerts/bulk-delete", nil)
	w := httptest.NewRecorder()

	handler.AlertsBulkDeleteHandler(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestAlertsBulkDeleteHandler_NoIDs(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	form := url.Values{}
	req := createAuthenticatedPostRequest(t, "/alerts/bulk-delete", user.ID, user.Email, string(user.Role), form)
	w := httptest.NewRecorder()

	handler.AlertsBulkDeleteHandler(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
}

func TestAlertsBulkDeleteHandler_Success(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.CreateUser(database, "test@example.com", "Test User", models.AdminRole, false)
	require.NoError(t, err)
	user, err := models.GetUserByEmail(database, "test@example.com")
	require.NoError(t, err)

	err = models.CreateTagAlert(database, user.ID, "tag1", "C111", "general", "new_ticket")
	require.NoError(t, err)
	err = models.CreateTagAlert(database, user.ID, "tag2", "C222", "billing", "sla_reply")
	require.NoError(t, err)
	err = models.CreateTagAlert(database, user.ID, "tag3", "C333", "support", "ticket_update")
	require.NoError(t, err)

	alerts, err := models.GetTagAlertsByUser(database, user.ID)
	require.NoError(t, err)
	require.Len(t, alerts, 3)

	form := url.Values{
		"alert_ids": {
			fmt.Sprintf("%d", alerts[0].ID),
			fmt.Sprintf("%d", alerts[1].ID),
		},
	}
	req := createAuthenticatedPostRequest(t, "/alerts/bulk-delete", user.ID, user.Email, string(user.Role), form)
	w := httptest.NewRecorder()

	handler.AlertsBulkDeleteHandler(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/alerts", w.Header().Get("Location"))

	remaining, err := models.GetTagAlertsByUser(database, user.ID)
	assert.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, "tag3", remaining[0].Tag)
}

// --- AlertsTagsJSONHandler Tests ---

func TestAlertsTagsJSONHandler_EmptyCache(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	req := httptest.NewRequest("GET", "/alerts/tags", nil)
	w := httptest.NewRecorder()

	handler.AlertsTagsJSONHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var tags []string
	err := json.NewDecoder(w.Body).Decode(&tags)
	assert.NoError(t, err)
	assert.Empty(t, tags)
}

func TestAlertsTagsJSONHandler_WithCachedTags(t *testing.T) {
	database := setupHandlersTestDB(t)
	defer database.Close()
	handler := NewAppHandler(database, nil, nil)

	err := models.ClearAndReplaceCachedTags(database, []string{"billing", "urgent", "support"})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/alerts/tags", nil)
	w := httptest.NewRecorder()

	handler.AlertsTagsJSONHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var tags []string
	err = json.NewDecoder(w.Body).Decode(&tags)
	assert.NoError(t, err)
	assert.Equal(t, []string{"billing", "support", "urgent"}, tags)
}
