package handlers

import (
	"html/template"
	"log"
	"net/http"

	"github.com/TylerConlee/TicketPulse/models"
)

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	userID, ok := session.Values["user_id"].(int)
	if !ok {
		log.Println("No user_id found in session")
		http.Error(w, "Unable to retrieve user ID from session", http.StatusInternalServerError)
		return
	}

	log.Println("Retrieved user_id from session:", userID)

	user, err := models.GetUserByID(userID)
	if err != nil {
		log.Println("Error retrieving user from DB:", err)
		http.Error(w, "Unable to retrieve user", http.StatusInternalServerError)
		return
	}

	log.Printf("User retrieved: %+v\n", user)

	data := struct {
		Title string
		User  models.User
	}{
		Title: "Dashboard",
		User:  user,
	}

	t := template.Must(template.ParseFiles("templates/layout.html", "templates/dashboard.html"))
	t.ExecuteTemplate(w, "layout.html", data)
}
