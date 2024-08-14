package handlers

import (
	"net/http"

	"github.com/TylerConlee/TicketPulse/models"
)

func SettingsHandler(w http.ResponseWriter, r *http.Request) {
	// General user settings (for agents)
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)
	user, _ := models.GetUserByID(userID)

	if r.Method == "POST" {
		// Update user's settings
	}

	// Load the settings page
	templates.ExecuteTemplate(w, "settings.html", user)
}

func AdminSettingsHandler(w http.ResponseWriter, r *http.Request) {
	// Admin-specific settings
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)
	user, _ := models.GetUserByID(userID)

	if r.Method == "POST" {
		// Handle admin-specific configuration updates
	}

	// Load the admin settings page
	templates.ExecuteTemplate(w, "admin_settings.html", user)
}
