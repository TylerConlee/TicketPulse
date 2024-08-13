package main

import (
	"net/http"
	"strconv"
	"text/template"

	"github.com/TylerConlee/TicketPulse/models"

	"github.com/gorilla/mux"
)

func StartServer() {
	templates = template.Must(template.ParseGlob("templates/*.html"))

	r := mux.NewRouter()
	r.HandleFunc("/", IndexHandler).Methods("GET")
	r.HandleFunc("/users/create", CreateHandler).Methods("GET", "POST")
	r.HandleFunc("/users/update/{id}", UpdateHandler).Methods("GET", "POST")
	r.HandleFunc("/users/delete/{id}", DeleteHandler).Methods("POST")

	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}

// IndexHandler lists all users
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	var users []models.User
	db.Find(&users)
	templates.ExecuteTemplate(w, "index.html", users)
}

// CreateHandler creates a new item
func CreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		user := models.User{
			Name:    r.FormValue("name"),
			SlackID: r.FormValue("slackID"),
			Email:   r.FormValue("email"),
		}
		db.Create(&user)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	templates.ExecuteTemplate(w, "create.html", nil)
}

// UpdateHandler updates an existing item
func UpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	var user models.User
	db.First(&user, id)

	if r.Method == http.MethodPost {
		user.Name = r.FormValue("name")
		user.SlackID = r.FormValue("slackID")
		user.Email = r.FormValue("slackID")
		db.Save(&user)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	templates.ExecuteTemplate(w, "update.html", user)
}

// DeleteHandler deletes an item
func DeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	db.Delete(&models.User{}, id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func parseFloat(value string) float64 {
	result, _ := strconv.ParseFloat(value, 64)
	return result
}
