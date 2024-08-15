package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/handlers"
	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var sseServer = middlewares.NewSSEServer()

func main() {
	// Initialize the application
	initDatabase()
	loadEnvVariables()
	envCheck()

	r := setupRouter()

	// SSE endpoint
	r.Handle("/events", sseServer)

	// Use a Go channel to control when to start polling
	startPollingChan := make(chan struct{})

	// Periodically check configuration and start polling when ready
	go checkConfigAndStartPolling(startPollingChan)

	// Start the HTTP server
	startServer(r, sseServer, startPollingChan)
}

func initDatabase() {
	db.InitDB("users.db")
}

func loadEnvVariables() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func envCheck() {
	if os.Getenv("GOOGLE_CLIENT_ID") == "" || os.Getenv("GOOGLE_CLIENT_SECRET") == "" {
		log.Fatal("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set")
	}
}

func checkConfigAndStartPolling(startPollingChan chan struct{}) {
	for {
		if configComplete() {
			close(startPollingChan)
			log.Println("Polling channel closed, starting Zendesk polling")

			break
		}
		log.Println("Waiting for complete configuration...")
		time.Sleep(30 * time.Second)
	}
}

func configComplete() bool {
	requiredConfigs := []string{"zendesk_subdomain", "zendesk_email", "zendesk_api_key", "slack_signing_secret"}
	for _, key := range requiredConfigs {
		if value, err := models.GetConfiguration(key); err != nil || value == "" {
			return false
		}
	}
	return true
}

func setupRouter() *mux.Router {
	r := mux.NewRouter()

	// Public routes
	r.HandleFunc("/login", serveLoginPage).Methods("GET")
	r.HandleFunc("/auth/google/login", handlers.GoogleLoginHandler).Methods("GET")
	r.HandleFunc("/auth/google/callback", handlers.GoogleCallbackHandler).Methods("GET")
	r.HandleFunc("/unauthorized", serveUnauthorizedPage).Methods("GET")

	// Protected routes
	protected := setupProtectedRoutes(r)

	// Admin routes
	setupAdminRoutes(protected)

	// Static file server
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	return r
}

func serveLoginPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/login.html")
}

func serveUnauthorizedPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/unauthorized.html")
}

func setupProtectedRoutes(r *mux.Router) *mux.Router {
	protected := r.PathPrefix("/").Subrouter()
	protected.Use(handlers.AuthMiddleware)
	protected.Use(middlewares.NotificationMiddleware)

	protected.HandleFunc("/dashboard", handlers.DashboardHandler).Methods("GET")
	protected.HandleFunc("/profile", handlers.ProfileHandler).Methods("GET", "POST")
	protected.HandleFunc("/profile/add-tag", handlers.ProfileHandler).Methods("POST")
	protected.HandleFunc("/profile/delete-tag/{id}", handlers.ProfileHandler).Methods("POST")
	protected.HandleFunc("/settings", handlers.SettingsHandler).Methods("GET", "POST")
	protected.HandleFunc("/logout", handlers.LogoutHandler).Methods("GET")

	return protected
}

func setupAdminRoutes(protected *mux.Router) {
	admin := protected.PathPrefix("/admin").Subrouter()
	admin.Use(handlers.AdminMiddleware)

	admin.HandleFunc("/users", handlers.UserManagementHandler).Methods("GET")
	admin.HandleFunc("/users/edit/{id}", handlers.EditUserHandler).Methods("GET", "POST")
	admin.HandleFunc("/users/delete/{id}", handlers.DeleteUserHandler).Methods("POST")
	admin.HandleFunc("/users/new", handlers.NewUserHandler).Methods("GET", "POST")
	admin.HandleFunc("/tags", handlers.TagManagementHandler).Methods("GET")
	admin.HandleFunc("/tag/delete/{id}", handlers.DeleteTagAlertHandler).Methods("POST")
	admin.HandleFunc("/configuration", handlers.ConfigurationHandler).Methods("GET", "POST")
}

func startServer(r *mux.Router, sseServer *middlewares.SSEServer, startPollingChan chan struct{}) {
	// Notify and start polling if the configuration is complete
	go func() {
		<-startPollingChan
		time.Sleep(10 * time.Second) // Give the server time to start before polling
		fmt.Println("Zendesk configuration polling has started.")

		services.StartZendeskPolling(sseServer) // Start polling once the channel is closed
	}()

	fmt.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal("Server failed:", err)
	}
}
