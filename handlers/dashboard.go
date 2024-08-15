package handlers

import (
	"net/http"
)

func DashboardHandler(w http.ResponseWriter, r *http.Request) {

	data, err := getCommonData(r, "Dashboard")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "templates/dashboard.html", data)
}
