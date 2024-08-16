package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"

	"github.com/gorilla/mux"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request, slackService *services.SlackService) {
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)

	// Retrieve the user from the database
	user, err := models.GetUserByID(userID)
	if err != nil {
		log.Println("Error retrieving user:", err)
		http.Error(w, "Unable to retrieve user", http.StatusInternalServerError)
		return
	}

	// Handle updating daily summary settings
	if r.Method == "POST" && r.URL.Path == "/profile/update-summary-settings" {
		dailySummary := r.FormValue("daily_summary") == "on"
		summaryTime, err := time.Parse("15:04", r.FormValue("summary_time"))
		if err != nil {
			http.Error(w, "Invalid time format", http.StatusBadRequest)
			return
		}
		// Update user settings
		err = user.UpdateDailySummarySettings(dailySummary, summaryTime)
		if err != nil {
			http.Error(w, "Unable to update settings", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

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
	tagAlerts, err := models.GetTagAlertsByUser(userID)
	if err != nil {
		http.Error(w, "Unable to retrieve tag alerts", http.StatusInternalServerError)
		return
	}

	// Fetch available Slack channels
	channels := []struct {
		ID   string
		Name string
	}{}
	if slackService != nil && slackService.IsReady() {
		slackChannels, err := slackService.GetConversations()
		if err == nil {
			for _, channel := range slackChannels {
				channels = append(channels, struct {
					ID   string
					Name string
				}{
					ID:   channel.ID,
					Name: channel.Name,
				})
			}
		} else {
			log.Println("Error fetching Slack channels:", err)
		}
	}

	// Prepare common data for the template
	data, err := getCommonData(r, "Profile")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}
	data["SlackChannels"] = channels
	data["TagAlerts"] = tagAlerts
	data["User"] = user

	// Render the template
	t := template.Must(template.ParseFiles("templates/layout.html", "templates/profile.html"))
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Unable to render template", http.StatusInternalServerError)
	}
}
