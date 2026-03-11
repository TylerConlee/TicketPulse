package handlers

import (
	"log"
	"net/http"

	"github.com/TylerConlee/TicketPulse/models"
)

const sensitiveValuePlaceholder = "********"

// maskSensitiveValue returns a masked version of a sensitive string, showing only
// a prefix hint (e.g., "xoxb-****...") so the user knows a value is set.
func maskSensitiveValue(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return sensitiveValuePlaceholder
	}
	return value[:5] + sensitiveValuePlaceholder
}

// ConfigurationHandler handles the configuration management page.
func (h *AppHandler) ConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := h.saveConfigurationSettings(r)
		if err != nil {
			http.Error(w, "Unable to save configuration", http.StatusInternalServerError)
			return
		}

		if h.ConfigCache != nil {
			h.ConfigCache.InvalidateAll()
		}
		if h.notifyConfigChange != nil {
			h.notifyConfigChange()
		}

		http.Redirect(w, r, "/admin/configuration", http.StatusSeeOther)
		return
	}

	configs, err := models.GetAllConfigurations(h.DB)
	if err != nil {
		http.Error(w, "Unable to load configuration", http.StatusInternalServerError)
		return
	}

	maskedConfigs := make(map[string]string)
	for k, v := range configs {
		if models.SensitiveKeys[k] {
			maskedConfigs[k] = maskSensitiveValue(v)
		} else {
			maskedConfigs[k] = v
		}
	}

	data, err := h.getCommonData(r, "Application Configuration")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}
	data["Configs"] = maskedConfigs

	h.renderTemplate(w, "templates/admin/configuration.html", data)
}

// saveConfigurationSettings saves the updated configuration settings to the database.
// Sensitive values that match the placeholder are skipped (user didn't change them).
func (h *AppHandler) saveConfigurationSettings(r *http.Request) error {
	configs := map[string]string{
		"daily_summary_enabled": r.FormValue("daily_summary_enabled"),
		"slack_app_token":       r.FormValue("slack_app_token"),
		"slack_bot_token":       r.FormValue("slack_bot_token"),
		"zendesk_api_key":       r.FormValue("zendesk_api_key"),
		"zendesk_subdomain":     r.FormValue("zendesk_subdomain"),
		"zendesk_email":         r.FormValue("zendesk_email"),
	}

	for key, value := range configs {
		if models.SensitiveKeys[key] && value == "" {
			continue
		}
		err := models.SetConfiguration(h.DB, key, value)
		if err != nil {
			log.Printf("Error saving config for key %s: %v", key, err)
			return err
		}
	}
	return nil
}
