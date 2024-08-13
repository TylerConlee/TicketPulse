package main

import (
	"net/http"

	"github.com/tylerconlee/ticketpulse/db"
	"github.com/tylerconlee/ticketpulse/handlers"

	"github.com/gorilla/mux"
)

func main() {
	db.InitDB("users.db")

	r := mux.NewRouter()
	r.HandleFunc("/", handlers.HomeHandler).Methods("GET")
	r.HandleFunc("/auth/google/login", handlers.GoogleLoginHandler).Methods("GET")
	r.HandleFunc("/auth/google/callback", handlers.GoogleCallbackHandler).Methods("GET")

	// Add additional routes for CRUD operations and configuration...

	http.ListenAndServe(":8080", r)
}
