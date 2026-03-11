package handlers

import (
	"errors"
	"html/template"
	"net/http"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/services"
	"github.com/gorilla/csrf"
)

// AppHandler struct will hold the database and service instances
type AppHandler struct {
	DB                 db.Database
	SlackService       services.SlackServiceInterface
	Zendesk            services.ZendeskClientInterface
	ConfigCache        *services.ConfigCache
	notifyConfigChange func()
}

// NewAppHandler initializes the AppHandler with a database
func NewAppHandler(db db.Database, configCache *services.ConfigCache, notifyFn func()) *AppHandler {
	return &AppHandler{
		DB:                 db,
		ConfigCache:        configCache,
		notifyConfigChange: notifyFn,
	}
}

// NewAppHandlerWithServices initializes the AppHandler with database and service interfaces.
func NewAppHandlerWithServices(db db.Database, slack services.SlackServiceInterface, zendesk services.ZendeskClientInterface) *AppHandler {
	return &AppHandler{
		DB:           db,
		SlackService: slack,
		Zendesk:      zendesk,
	}
}

// LogoutHandler logs the user out by clearing the session. Requires POST to prevent logout CSRF.
func (h *AppHandler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, _ := store.Get(r, "session-name")
	session.Options.MaxAge = -1
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
	if user.ID == 0 {
		return nil, errors.New("user not found in session")
	}
	firstUserID, err := models.GetFirstUserID(h.DB)
	if err != nil {
		return nil, err
	}
	notifications := middlewares.GetNotificationsFromContext(r)

	alertCount, _ := models.GetTagAlertCountByUser(h.DB, user.ID)

	data := map[string]interface{}{
		"Title":         title,
		"User":          user,
		"FirstUserID":   firstUserID,
		"Notifications": notifications,
		"CSRFField":     template.HTML(csrf.TemplateField(r)),
		"AlertCount":    alertCount,
		"CSPNonce":      middlewares.GetCSPNonce(r),
	}

	return data, nil
}
