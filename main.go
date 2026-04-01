package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/handlers"
	"github.com/TylerConlee/TicketPulse/logging"
	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/hkdf"
)

// App holds all application-level dependencies, replacing package-level globals.
type App struct {
	DB               db.Database
	SSEServer        *middlewares.SSEServer
	ConfigCache      *services.ConfigCache
	SlackService     *services.SlackService
	DashboardService *services.DashboardService
	SchedulerService *services.SchedulerService
	configNotify     chan struct{}
}

func main() {
	loadEnvVariables()
	database := initDatabase()

	cleanupLogger := logging.InitLogger(os.Getenv("LOG_FORMAT"))
	defer cleanupLogger()
	logging.LoadFromEnv()

	enabledAreas := logging.EnabledAreas()
	if len(enabledAreas) > 0 {
		slog.Info("Debug logging enabled", "areas", enabledAreas)
	} else {
		slog.Info("Debug logging disabled. Set DEBUG_AREAS=all to enable verbose diagnostics.")
	}

	initSessionAndAuth()

	configCache := services.NewConfigCache(database, 5*time.Minute)

	app := &App{
		DB:           database,
		SSEServer:    middlewares.NewSSEServer(),
		ConfigCache:  configCache,
		configNotify: make(chan struct{}, 1),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startZenPollingChan := make(chan struct{})
	startSlackPollingChan := make(chan struct{})

	go app.checkZenPolling(startZenPollingChan)
	go app.checkSlackPolling(startSlackPollingChan)

	r := app.setupRouter()

	server := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Starting server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	go func() {
		<-startSlackPollingChan
		<-startZenPollingChan

		slog.Info("Initializing services...")

		slackService, err := services.NewSlackService(database, app.SSEServer)
		if err != nil {
			slog.Error("Failed to initialize Slack service", "error", err)
			return
		}

		zendeskClient, err := services.NewZendeskClient(database)
		if err != nil {
			slog.Error("Failed to initialize Zendesk client for Slack service", "error", err)
		} else {
			slackService.ZendeskClient = zendeskClient
		}

		app.SlackService = slackService
		app.DashboardService = services.NewDashboardService(database)
		app.SchedulerService = services.NewSchedulerService(database, slackService, configCache)

		go slackService.StartSocketMode(ctx)

		pollingService := services.NewPollingService(database, app.SSEServer, slackService, configCache)
		go pollingService.StartPolling(ctx)

		go app.SchedulerService.StartScheduler(ctx)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("Shutdown signal received", "signal", sig)

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	if err := database.Close(); err != nil {
		slog.Error("Error closing database", "error", err)
	}

	slog.Info("Server exited gracefully")
}

func initDatabase() db.Database {
	dbPath := os.Getenv("DB_FILEPATH")
	if dbPath == "" {
		log.Fatal("DB_FILEPATH environment variable is not set.")
	}
	database := db.InitDB(dbPath)
	models.SetDatabase(database)
	return database
}

func loadEnvVariables() {
	err := godotenv.Load()
	if err != nil {
		if os.Getenv("GOOGLE_CLIENT_ID") == "" || os.Getenv("GOOGLE_CLIENT_SECRET") == "" {
			log.Fatal("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set")
		}
	}
}

func initSessionAndAuth() {
	sessionKeyStr := os.Getenv("SESSION_KEY")
	var sessionKey []byte

	isProd := os.Getenv("GO_ENV") == "production"

	if sessionKeyStr == "" {
		if isProd {
			log.Fatal("SESSION_KEY must be set in production. Generate one with: openssl rand -base64 32")
		}
		sessionKey = make([]byte, 32)
		if _, err := rand.Read(sessionKey); err != nil {
			log.Fatal("Failed to generate session key:", err)
		}
		slog.Warn("SESSION_KEY not set: generated a random key. Sessions will not persist across restarts. Set SESSION_KEY in your .env for production use.")
	} else {
		decoded, err := base64.StdEncoding.DecodeString(sessionKeyStr)
		if err != nil {
			sessionKey = []byte(sessionKeyStr)
		} else {
			sessionKey = decoded
		}
		if len(sessionKey) < 32 {
			if isProd {
				log.Fatal("SESSION_KEY must be at least 32 bytes in production.")
			}
			slog.Warn("SESSION_KEY is shorter than 32 bytes. Use a longer key for production security.")
		}
	}

	appBaseURL := os.Getenv("BASE_URL")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:8080"
	}

	if isProd && !strings.HasPrefix(appBaseURL, "https://") {
		slog.Warn("BASE_URL does not use HTTPS. TLS termination via a reverse proxy is required for production.")
	}

	csrfKey := deriveKey(sessionKey, "ticketpulse-csrf-key")

	handlers.InitAuth(sessionKey, csrfKey, appBaseURL)
	models.InitEncryption(sessionKey)
}

func deriveKey(master []byte, info string) []byte {
	hkdfReader := hkdf.New(sha256.New, master, []byte("ticketpulse-key-derivation"), []byte(info))
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		log.Fatalf("Failed to derive key for %s: %v", info, err)
	}
	return key
}

func (app *App) notifyConfigChange() {
	select {
	case app.configNotify <- struct{}{}:
	default:
	}
}

func (app *App) checkZenPolling(startPollingChan chan struct{}) {
	for {
		if app.checkZendeskConfig() {
			close(startPollingChan)
			slog.Info("Zendesk configuration ready, starting polling")
			break
		}
		slog.Info("Waiting for complete Zendesk configuration...")
		select {
		case <-time.After(30 * time.Second):
		case <-app.configNotify:
		}
	}
}

func (app *App) checkSlackPolling(startPollingChan chan struct{}) {
	for {
		err := app.checkSlackConfig()
		if err == nil {
			close(startPollingChan)
			slog.Info("Slack configuration ready, starting polling")
			break
		}
		slog.Info("Waiting for complete Slack configuration...")
		select {
		case <-time.After(30 * time.Second):
		case <-app.configNotify:
		}
	}
}

func (app *App) checkZendeskConfig() bool {
	requiredConfigs := []string{"zendesk_subdomain", "zendesk_email", "zendesk_api_key"}
	for _, key := range requiredConfigs {
		value, err := app.ConfigCache.Get(key)
		if err != nil || value == "" {
			return false
		}
	}
	botToken, err := app.ConfigCache.Get("slack_bot_token")
	if err != nil || botToken == "" {
		return false
	}
	return true
}

func (app *App) checkSlackConfig() error {
	botToken, err := app.ConfigCache.Get("slack_bot_token")
	if err != nil || botToken == "" {
		return fmt.Errorf("slack bot token is missing")
	}

	appToken, err := app.ConfigCache.Get("slack_app_token")
	if err != nil || appToken == "" {
		return fmt.Errorf("slack app token is missing")
	}

	return nil
}

func (app *App) setupRouter() *mux.Router {
	r := mux.NewRouter()

	r.Use(middlewares.SecurityHeadersMiddleware)

	appHandler := handlers.NewAppHandler(app.DB, app.ConfigCache, app.notifyConfigChange)

	r.HandleFunc("/healthz", app.healthHandler).Methods("GET")
	r.HandleFunc("/readyz", app.readyHandler).Methods("GET")

	authLimiter := middlewares.NewRateLimiter(1, 10)
	if proxyCIDRs := os.Getenv("TRUSTED_PROXY_CIDRS"); proxyCIDRs != "" {
		authLimiter.SetTrustedProxies(strings.Split(proxyCIDRs, ","))
	}

	r.HandleFunc("/login", serveLoginPage).Methods("GET")
	r.Handle("/auth/google/login", authLimiter.Middleware(http.HandlerFunc(appHandler.GoogleLoginHandler))).Methods("GET")
	r.Handle("/auth/google/callback", authLimiter.Middleware(http.HandlerFunc(appHandler.GoogleCallbackHandler))).Methods("GET")
	r.HandleFunc("/unauthorized", serveUnauthorizedPage).Methods("GET")

	protected := app.setupProtectedRoutes(r, appHandler)
	app.setupAdminRoutes(protected, appHandler)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", noDirectoryListing(http.FileServer(http.Dir("static/")))))

	return r
}

func noDirectoryListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	err := app.DB.GetDB().Ping()
	status := map[string]string{"status": "ok"}
	if err != nil {
		slog.Error("Health check failed", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		status["status"] = "unhealthy"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (app *App) readyHandler(w http.ResponseWriter, r *http.Request) {
	dbOK := app.DB.GetDB().Ping() == nil
	zendeskOK := app.checkZendeskConfig()
	slackOK := app.checkSlackConfig() == nil

	ready := dbOK && zendeskOK && slackOK

	status := map[string]interface{}{
		"status": "ready",
	}

	if !ready {
		w.WriteHeader(http.StatusServiceUnavailable)
		status["status"] = "not_ready"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func serveLoginPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/login.html")
}

func serveUnauthorizedPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/unauthorized.html")
}

func trustedOrigins(baseURL string) []string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return nil
	}
	return []string{u.Host}
}

func (app *App) setupProtectedRoutes(r *mux.Router, appHandler *handlers.AppHandler) *mux.Router {
	csrfMiddleware := csrf.Protect(
		handlers.GetCSRFKey(),
		csrf.Secure(handlers.IsHTTPS()),
		csrf.Path("/"),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.TrustedOrigins(trustedOrigins(handlers.BaseURL())),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reason := csrf.FailureReason(r)
			log.Printf("CSRF validation failed: reason=%v method=%s path=%s", reason, r.Method, r.URL.Path)
			http.Error(w, fmt.Sprintf("CSRF error: %v", reason), http.StatusForbidden)
		})),
	)

	protected := r.PathPrefix("/").Subrouter()
	protected.Use(handlers.AuthMiddleware)
	protected.Use(csrfMiddleware)
	protected.Use(middlewares.SetupMiddleware(app.ConfigCache))
	protected.Use(middlewares.NotificationMiddleware)

	protected.Handle("/events", app.SSEServer)

	protected.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		appHandler.DashboardHandler(w, r, app.DashboardService)
	}).Methods("GET")
	protected.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, app.SlackService)
	}).Methods("GET", "POST")
	protected.HandleFunc("/profile/update-summary-settings", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, app.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/profile/update-work-day-settings", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, app.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/profile/update-summary-filters", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, app.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/profile/update-profile", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ProfileHandler(w, r, app.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/profile/summary/now", func(w http.ResponseWriter, r *http.Request) {
		appHandler.OnDemandSummaryHandler(w, r, app.SlackService)
	}).Methods("GET")

	protected.HandleFunc("/alerts", func(w http.ResponseWriter, r *http.Request) {
		appHandler.AlertsPageHandler(w, r, app.SlackService)
	}).Methods("GET")
	protected.HandleFunc("/alerts/add", func(w http.ResponseWriter, r *http.Request) {
		appHandler.AlertsAddHandler(w, r, app.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/alerts/update/{id}", func(w http.ResponseWriter, r *http.Request) {
		appHandler.AlertsUpdateHandler(w, r, app.SlackService)
	}).Methods("POST")
	protected.HandleFunc("/alerts/delete/{id}", func(w http.ResponseWriter, r *http.Request) {
		appHandler.AlertsDeleteHandler(w, r)
	}).Methods("POST")
	protected.HandleFunc("/alerts/bulk-delete", func(w http.ResponseWriter, r *http.Request) {
		appHandler.AlertsBulkDeleteHandler(w, r)
	}).Methods("POST")
	protected.HandleFunc("/alerts/tags", func(w http.ResponseWriter, r *http.Request) {
		appHandler.AlertsTagsJSONHandler(w, r)
	}).Methods("GET")
	protected.HandleFunc("/alerts/history", func(w http.ResponseWriter, r *http.Request) {
		appHandler.AlertsHistoryHandler(w, r)
	}).Methods("GET")

	protected.HandleFunc("/setup", appHandler.SetupHandler).Methods("GET", "POST")
	protected.HandleFunc("/setup/test-zendesk", appHandler.TestZendeskHandler).Methods("POST")
	protected.HandleFunc("/setup/test-slack", appHandler.TestSlackHandler).Methods("POST")

	protected.HandleFunc("/logout", appHandler.LogoutHandler).Methods("POST")

	return protected
}

func (app *App) setupAdminRoutes(protected *mux.Router, appHandler *handlers.AppHandler) {
	admin := protected.PathPrefix("/admin").Subrouter()
	admin.Use(handlers.AdminMiddlewareWithDB(app.DB))

	admin.HandleFunc("/users", appHandler.UserManagementHandler).Methods("GET")
	admin.HandleFunc("/users/edit/{id}", appHandler.EditUserHandler).Methods("GET", "POST")
	admin.HandleFunc("/users/delete/{id}", appHandler.DeleteUserHandler).Methods("POST")
	admin.HandleFunc("/users/new", appHandler.NewUserHandler).Methods("GET", "POST")
	admin.HandleFunc("/tags", appHandler.TagManagementHandler).Methods("GET")
	admin.HandleFunc("/tag/delete/{id}", appHandler.DeleteTagAlertHandler).Methods("POST")
	admin.HandleFunc("/configuration", func(w http.ResponseWriter, r *http.Request) {
		appHandler.ConfigurationHandler(w, r, app.SlackService)
	}).Methods("GET", "POST")
}
