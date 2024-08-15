package handlers

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/TylerConlee/TicketPulse/models"

	"github.com/gorilla/mux"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)

	// Handle adding a new tag alert
	if r.Method == "POST" && r.URL.Path == "/profile/add-tag" {
		tag := r.FormValue("tag")
		slackChannelID := r.FormValue("slack_channel")
		alertType := r.FormValue("alert_type")

		err := models.CreateTagAlert(userID, tag, slackChannelID, alertType)
		if err != nil {
			http.Error(w, "Unable to add tag alert", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Handle deleting a tag alert
	if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/profile/delete-tag/") {
		alertID, _ := strconv.Atoi(mux.Vars(r)["id"])

		err := models.DeleteTagAlert(alertID)
		if err != nil {
			http.Error(w, "Unable to delete tag alert", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Retrieve the user's tag alerts
	tagAlerts, _ := models.GetTagAlertsByUser(userID)

	// Get available Slack channels (we'll mock this for now)
	slackChannels := []struct {
		ID   string
		Name string
	}{
		{"C01", "#general"},
		{"C02", "#alerts"},
	}
	data, err := getCommonData(r, "Profile")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}
	data["SlackChannels"] = slackChannels
	data["TagAlerts"] = tagAlerts

	t := template.Must(template.ParseFiles("templates/layout.html", "templates/profile.html"))
	t.ExecuteTemplate(w, "layout.html", data)
}
