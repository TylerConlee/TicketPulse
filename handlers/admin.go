package handlers

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/gorilla/mux"
)

func (h *AppHandler) renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	// Check if we are running in a test environment
	if os.Getenv("GO_ENV") == "test" {
		// In a test environment, mock the template rendering
		log.Println("Mock rendering template for test environment:", tmpl)
		return
	}

	t, err := template.New("").ParseFiles("templates/layout.html", tmpl)
	if err != nil {
		log.Println("Error parsing template:", err)
		http.Error(w, "Error parsing template", http.StatusInternalServerError)
		return
	}

	err = t.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		log.Println("Error rendering template:", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

func (h *AppHandler) UserManagementHandler(w http.ResponseWriter, r *http.Request) {
	users, err := models.GetAllUsers(h.DB)
	if err != nil {
		http.Error(w, "Unable to retrieve users", http.StatusInternalServerError)
		return
	}

	data, err := h.getCommonData(r, "User Management")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}

	data["Users"] = users
	data["CurrentUserID"] = data["User"].(models.User).ID

	h.renderTemplate(w, "templates/admin/user_management.html", data)
}

func (h *AppHandler) EditUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if r.Method == "POST" {
		h.handleEditUserPost(w, r, userID)
		return
	}

	user, err := models.GetUserByID(h.DB, userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	data, err := h.getCommonData(r, "Edit User")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}

	data["EditUser"] = user

	h.renderTemplate(w, "templates/admin/edit_user.html", data)
}

func (h *AppHandler) handleEditUserPost(w http.ResponseWriter, r *http.Request, userID int) {
	user, err := models.GetUserByID(h.DB, userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	user.Name = r.FormValue("name")
	user.Role = models.Role(r.FormValue("role"))
	user.DailySummary = r.FormValue("daily_summary") == "on"

	err = models.UpdateUser(h.DB, user)
	if err != nil {
		http.Error(w, "Unable to update user", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *AppHandler) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	currentUser := h.getCurrentUser(r)
	firstUserID, err := models.GetFirstUserID(h.DB)
	if err != nil {
		http.Error(w, "Unable to retrieve first user ID", http.StatusInternalServerError)
		return
	}

	if userID == currentUser.ID || userID == firstUserID {
		http.Error(w, "You cannot delete this user.", http.StatusForbidden)
		return
	}

	err = models.DeleteUserByID(h.DB, userID)
	if err != nil {
		http.Error(w, "Unable to delete user", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *AppHandler) NewUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		data, err := h.getCommonData(r, "Create New User")
		if err != nil {
			http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
			return
		}

		h.renderTemplate(w, "templates/admin/new_user.html", data)
		return
	}

	if r.Method == "POST" {
		email := r.FormValue("email")
		name := r.FormValue("name")
		role := models.Role(r.FormValue("role"))
		dailySummary := r.FormValue("daily_summary") == "on"

		err := models.CreateUser(h.DB, email, name, role, dailySummary)
		if err != nil {
			log.Println("Error creating user:", err)
			http.Error(w, "Unable to create user", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	}
}

func (h *AppHandler) TagManagementHandler(w http.ResponseWriter, r *http.Request) {
	tagAlerts, err := models.GetAllTagAlerts(h.DB)
	if err != nil {
		http.Error(w, "Unable to retrieve tag alerts", http.StatusInternalServerError)
		return
	}

	data, err := h.getCommonData(r, "Tag Management")
	if err != nil {
		http.Error(w, "Unable to retrieve common data", http.StatusInternalServerError)
		return
	}

	data["TagAlerts"] = tagAlerts

	h.renderTemplate(w, "templates/admin/tag_management.html", data)
}

func (h *AppHandler) DeleteTagAlertHandler(w http.ResponseWriter, r *http.Request) {
	alertID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid tag alert ID", http.StatusBadRequest)
		return
	}

	err = models.DeleteTagAlert(h.DB, alertID)
	if err != nil {
		http.Error(w, "Unable to delete tag alert", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/tags", http.StatusSeeOther)
}

func (h *AppHandler) getCurrentUser(r *http.Request) models.User {
	session, _ := store.Get(r, "session-name")
	userID := session.Values["user_id"].(int)
	user, _ := models.GetUserByID(h.DB, userID)
	return user
}
