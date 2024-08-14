package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/TylerConlee/TicketPulse/models"

	"github.com/gorilla/mux"
)

func UserManagementHandler(w http.ResponseWriter, r *http.Request) {
	users, err := models.GetAllUsers()
	if err != nil {
		http.Error(w, "Unable to retrieve users", http.StatusInternalServerError)
		log.Println("Error retrieving users:", err)
		return
	}

	data := struct {
		Title string
		User  models.User
		Users []models.User
	}{
		Title: "User Management",
		User:  getCurrentUser(r), // Function to get the current logged-in user
		Users: users,
	}

	t := template.Must(template.ParseFiles("templates/layout.html", "templates/admin/user_management.html"))
	t.ExecuteTemplate(w, "layout.html", data)
}

func EditUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := models.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if r.Method == "POST" {
		user.Name = r.FormValue("name")
		user.Role = models.Role(r.FormValue("role"))
		user.DailySummary = r.FormValue("daily_summary") == "on"

		err = models.UpdateUser(user)
		if err != nil {
			http.Error(w, "Unable to update user", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	session, _ := store.Get(r, "session-name")
	currentUserID := session.Values["user_id"].(int)
	currentUser, _ := models.GetUserByID(currentUserID)

	data := struct {
		Title    string
		User     models.User
		EditUser models.User
	}{
		Title:    "Edit User",
		User:     currentUser,
		EditUser: user,
	}
	log.Println("Rendering Edit User Page for user ID:", userID)
	log.Printf("Data: %+v\n", data)
	// This explicitly selects the "content" block from "user_management.html"
	t := template.Must(template.ParseFiles("templates/layout.html", "templates/admin/edit_user.html"))
	t.ExecuteTemplate(w, "layout.html", data)

}

func NewUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		session, _ := store.Get(r, "session-name")
		userID := session.Values["user_id"].(int)
		currentUser, _ := models.GetUserByID(userID)

		data := struct {
			Title string
			User  models.User
		}{
			Title: "Create New User",
			User:  currentUser,
		}

		t := template.Must(template.ParseFiles("templates/layout.html", "templates/admin/new_user.html"))
		t.ExecuteTemplate(w, "layout.html", data)
		return
	}

	if r.Method == "POST" {
		email := r.FormValue("email")
		name := r.FormValue("name")
		role := models.Role(r.FormValue("role"))
		dailySummary := r.FormValue("daily_summary") == "on"

		err := models.CreateUser(email, name, role, dailySummary)
		if err != nil {
			http.Error(w, "Unable to create user", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	}
}

func getCurrentUser(r *http.Request) models.User {
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)
	user, _ := models.GetUserByID(userID)
	return user
}
