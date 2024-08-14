package handlers

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/TylerConlee/TicketPulse/models"

	"github.com/gorilla/mux"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	users, err := models.GetAllUsers()
	if err != nil {
		http.Error(w, "Unable to retrieve users", http.StatusInternalServerError)
		return
	}
	templates.ExecuteTemplate(w, "index.html", users)
}

func ViewHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	item, err := models.GetUserByID(id)
	if err != nil {
		// handle error
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
	templates.ExecuteTemplate(w, "view.html", item)
}

func UpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	if r.Method == "POST" {
		name := r.FormValue("name")
		role := models.Role(r.FormValue("role"))
		sso_provider := r.FormValue("ssoprovider")
		email := r.FormValue("email")
		user := models.User{
			ID:          id,
			Name:        name,
			Role:        role,
			SSOProvider: sso_provider,
			Email:       email,
		}
		models.UpdateUser(user)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	item, err := models.GetUserByID(id)
	if err != nil {
		// handle error
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
	templates.ExecuteTemplate(w, "update.html", item)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	session.Options.MaxAge = -1 // This deletes the session
	err := session.Save(r, w)
	if err != nil {
		http.Error(w, "Unable to logout", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
