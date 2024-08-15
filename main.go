package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/handlers"
	"github.com/TylerConlee/TicketPulse/services"
	"github.com/joho/godotenv"

	"github.com/gorilla/mux"
)

func main() {
	db.InitDB("users.db")
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	envCheck()

	r := mux.NewRouter()
	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "templates/login.html")
	}).Methods("GET")
	r.HandleFunc("/auth/google/login", handlers.GoogleLoginHandler).Methods("GET")
	r.HandleFunc("/auth/google/callback", handlers.GoogleCallbackHandler).Methods("GET")
	r.HandleFunc("/unauthorized", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "templates/unauthorized.html")
	}).Methods("GET")

	// Private routes
	protected := r.PathPrefix("/").Subrouter()
	protected.Use(handlers.AuthMiddleware)
	protected.HandleFunc("/dashboard", handlers.DashboardHandler).Methods("GET")
	protected.HandleFunc("/profile", handlers.ProfileHandler).Methods("GET", "POST")
	protected.HandleFunc("/profile/add-tag", handlers.ProfileHandler).Methods("POST")
	protected.HandleFunc("/profile/delete-tag/{id}", handlers.ProfileHandler).Methods("POST")

	protected.HandleFunc("/settings", handlers.SettingsHandler).Methods("GET", "POST")
	protected.HandleFunc("/logout", handlers.LogoutHandler).Methods("GET")

	// Admin routes
	admin := protected.PathPrefix("/admin").Subrouter()
	admin.Use(handlers.AdminMiddleware)
	admin.HandleFunc("/users", handlers.UserManagementHandler).Methods("GET")
	admin.HandleFunc("/users/edit/{id}", handlers.EditUserHandler).Methods("GET", "POST")
	admin.HandleFunc("/users/delete/{id}", handlers.DeleteUserHandler).Methods("POST")
	admin.HandleFunc("/users/new", handlers.NewUserHandler).Methods("GET", "POST")
	admin.HandleFunc("/tags", handlers.TagManagementHandler).Methods("GET")
	admin.HandleFunc("/tag/delete/{id}", handlers.DeleteTagAlertHandler).Methods("POST")

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	zendeskClient := services.NewZendeskClient("your_subdomain", "your_email", "your_api_token")

	go func() {
		for {
			tickets, err := zendeskClient.GetTickets()
			if err != nil {
				fmt.Println("Error fetching tickets:", err)
				continue
			}

			// Process tickets, check SLAs, send Slack notifications...
			fmt.Println("Fetched tickets:", tickets)

			time.Sleep(5 * time.Minute) // 5-minute polling interval
		}
	}()

	http.ListenAndServe(":8080", r)
}

func envCheck() {
	if os.Getenv("GOOGLE_CLIENT_ID") == "" || os.Getenv("GOOGLE_CLIENT_SECRET") == "" {
		log.Fatal("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set")
	}
}
