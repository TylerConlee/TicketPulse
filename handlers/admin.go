package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/gorilla/mux"
)

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	t := template.Must(template.ParseFiles("templates/layout.html", tmpl))
	err := t.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		log.Println("Error rendering template:", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

func UserManagementHandler(w http.ResponseWriter, r *http.Request) {

	users, err := models.GetAllUsers()
	if err != nil {
		http.Error(w, "Unable to retrieve users", http.StatusInternalServerError)
		return
	}

	data, err := getCommonData(r, "User Management")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}

	data["Users"] = users
	data["CurrentUserID"] = data["User"].(models.User).ID

	renderTemplate(w, "templates/admin/user_management.html", data)
}

func EditUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if r.Method == "POST" {
		handleEditUserPost(w, r, userID)
		return
	}

	user, err := models.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	data, err := getCommonData(r, "Edit User")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}

	data["EditUser"] = user

	renderTemplate(w, "templates/admin/edit_user.html", data)
}

func handleEditUserPost(w http.ResponseWriter, r *http.Request, userID int) {
	user, err := models.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	user.Name = r.FormValue("name")
	user.Role = models.Role(r.FormValue("role"))
	user.DailySummary = r.FormValue("daily_summary") == "on"

	err = models.UpdateUser(user)
	if err != nil {
		http.Error(w, "Unable to update user", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	currentUser := getCurrentUser(r)
	firstUserID, err := models.GetFirstUserID()
	if err != nil {
		http.Error(w, "Unable to retrieve first user ID", http.StatusInternalServerError)
		return
	}

	if userID == currentUser.ID || userID == firstUserID {
		http.Error(w, "You cannot delete this user.", http.StatusForbidden)
		return
	}

	err = models.DeleteUserByID(userID)
	if err != nil {
		http.Error(w, "Unable to delete user", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func NewUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		data, err := getCommonData(r, "Create New User")
		if err != nil {
			http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
			return
		}

		renderTemplate(w, "templates/admin/new_user.html", data)
		return
	}

	if r.Method == "POST" {
		email := r.FormValue("email")
		name := r.FormValue("name")
		role := models.Role(r.FormValue("role"))
		dailySummary := r.FormValue("daily_summary") == "on"

		err := models.CreateUser(email, name, role, dailySummary)
		if err != nil {
			log.Println("Error creating user:", err)
			http.Error(w, "Unable to create user", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	}
}

func TagManagementHandler(w http.ResponseWriter, r *http.Request) {
	tagAlerts, err := models.GetAllTagAlerts()
	if err != nil {
		http.Error(w, "Unable to retrieve tag alerts", http.StatusInternalServerError)
		return
	}

	data, err := getCommonData(r, "Tag Management")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}

	data["TagAlerts"] = tagAlerts

	renderTemplate(w, "templates/admin/tag_management.html", data)
}

func DeleteTagAlertHandler(w http.ResponseWriter, r *http.Request) {
	alertID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid tag alert ID", http.StatusBadRequest)
		return
	}

	err = models.DeleteTagAlert(alertID)
	if err != nil {
		http.Error(w, "Unable to delete tag alert", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/tags", http.StatusSeeOther)
}

func getCurrentUser(r *http.Request) models.User {
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)
	user, _ := models.GetUserByID(userID)
	return user
}
