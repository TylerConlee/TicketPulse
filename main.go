package main

import (
	"context"
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
	SlackService     *services.SlackService
	DashboardService *services.DashboardService
}

var Service *Services
var database db.Database

func main() {
	// Initialize the application
	initDatabase()
	loadEnvVariables()
	envCheck()

	// Initialize the SlackService and DashboardService before setting up routes
	startZenPollingChan := make(chan struct{})
	startSlackPollingChan := make(chan struct{})

	slackService, dashboardService := initializeServices(startZenPollingChan, startSlackPollingChan)
	Service = &Services{
		SlackService:     slackService,
		DashboardService: dashboardService,
	}

	// Set up the router
	r := setupRouter()

	// SSE endpoint
	r.Handle("/events", sseServer)

	// Start the HTTP server
	startServer(r, sseServer, startZenPollingChan, startSlackPollingChan)
}

func initDatabase() {
	// Assuming .env or another configuration method provides the DB filepath
	dbPath := os.Getenv("DB_FILEPATH")
	if dbPath == "" {
		log.Fatal("DB_FILEPATH environment variable is not set.")
	}
	database = db.InitDB(dbPath)
	models.SetDatabase(database)
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

func initializeServices(startZenPollingChan, startSlackPollingChan chan struct{}) (*services.SlackService, *services.DashboardService) {
	ctx := context.Background()
	// Periodically check configuration and start polling when ready
	go checkZenPolling(startZenPollingChan)
	go checkSlackPolling(startSlackPollingChan)

	// Wait for both channels to close
	<-startSlackPollingChan
	<-startZenPollingChan

	log.Println("Initializing services...")

	// Initialize SlackService
	slackService, err := services.NewSlackService(database, sseServer)
	if err != nil {
		log.Fatalf("Failed to initialize Slack service: %v", err)
	}
	// Start Slack Socket Mode
	go slackService.StartSocketMode()
	// Initialize DashboardService
	dashboardService := services.NewDashboardService(database)

	// Start Zendesk polling with the SlackService
	go services.StartZendeskPolling(ctx, database, sseServer, slackService) // <-- Start Zendesk polling here

	return slackService, dashboardService
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
		if err == nil {
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
		if value, err := models.GetConfiguration(database, key); err != nil || value == "" {
			return false
		}
	}
	botToken, err := models.GetConfiguration(database, "slack_bot_token")
	if err != nil || botToken == "" {
		return false
	}
	return true
}

func checkSlackConfig() error {
	botToken, err := models.GetConfiguration(database, "slack_bot_token")
	if err != nil || botToken == "" {
		return fmt.Errorf("Slack bot token is missing")
	}

	appToken, err := models.GetConfiguration(database, "slack_app_token")
	if err != nil || appToken == "" {
		return fmt.Errorf("Slack app token is missing")
	}

	return nil
}

func setupRouter() *mux.Router {
	r := mux.NewRouter()
	appHandler := handlers.NewAppHandler(database)
	// Public routes
	r.HandleFunc("/login", serveLoginPage).Methods("GET")
	r.HandleFunc("/auth/google/login", appHandler.GoogleLoginHandler).Methods("GET")
	r.HandleFunc("/auth/google/callback", appHandler.GoogleCallbackHandler).Methods("GET")
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
	appHandler := handlers.NewAppHandler(database)
	protected.HandleFunc("/dashboard", appHandler.DashboardHandler(Service.DashboardService)).Methods("GET")
	protected.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, Service.SlackService)
	}).Methods("GET", "POST")
	protected.HandleFunc("/profile/add-tag", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, Service.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/profile/delete-tag/{id}", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, Service.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/profile/update-summary-settings", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, Service.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/profile/update-profile", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, Service.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/profile/summary/now", func(w http.ResponseWriter, r *http.Request) {
		appHandler.OnDemandSummaryHandler(w, r, Service.SlackService)
	}).Methods("GET")

	protected.HandleFunc("/settings", appHandler.SettingsHandler).Methods("GET", "POST")
	protected.HandleFunc("/logout", appHandler.LogoutHandler).Methods("GET")

	return protected
}

func setupAdminRoutes(protected *mux.Router) {
	adminHandler := handlers.NewAppHandler(database)
	admin := protected.PathPrefix("/admin").Subrouter()
	admin.Use(handlers.AdminMiddleware)

	admin.HandleFunc("/users", adminHandler.UserManagementHandler).Methods("GET")
	admin.HandleFunc("/users/edit/{id}", adminHandler.EditUserHandler).Methods("GET", "POST")
	admin.HandleFunc("/users/delete/{id}", adminHandler.DeleteUserHandler).Methods("POST")
	admin.HandleFunc("/users/new", adminHandler.NewUserHandler).Methods("GET", "POST")
	admin.HandleFunc("/tags", adminHandler.TagManagementHandler).Methods("GET")
	admin.HandleFunc("/tag/delete/{id}", adminHandler.DeleteTagAlertHandler).Methods("POST")
	admin.HandleFunc("/configuration", adminHandler.ConfigurationHandler).Methods("GET", "POST")
}

func startServer(r *mux.Router, sseServer *middlewares.SSEServer, startZenPollingChan, startSlackPollingChan chan struct{}) {
	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal("Server failed:", err)
	}
}
