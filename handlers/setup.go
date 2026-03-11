package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/gorilla/csrf"
	"github.com/slack-go/slack"
)

// SetupHandler renders and processes the multi-step setup wizard.
func (h *AppHandler) SetupHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	roleValue, _ := session.Values["role"]
	var userRole models.Role
	switch v := roleValue.(type) {
	case models.Role:
		userRole = v
	case string:
		userRole = models.Role(v)
	}
	if userRole != models.AdminRole {
		http.Error(w, "Only admins can run the setup wizard", http.StatusForbidden)
		return
	}

	if r.Method == "POST" {
		h.handleSetupPost(w, r)
		return
	}

	step := r.URL.Query().Get("step")
	if step == "" {
		step = "welcome"
	}

	configs, _ := models.GetAllConfigurations(h.DB)

	maskedConfigs := make(map[string]string)
	for k, v := range configs {
		if models.SensitiveKeys[k] {
			maskedConfigs[k] = maskSensitiveValue(v)
		} else {
			maskedConfigs[k] = v
		}
	}

	data := map[string]interface{}{
		"Step":      step,
		"Configs":   maskedConfigs,
		"CSRFField": template.HTML(csrf.TemplateField(r)),
		"CSPNonce":  middlewares.GetCSPNonce(r),
	}

	t, err := template.ParseFiles("templates/setup/wizard.html")
	if err != nil {
		log.Println("Error parsing setup template:", err)
		http.Error(w, "Error loading setup wizard", http.StatusInternalServerError)
		return
	}
	if err := t.Execute(w, data); err != nil {
		log.Println("Error rendering setup template:", err)
		http.Error(w, "Error rendering setup wizard", http.StatusInternalServerError)
	}
}

func (h *AppHandler) handleSetupPost(w http.ResponseWriter, r *http.Request) {
	step := r.FormValue("step")

	switch step {
	case "zendesk":
		subdomain := strings.TrimSpace(r.FormValue("zendesk_subdomain"))
		email := strings.TrimSpace(r.FormValue("zendesk_email"))
		apiKey := strings.TrimSpace(r.FormValue("zendesk_api_key"))

		if subdomain == "" || email == "" {
			http.Error(w, "Zendesk subdomain and email are required", http.StatusBadRequest)
			return
		}

		existingKey, _ := models.GetConfiguration(h.DB, "zendesk_api_key")
		if apiKey == "" && existingKey == "" {
			http.Error(w, "Zendesk API key is required", http.StatusBadRequest)
			return
		}

		models.SetConfiguration(h.DB, "zendesk_subdomain", subdomain)
		models.SetConfiguration(h.DB, "zendesk_email", email)
		if apiKey != "" {
			models.SetConfiguration(h.DB, "zendesk_api_key", apiKey)
		}

		http.Redirect(w, r, "/setup?step=slack", http.StatusSeeOther)

	case "slack":
		botToken := strings.TrimSpace(r.FormValue("slack_bot_token"))
		appToken := strings.TrimSpace(r.FormValue("slack_app_token"))

		existingBot, _ := models.GetConfiguration(h.DB, "slack_bot_token")
		existingApp, _ := models.GetConfiguration(h.DB, "slack_app_token")

		if botToken == "" && existingBot == "" {
			http.Error(w, "Slack bot token is required", http.StatusBadRequest)
			return
		}
		if appToken == "" && existingApp == "" {
			http.Error(w, "Slack app token is required", http.StatusBadRequest)
			return
		}

		if botToken != "" {
			models.SetConfiguration(h.DB, "slack_bot_token", botToken)
		}
		if appToken != "" {
			models.SetConfiguration(h.DB, "slack_app_token", appToken)
		}

		http.Redirect(w, r, "/setup?step=review", http.StatusSeeOther)

	case "complete":
		models.SetConfiguration(h.DB, "setup_complete", "true")

		if h.ConfigCache != nil {
			h.ConfigCache.InvalidateAll()
		}
		if h.notifyConfigChange != nil {
			h.notifyConfigChange()
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)

	default:
		http.Redirect(w, r, "/setup?step=welcome", http.StatusSeeOther)
	}
}

// TestZendeskHandler validates Zendesk credentials via API call.
func (h *AppHandler) TestZendeskHandler(w http.ResponseWriter, r *http.Request) {
	subdomain := strings.TrimSpace(r.FormValue("zendesk_subdomain"))
	email := strings.TrimSpace(r.FormValue("zendesk_email"))
	apiKey := strings.TrimSpace(r.FormValue("zendesk_api_key"))

	log.Printf("TestZendesk: subdomain=%q email=%q apiKey_len=%d", subdomain, email, len(apiKey))

	if subdomain == "" || email == "" || apiKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "All fields are required"})
		return
	}

	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/users/me.json", subdomain)
	req, err := http.NewRequestWithContext(r.Context(), "GET", endpoint, nil)
	if err != nil {
		log.Printf("TestZendesk: failed to create request: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create request"})
		return
	}
	req.SetBasicAuth(email+"/token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("TestZendesk: HTTP request failed: %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("TestZendesk: Zendesk returned %s: %s", resp.Status, string(body))
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Zendesk returned %s: %s", resp.Status, string(body)),
		})
		return
	}

	var result struct {
		User struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("TestZendesk: failed to decode response: %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to parse response"})
		return
	}

	log.Printf("TestZendesk: success - connected as %s (%s)", result.User.Name, result.User.Email)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Connected as %s (%s)", result.User.Name, result.User.Email),
	})
}

// TestSlackHandler validates Slack tokens via auth.test API call.
func (h *AppHandler) TestSlackHandler(w http.ResponseWriter, r *http.Request) {
	botToken := strings.TrimSpace(r.FormValue("slack_bot_token"))
	appToken := strings.TrimSpace(r.FormValue("slack_app_token"))

	log.Printf("TestSlack: botToken_len=%d appToken_len=%d", len(botToken), len(appToken))

	if botToken == "" || appToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Both tokens are required"})
		return
	}

	if !strings.HasPrefix(botToken, "xoxb-") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bot token must start with xoxb-"})
		return
	}
	if !strings.HasPrefix(appToken, "xapp-") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "App token must start with xapp-"})
		return
	}

	client := slack.New(botToken)
	resp, err := client.AuthTest()
	if err != nil {
		log.Printf("TestSlack: auth.test failed: %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	log.Printf("TestSlack: success - workspace=%q user=%s", resp.Team, resp.User)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Connected to workspace %q as %s", resp.Team, resp.User),
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
