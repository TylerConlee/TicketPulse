package handlers

import (
	"net/http"

	"github.com/TylerConlee/TicketPulse/models"
)

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)
	user, _ := models.GetUserByID(userID)

	data := struct {
		Title string
		User  models.User
	}{
		Title: "Dashboard",
		User:  user,
	}

	templates.ExecuteTemplate(w, "layout.html", data)
}
