package handlers

import (
	"database/sql"
	"encoding/json"
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

// ProfileHandler handles requests related to the user's profile.
func (h *AppHandler) ProfileHandler(w http.ResponseWriter, r *http.Request, slackService *services.SlackService) {
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)

	// Retrieve the user from the database
	user, err := models.GetUserByID(h.DB, userID)
	if err != nil {
		log.Println("Error retrieving user:", err)
		http.Error(w, "Unable to retrieve user", http.StatusInternalServerError)
		return
	}

	// Handle sql.NullTime conversion to a string for display
	summaryTime := ""
	if user.SummaryTime.Valid {
		summaryTime = user.SummaryTime.Time.Format("15:04") // Adjust format as needed
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
		if err := user.UpdateDailySummarySettings(h.DB, dailySummary, summaryTime); err != nil {
			http.Error(w, "Unable to update settings", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Handle Slack User ID update
	if r.Method == "POST" && r.URL.Path == "/profile/update-profile" {
		if slackService.IsReady() {
			slackEmail := r.FormValue("slack_email")
			if slackEmail == "" {
				slackEmail = user.Email
			}

			slackUserID, err := slackService.GetUserIDByEmail(slackEmail)
			if err == nil {
				user.SlackUserID = sql.NullString{String: slackUserID, Valid: true}
				if err := models.UpdateSlackUserID(h.DB, user.Email, user.SlackUserID.String); err != nil {
					log.Printf("Error updating user Slack ID: %v", err)
					http.Error(w, "Failed to update Slack ID", http.StatusInternalServerError)
					return
				}
			} else {
				log.Printf("Error retrieving Slack ID: %v", err)
			}
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Handle adding a new tag alert
	if r.Method == "POST" && r.URL.Path == "/profile/add-tag" {
		tag := r.FormValue("tag")
		slackChannelID := r.FormValue("slack_channel")
		alertType := r.FormValue("alert_type")

		if err := models.CreateTagAlert(h.DB, userID, tag, slackChannelID, alertType); err != nil {
			http.Error(w, "Unable to add tag alert", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Handle deleting a tag alert
	if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/profile/delete-tag/") {
		alertID, _ := strconv.Atoi(mux.Vars(r)["id"])

		if err := models.DeleteTagAlert(h.DB, alertID); err != nil {
			http.Error(w, "Unable to delete tag alert", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Retrieve the user's tag alerts
	tagAlerts, err := models.GetTagAlertsByUser(h.DB, userID)
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
	data, err := h.getCommonData(r, "Profile")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}
	data["SlackChannels"] = channels
	data["TagAlerts"] = tagAlerts
	data["User"] = user
	data["SummaryTime"] = summaryTime

	// Render the template
	t := template.Must(template.ParseFiles("templates/layout.html", "templates/profile.html"))
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Unable to render template", http.StatusInternalServerError)
	}
}

// OnDemandSummaryHandler handles the on-demand summary generation.
func (h *AppHandler) OnDemandSummaryHandler(w http.ResponseWriter, r *http.Request, slackService *services.SlackService) {
	session, _ := store.Get(r, "session-name")
	userEmail, ok := session.Values["user_email"].(string)
	if !ok || userEmail == "" {
		http.Error(w, "User email is missing", http.StatusBadRequest)
		return
	}
	log.Println("Generating on-demand summary for user:", userEmail)

	// Create a new Zendesk client
	zendeskClient, err := services.NewZendeskClient(h.DB)
	if err != nil {
		log.Printf("Error creating Zendesk client: %v", err)
		http.Error(w, "Failed to create Zendesk client", http.StatusInternalServerError)
		return
	}

	summary, err := zendeskClient.GenerateDailySummary(userEmail, slackService)
	if err != nil {
		log.Printf("Error generating summary: %v", err)
		http.Error(w, "Failed to generate summary", http.StatusInternalServerError)
		return
	}

	// Return the summary as JSON
	json.NewEncoder(w).Encode(map[string]string{"message": summary})
}
