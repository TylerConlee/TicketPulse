// handlers/dashboard.go

package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"
)

var funcMap = template.FuncMap{
	"replace": strings.ReplaceAll,
	"split":   strings.Split,
}

func DashboardHandler(dashboardService *services.DashboardService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Extract user ID from the context
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok || userID == 0 {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Get the user information from the database
		user, err := models.GetUserByID(userID)
		if err != nil {
			log.Println("Error getting user information:", err)
			http.Error(w, "Failed to get user information", http.StatusInternalServerError)
			return
		}

		// Attempt to retrieve alert stats for the user
		stats, err := dashboardService.GetAlertStatsForUser(userID)
		if err != nil {
			log.Println("Error getting alert stats:", err)
			http.Error(w, "Failed to get alert stats", http.StatusInternalServerError)
			return
		}

		// Render the dashboard template with the data or a flag for no data

		t := template.Must(template.ParseFiles("templates/layout.html", "templates/dashboard.html"))
		if err := t.ExecuteTemplate(w, "layout.html", map[string]interface{}{
			"Title":        "Dashboard",
			"AlertData":    stats,
			"User":         user,
			"HasAlertData": len(stats) > 0,
		}); err != nil {
			log.Printf("Error rendering template: %v", err)
			http.Error(w, "Unable to render template", http.StatusInternalServerError)
		}
	}
}
