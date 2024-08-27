package handlers

import (
	"net/http"

	"github.com/TylerConlee/TicketPulse/models"
)

// SettingsHandler handles general user settings (for agents).
func (h *AppHandler) SettingsHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve user session
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)

	// Retrieve the user from the database
	user, _ := models.GetUserByID(h.DB, userID)

	if r.Method == "POST" {
		// Update user's settings
		// You can add logic here to update the user's settings based on the form data.
	}

	// Load the settings page
	templates.ExecuteTemplate(w, "settings.html", user)
}

// AdminSettingsHandler handles admin-specific settings.
func (h *AppHandler) AdminSettingsHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve user session
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)

	// Retrieve the user from the database
	user, _ := models.GetUserByID(h.DB, userID)

	if r.Method == "POST" {
		// Handle admin-specific configuration updates
		// You can add logic here to handle configuration updates for admin users.
	}

	// Load the admin settings page
	templates.ExecuteTemplate(w, "admin_settings.html", user)
}
