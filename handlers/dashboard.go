package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"
)

var funcMap = template.FuncMap{
	"replace": strings.ReplaceAll,
	"split":   strings.Split,
}

// DashboardHandler handles requests to the dashboard and injects dependencies via AdminHandler.
func (h *AppHandler) DashboardHandler(dashboardService *services.DashboardService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract user ID from the context
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok || userID == 0 {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Get the user information from the database
		user, err := models.GetUserByID(h.DB, userID)
		if err != nil {
			log.Println("Error getting user information:", err)
			http.Error(w, "Failed to get user information", http.StatusInternalServerError)
			return
		}

		// Retrieve alert stats for the user
		stats, err := dashboardService.GetAlertStatsForUser(userID)
		if err != nil {
			log.Println("Error getting alert stats:", err)
			http.Error(w, "Failed to get alert stats", http.StatusInternalServerError)
			return
		}

		// Process stats for rendering in the dashboard
		newTicketData := processAlertStatsForChart(stats, "new_ticket")
		slaDeadlineData := processAlertStatsForChart(stats, "sla_deadline")
		ticketUpdateData := processAlertStatsForChart(stats, "ticket_update")

		// Convert data to JSON for use in JavaScript
		newTicketDataJSON, _ := json.Marshal(newTicketData)
		slaDeadlineDataJSON, _ := json.Marshal(slaDeadlineData)
		ticketUpdateDataJSON, _ := json.Marshal(ticketUpdateData)

		// Render the dashboard template with the processed data
		t := template.Must(template.New("layout.html").Funcs(funcMap).ParseFiles("templates/layout.html", "templates/dashboard.html"))
		if err := t.ExecuteTemplate(w, "layout.html", map[string]interface{}{
			"Title":               "Dashboard",
			"AlertData":           stats,
			"User":                user,
			"HasNewTicketData":    len(newTicketData) > 0,
			"HasTicketUpdateData": len(ticketUpdateData) > 0,
			"HasSlaDeadlineData":  len(slaDeadlineData) > 0,
			"NewTicketData":       template.JS(newTicketDataJSON),
			"SlaDeadlineData":     template.JS(slaDeadlineDataJSON),
			"TicketUpdateData":    template.JS(ticketUpdateDataJSON),
		}); err != nil {
			log.Printf("Error rendering template: %v", err)
			http.Error(w, "Unable to render template", http.StatusInternalServerError)
		}
	}
}

// processAlertStatsForChart transforms the AlertStats data into a Chart.js-compatible format.
func processAlertStatsForChart(stats []services.AlertStats, alertType string) map[string]interface{} {
	labelsMap := make(map[string]bool)
	tagData := make(map[string]map[string]int) // map[tag]map[date]count

	// Organize data by date and tag
	for _, stat := range stats {
		if stat.AlertType == alertType {
			labelsMap[stat.Date] = true

			if _, ok := tagData[stat.Tag]; !ok {
				tagData[stat.Tag] = make(map[string]int)
			}
			tagData[stat.Tag][stat.Date] = stat.AlertCount
		}
	}

	// Sort dates
	var labels []string
	for date := range labelsMap {
		labels = append(labels, date)
	}
	sort.Strings(labels)

	// Create datasets for Chart.js
	var datasets []map[string]interface{}
	colors := []string{"#FF6384", "#36A2EB", "#FFCE56"} // Example colors
	i := 0

	for tag, data := range tagData {
		dataset := make(map[string]interface{})
		dataset["label"] = tag
		dataset["backgroundColor"] = colors[i%len(colors)]
		dataset["borderColor"] = colors[i%len(colors)]
		dataset["fill"] = false

		var dataPoints []int
		for _, date := range labels {
			dataPoints = append(dataPoints, data[date])
		}
		dataset["data"] = dataPoints
		datasets = append(datasets, dataset)
		i++
	}

	return map[string]interface{}{
		"labels":   labels,
		"datasets": datasets,
	}
}
