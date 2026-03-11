package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"
	"github.com/gorilla/mux"
)

type channelGroup struct {
	ChannelID   string
	ChannelName string
	Alerts      []models.TagAlert
}

// AlertsPageHandler renders the main alerts management page.
func (h *AppHandler) AlertsPageHandler(w http.ResponseWriter, r *http.Request, slackService *services.SlackService) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tagAlerts, err := models.GetTagAlertsByUser(h.DB, userID)
	if err != nil {
		http.Error(w, "Unable to retrieve tag alerts", http.StatusInternalServerError)
		return
	}

	channels := buildChannelList(slackService)
	groups := groupAlertsByChannel(tagAlerts)

	data, err := h.getCommonData(r, "Alerts")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}
	data["ChannelGroups"] = groups
	data["SlackChannels"] = channels
	data["TagAlerts"] = tagAlerts
	data["AlertCount"] = len(tagAlerts)

	t := template.Must(template.ParseFiles("templates/layout.html", "templates/alerts/index.html"))
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("Error rendering alerts template: %v", err)
		http.Error(w, "Unable to render template", http.StatusInternalServerError)
	}
}

// AlertsAddHandler handles adding new tag alerts (supports multi alert-type).
func (h *AppHandler) AlertsAddHandler(w http.ResponseWriter, r *http.Request, slackService *services.SlackService) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	tag := strings.TrimSpace(r.FormValue("tag"))
	slackChannelID := r.FormValue("slack_channel")
	alertTypes := r.Form["alert_type"]

	if tag == "" || slackChannelID == "" || len(alertTypes) == 0 {
		http.Error(w, "Tag, channel, and at least one alert type are required", http.StatusBadRequest)
		return
	}

	channelName := resolveChannelName(slackService, slackChannelID)

	for _, alertType := range alertTypes {
		if err := models.CreateTagAlert(h.DB, userID, tag, slackChannelID, channelName, alertType); err != nil {
			log.Printf("Error creating tag alert: %v", err)
			http.Error(w, "Unable to add tag alert", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
}

// AlertsUpdateHandler handles inline editing of an existing tag alert.
func (h *AppHandler) AlertsUpdateHandler(w http.ResponseWriter, r *http.Request, slackService *services.SlackService) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	alertID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	alert, err := models.GetTagAlertByID(h.DB, alertID)
	if err != nil {
		http.Error(w, "Tag alert not found", http.StatusNotFound)
		return
	}
	if alert.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tag := strings.TrimSpace(r.FormValue("tag"))
	slackChannelID := r.FormValue("slack_channel")
	alertType := r.FormValue("alert_type")

	if tag == "" || slackChannelID == "" || alertType == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	channelName := resolveChannelName(slackService, slackChannelID)

	if err := models.UpdateTagAlert(h.DB, alertID, tag, slackChannelID, channelName, alertType); err != nil {
		http.Error(w, "Unable to update tag alert", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
}

// AlertsDeleteHandler handles deleting a single tag alert.
func (h *AppHandler) AlertsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	alertID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	alert, err := models.GetTagAlertByID(h.DB, alertID)
	if err != nil {
		http.Error(w, "Tag alert not found", http.StatusNotFound)
		return
	}
	if alert.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := models.DeleteTagAlert(h.DB, alertID); err != nil {
		http.Error(w, "Unable to delete tag alert", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
}

// AlertsBulkDeleteHandler handles deleting multiple tag alerts at once.
func (h *AppHandler) AlertsBulkDeleteHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	idStrings := r.Form["alert_ids"]
	var ids []int
	for _, idStr := range idStrings {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		http.Redirect(w, r, "/alerts", http.StatusSeeOther)
		return
	}

	if err := models.DeleteTagAlertsByIDs(h.DB, userID, ids); err != nil {
		http.Error(w, "Unable to delete tag alerts", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
}

// AlertsTagsJSONHandler returns cached Zendesk tags as JSON for autocomplete.
func (h *AppHandler) AlertsTagsJSONHandler(w http.ResponseWriter, r *http.Request) {
	tags, err := models.GetCachedTags(h.DB)
	if err != nil {
		tags = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}

// AlertsHistoryHandler renders the alert history page with stats and paginated log.
func (h *AppHandler) AlertsHistoryHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 25
	offset := (page - 1) * perPage

	stats, _ := models.GetAlertHistoryStats(h.DB, userID)
	logs, total, err := models.GetAlertHistoryPaginated(h.DB, userID, perPage, offset)
	if err != nil {
		http.Error(w, "Unable to retrieve alert history", http.StatusInternalServerError)
		return
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	data, err := h.getCommonData(r, "Alert History")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}
	data["Stats"] = stats
	data["Logs"] = logs
	data["Page"] = page
	data["TotalPages"] = totalPages
	data["Total"] = total
	data["HasPrev"] = page > 1
	data["HasNext"] = page < totalPages
	data["PrevPage"] = page - 1
	data["NextPage"] = page + 1

	t := template.Must(template.ParseFiles("templates/layout.html", "templates/alerts/history.html"))
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("Error rendering alert history template: %v", err)
		http.Error(w, "Unable to render template", http.StatusInternalServerError)
	}
}

func buildChannelList(slackService *services.SlackService) []struct {
	ID   string
	Name string
} {
	var channels []struct {
		ID   string
		Name string
	}
	if slackService == nil || !slackService.IsReady() {
		return channels
	}
	slackChannels, err := slackService.GetConversations()
	if err != nil {
		return channels
	}
	for _, ch := range slackChannels {
		channels = append(channels, struct {
			ID   string
			Name string
		}{ID: ch.ID, Name: ch.Name})
	}
	sort.Slice(channels, func(i, j int) bool {
		return strings.ToLower(channels[i].Name) < strings.ToLower(channels[j].Name)
	})
	return channels
}

func resolveChannelName(slackService *services.SlackService, channelID string) string {
	if slackService == nil || !slackService.IsReady() {
		return ""
	}
	channels, err := slackService.GetConversations()
	if err != nil {
		return ""
	}
	for _, ch := range channels {
		if ch.ID == channelID {
			return ch.Name
		}
	}
	return ""
}

func groupAlertsByChannel(alerts []models.TagAlert) []channelGroup {
	groupMap := make(map[string]*channelGroup)
	var order []string

	for _, alert := range alerts {
		if _, exists := groupMap[alert.SlackChannelID]; !exists {
			name := alert.SlackChannelName
			if name == "" {
				name = alert.SlackChannelID
			}
			groupMap[alert.SlackChannelID] = &channelGroup{
				ChannelID:   alert.SlackChannelID,
				ChannelName: name,
			}
			order = append(order, alert.SlackChannelID)
		}
		g := groupMap[alert.SlackChannelID]
		g.Alerts = append(g.Alerts, alert)
	}

	var groups []channelGroup
	for _, chID := range order {
		groups = append(groups, *groupMap[chID])
	}
	return groups
}
