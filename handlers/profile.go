package handlers

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"
)

var validWorkDays = map[string]bool{
	"Monday": true, "Tuesday": true, "Wednesday": true,
	"Thursday": true, "Friday": true, "Saturday": true, "Sunday": true,
}

func validateWorkDaysJSON(raw string) bool {
	var days []string
	if err := json.Unmarshal([]byte(raw), &days); err != nil {
		return false
	}
	if len(days) > 7 {
		return false
	}
	for _, d := range days {
		if !validWorkDays[d] {
			return false
		}
	}
	return true
}

// ProfileHandler handles requests related to the user's profile.
func (h *AppHandler) ProfileHandler(w http.ResponseWriter, r *http.Request, slackService *services.SlackService) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

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

	// Handle updating work day settings
	if r.Method == "POST" && r.URL.Path == "/profile/update-work-day-settings" {
		startTimeStr := r.FormValue("work_day_start_time")
		endTimeStr := r.FormValue("work_day_end_time")
		timezone := r.FormValue("timezone")
		workDaysJSON := r.FormValue("work_days") // JSON array of day names

		startTime, err := time.Parse("15:04", startTimeStr)
		if err != nil {
			http.Error(w, "Invalid start time format", http.StatusBadRequest)
			return
		}

		endTime, err := time.Parse("15:04", endTimeStr)
		if err != nil {
			http.Error(w, "Invalid end time format", http.StatusBadRequest)
			return
		}

		if timezone == "" {
			timezone = "UTC"
		}

		if workDaysJSON == "" {
			workDaysJSON = `["Monday","Tuesday","Wednesday","Thursday","Friday"]`
		}
		if !validateWorkDaysJSON(workDaysJSON) {
			http.Error(w, "Invalid work days format", http.StatusBadRequest)
			return
		}

		if err := user.UpdateWorkDaySettings(h.DB, startTime, endTime, timezone, workDaysJSON); err != nil {
			http.Error(w, "Unable to update work day settings", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Handle updating summary filter settings
	if r.Method == "POST" && r.URL.Path == "/profile/update-summary-filters" {
		tagFilter := r.FormValue("summary_tag_filter")
		ticketFilter := r.FormValue("summary_ticket_filter")

		if tagFilter != services.TagFilterAllTags && tagFilter != services.TagFilterConfiguredTags {
			tagFilter = services.TagFilterAllTags // Default
		}

		if ticketFilter != services.TicketFilterAllTickets && ticketFilter != services.TicketFilterAssigned {
			ticketFilter = services.TicketFilterAssigned // Default
		}

		// Update filter settings
		if err := user.UpdateSummaryFilterSettings(h.DB, tagFilter, ticketFilter); err != nil {
			http.Error(w, "Unable to update filter settings", http.StatusInternalServerError)
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

	// Prepare common data for the template
	data, err := h.getCommonData(r, "Profile")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}
	data["User"] = user
	data["SummaryTime"] = summaryTime

	// Render the template
	t := template.Must(template.ParseFiles("templates/layout.html", "templates/profile.html"))
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Unable to render template", http.StatusInternalServerError)
		return
	}
}

// OnDemandSummaryHandler handles the on-demand summary generation.
func (h *AppHandler) OnDemandSummaryHandler(w http.ResponseWriter, r *http.Request, slackService *services.SlackService) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Error(w, "User ID is missing", http.StatusBadRequest)
		return
	}

	userEmail, ok := session.Values["user_email"].(string)
	if !ok || userEmail == "" {
		http.Error(w, "User email is missing", http.StatusBadRequest)
		return
	}
	log.Println("Generating on-demand summary for user:", userEmail)

	// Get user settings
	user, err := models.GetUserByID(h.DB, userID)
	if err != nil {
		log.Printf("Error retrieving user: %v", err)
		http.Error(w, "Unable to retrieve user", http.StatusInternalServerError)
		return
	}

	// Set defaults if work day settings are not configured
	workDayStart := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC) // Default 9 AM
	workDayEnd := time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC)   // Default 5 PM
	timezone := "UTC"
	if user.WorkDayStartTime.Valid {
		workDayStart = user.WorkDayStartTime.Time
	}
	if user.WorkDayEndTime.Valid {
		workDayEnd = user.WorkDayEndTime.Time
	}
	if user.Timezone.Valid {
		timezone = user.Timezone.String
	}

	// Get user's configured tags if tag filter is set to configured_tags
	var userTags []string
	tagFilterMode := user.SummaryTagFilter
	if tagFilterMode == "" {
		tagFilterMode = services.TagFilterAllTags
	}
	if tagFilterMode == services.TagFilterConfiguredTags {
		tagAlerts, err := models.GetTagAlertsByUser(h.DB, userID)
		if err == nil {
			tagMap := make(map[string]bool)
			for _, alert := range tagAlerts {
				if !tagMap[alert.Tag] {
					tagMap[alert.Tag] = true
					userTags = append(userTags, alert.Tag)
				}
			}
		}
	}

	ticketFilterMode := user.SummaryTicketFilter
	if ticketFilterMode == "" {
		ticketFilterMode = services.TicketFilterAssigned
	}

	// Create a new Zendesk client
	zendeskClient, err := services.NewZendeskClient(h.DB)
	if err != nil {
		log.Printf("Error creating Zendesk client: %v", err)
		http.Error(w, "Failed to create Zendesk client", http.StatusInternalServerError)
		return
	}

	summary, err := zendeskClient.GenerateDailySummary(userEmail, slackService, workDayStart, workDayEnd, timezone, tagFilterMode, ticketFilterMode, userTags)
	if err != nil {
		log.Printf("Error generating summary: %v", err)
		http.Error(w, "Failed to generate summary", http.StatusInternalServerError)
		return
	}

	// Return the summary as JSON
	json.NewEncoder(w).Encode(map[string]string{"message": summary})
}
