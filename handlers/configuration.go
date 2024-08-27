package handlers

import (
	"log"
	"net/http"

	"github.com/TylerConlee/TicketPulse/models"
)

// ConfigurationHandler handles the configuration management page.
func (h *AppHandler) ConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := h.saveConfigurationSettings(r)
		if err != nil {
			http.Error(w, "Unable to save configuration", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/configuration", http.StatusSeeOther)
		return
	}

	// Load all configurations
	configs, err := models.GetAllConfigurations(h.DB)
	if err != nil {
		http.Error(w, "Unable to load configuration", http.StatusInternalServerError)
		return
	}

	// Get common data and render the template
	data, err := h.getCommonData(r, "Application Configuration")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}
	data["Configs"] = configs

	h.renderTemplate(w, "templates/admin/configuration.html", data)
}

// saveConfigurationSettings saves the updated configuration settings to the database.
func (h *AppHandler) saveConfigurationSettings(r *http.Request) error {
	configs := map[string]string{
		"daily_summary_enabled": r.FormValue("daily_summary_enabled"),
		"slack_app_token":       r.FormValue("slack_app_token"),
		"slack_bot_token":       r.FormValue("slack_bot_token"),
		"zendesk_api_key":       r.FormValue("zendesk_api_key"),
		"zendesk_subdomain":     r.FormValue("zendesk_subdomain"),
		"zendesk_email":         r.FormValue("zendesk_email"), // New entry
	}

	for key, value := range configs {
		err := models.SetConfiguration(h.DB, key, value)
		if err != nil {
			log.Printf("Error saving config for key %s: %v", key, err)
			return err
		}
	}
	return nil
}
