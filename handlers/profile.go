package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"

	"github.com/gorilla/mux"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request, slackService *services.SlackService) {
	// Ensure SlackService is not nil
	if slackService == nil {
		http.Error(w, "Slack service is not initialized", http.StatusInternalServerError)
		log.Println("Slack service is nil in ProfileHandler")
		return
	}

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
			log.Printf("Error adding tag alert: %v", err)
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
			log.Printf("Error deleting tag alert: %v", err)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Retrieve the user's tag alerts
	tagAlerts, err := models.GetTagAlertsByUser(userID)
	if err != nil {
		http.Error(w, "Unable to retrieve tag alerts", http.StatusInternalServerError)
		log.Printf("Error retrieving tag alerts: %v", err)
		return
	}

	// Fetch available Slack channels
	channels, err := slackService.GetConversations()
	if err != nil {
		http.Error(w, "Unable to fetch Slack channels", http.StatusInternalServerError)
		log.Printf("Error fetching Slack channels: %v", err)
		return
	}

	// Convert Slack channels to a simple struct for template usage
	slackChannels := make([]struct {
		ID   string
		Name string
	}, len(channels))

	for i, channel := range channels {
		slackChannels[i] = struct {
			ID   string
			Name string
		}{
			ID:   channel.ID,
			Name: channel.Name,
		}
	}

	// Prepare common data for the template
	data, err := getCommonData(r, "Profile")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		log.Printf("Error retrieving common data: %v", err)
		return
	}
	data["SlackChannels"] = slackChannels
	data["TagAlerts"] = tagAlerts

	// Render the template
	t := template.Must(template.ParseFiles("templates/layout.html", "templates/profile.html"))
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Unable to render template", http.StatusInternalServerError)
	}
}
