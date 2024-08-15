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

type Services struct {
	SlackService *services.SlackService
}

var Service *Services

func main() {
	// Initialize the application
	initDatabase()
	loadEnvVariables()
	envCheck()

	r := setupRouter()

	// SSE endpoint
	r.Handle("/events", sseServer)

	// Use a Go channel to control when to start polling
	startZenPollingChan := make(chan struct{})

	startSlackPollingChan := make(chan struct{})

	// Periodically check configuration and start polling when ready
	go checkZenPolling(startZenPollingChan)

	go checkSlackPolling(startSlackPollingChan)

	// Start the HTTP server
	startServer(r, sseServer, startZenPollingChan, startSlackPollingChan)
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

func checkZenPolling(startPollingChan chan struct{}) {
	for {
		if checkZendeskConfig() {
			close(startPollingChan)
			log.Println("Polling channel closed, starting Zendesk polling")

			break
		}
		log.Println("Waiting for complete configuration...")
		time.Sleep(30 * time.Second)
	}
}

func checkSlackPolling(startPollingChan chan struct{}) {
	for {
		err := checkSlackConfig()
		if err == nil { // Only close the channel when config is complete and valid
			close(startPollingChan)
			log.Println("Polling channel closed, starting Slack polling")
			break
		}
		log.Println("Waiting for complete Slack configuration...")
		time.Sleep(30 * time.Second)
	}
}

func checkZendeskConfig() bool {
	requiredConfigs := []string{"zendesk_subdomain", "zendesk_email", "zendesk_api_key"}
	for _, key := range requiredConfigs {
		if value, err := models.GetConfiguration(key); err != nil || value == "" {
			return false
		}
	}
	return true
}
func checkSlackConfig() error {
	botToken, err := models.GetConfiguration("slack_bot_token")
	if err != nil || botToken == "" {
		return fmt.Errorf("Slack bot token is missing")
	}

	appToken, err := models.GetConfiguration("slack_app_token")
	if err != nil || appToken == "" {
		return fmt.Errorf("Slack app token is missing")
	}

	return nil
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
	protected.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		handlers.ProfileHandler(w, r, Service.SlackService)
	}).Methods("GET", "POST")
	protected.HandleFunc("/profile/add-tag", func(w http.ResponseWriter, r *http.Request) {
		handlers.ProfileHandler(w, r, Service.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/profile/delete-tag/{id}", func(w http.ResponseWriter, r *http.Request) {
		handlers.ProfileHandler(w, r, Service.SlackService)
	}).Methods("POST")
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

func startServer(r *mux.Router, sseServer *middlewares.SSEServer, startZenPollingChan, startSlackPollingChan chan struct{}) {
	// Wait for both polling channels to close
	go func() {
		<-startSlackPollingChan
		<-startZenPollingChan

		// Initialize SlackService
		slackService, err := services.NewSlackService()
		if err != nil {
			log.Fatalf("Failed to initialize Slack service: %v", err)
		}

		// Start Zendesk polling with the SlackService
		go services.StartZendeskPolling(sseServer, slackService)

		// Start Slack Socket Mode
		go slackService.StartSocketMode()
	}()

	fmt.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal("Server failed:", err)
	}
}
