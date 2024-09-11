//go:build !test

package handlers

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/gorilla/mux"
)

// AppHandler struct will hold the database instance
type AppHandler struct {
	DB db.Database
}

// NewAppHandler initializes the AppHandler with a database
func NewAppHandler(db db.Database) *AppHandler {
	return &AppHandler{DB: db}
}

// HomeHandler displays the home page with a list of users.
func (h *AppHandler) HomeHandler(w http.ResponseWriter, r *http.Request) {
	// Load the templates
	tmpl, err := template.New("").Funcs(funcMap).ParseGlob("templates/*.html")
	if err != nil {
		http.Error(w, "Unable to load templates", http.StatusInternalServerError)
		return
	}
	users, err := models.GetAllUsers(h.DB)
	if err != nil {
		http.Error(w, "Unable to retrieve users", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "index.html", users); err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

// ViewHandler displays a user's details based on their ID.
func (h *AppHandler) ViewHandler(w http.ResponseWriter, r *http.Request) {
	// Load the templates
	tmpl, err := template.New("").Funcs(funcMap).ParseGlob("templates/*.html")
	if err != nil {
		http.Error(w, "Unable to load templates", http.StatusInternalServerError)
		return
	}

	// Retrieve data
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	item, err := models.GetUserByID(h.DB, id)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Render the template
	if err := tmpl.ExecuteTemplate(w, "view.html", item); err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
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

	// Load the templates
	tmpl, err := template.New("").Funcs(funcMap).ParseGlob("templates/*.html")
	if err != nil {
		http.Error(w, "Unable to load templates", http.StatusInternalServerError)
		return
	}

	// Retrieve data
	item, err := models.GetUserByID(h.DB, id)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Render the template
	if err := tmpl.ExecuteTemplate(w, "update.html", item); err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
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
