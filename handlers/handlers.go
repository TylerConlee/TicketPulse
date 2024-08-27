package handlers

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/gorilla/mux"
)

var templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/*.html"))

// HomeHandler displays the home page with a list of users.
func (h *AppHandler) HomeHandler(w http.ResponseWriter, r *http.Request) {
	users, err := models.GetAllUsers(h.DB)
	if err != nil {
		http.Error(w, "Unable to retrieve users", http.StatusInternalServerError)
		return
	}
	templates.ExecuteTemplate(w, "index.html", users)
}

// ViewHandler displays a user's details based on their ID.
func (h *AppHandler) ViewHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	item, err := models.GetUserByID(h.DB, id)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	templates.ExecuteTemplate(w, "view.html", item)
}

// UpdateHandler updates a user's details or displays the update form.
func (h *AppHandler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	if r.Method == "POST" {
		name := r.FormValue("name")
		role := models.Role(r.FormValue("role"))
		email := r.FormValue("email")
		user := models.User{
			ID:    id,
			Name:  name,
			Role:  role,
			Email: email,
		}
		if err := models.UpdateUser(h.DB, user); err != nil {
			http.Error(w, "Unable to update user", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	item, err := models.GetUserByID(h.DB, id)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	templates.ExecuteTemplate(w, "update.html", item)
}

// LogoutHandler logs the user out by clearing the session.
func (h *AppHandler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	session.Options.MaxAge = -1 // This deletes the session
	err := session.Save(r, w)
	if err != nil {
		http.Error(w, "Unable to logout", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// getCommonData retrieves common data used across multiple pages.
func (h *AppHandler) getCommonData(r *http.Request, title string) (map[string]interface{}, error) {
	user := h.getCurrentUser(r)
	firstUserID, err := models.GetFirstUserID(h.DB)
	if err != nil {
		return nil, err
	}
	// Retrieve the notifications from the context
	notifications := middlewares.GetNotificationsFromContext(r)

	data := map[string]interface{}{
		"Title":         title,
		"User":          user,
		"FirstUserID":   firstUserID,
		"Notifications": notifications,
	}

	return data, nil
}
