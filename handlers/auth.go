package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

var (
	googleOAuthConfig *oauth2.Config
	store             *sessions.CookieStore
	baseURL           string
	csrfKey           []byte
)

func InitAuth(sessionKey []byte, csrfAuthKey []byte, appBaseURL string) {
	baseURL = strings.TrimRight(appBaseURL, "/")
	isHTTPS := strings.HasPrefix(baseURL, "https://")

	store = sessions.NewCookieStore(sessionKey)
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   isHTTPS,
		SameSite: http.SameSiteLaxMode,
	}

	csrfKey = csrfAuthKey

	googleOAuthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  baseURL + "/auth/google/callback",
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}
}

func GetCSRFKey() []byte {
	return csrfKey
}

func IsHTTPS() bool {
	return strings.HasPrefix(baseURL, "https://")
}

func BaseURL() string {
	return baseURL
}

func (h *AppHandler) GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	state := randomState()

	// Delete any stale oauthstate cookie that may exist on the legacy path
	// (/auth/google/) from before the rewrite. Without this, the browser
	// keeps both cookies and the more-specific path wins on readback,
	// causing a state mismatch every time.
	http.SetCookie(w, &http.Cookie{
		Name:     "oauthstate",
		Value:    "",
		MaxAge:   -1,
		Path:     "/auth/google/",
		HttpOnly: true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "oauthstate",
		Value:    state,
		MaxAge:   300,
		HttpOnly: true,
		Secure:   IsHTTPS(),
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	url := googleOAuthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *AppHandler) GoogleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if err := validateOAuthState(r); err != nil {
		log.Println("Invalid OAuth state:", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	userInfo, err := fetchGoogleUserInfo(r.FormValue("code"))
	if err != nil {
		log.Println("Error fetching user info:", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	user, err := h.getOrCreateUser(userInfo)
	if err != nil {
		if errors.Is(err, ErrUnauthorizedUser) {
			log.Printf("Unauthorized login attempt: %s", userInfo.Email)
			http.Redirect(w, r, "/unauthorized", http.StatusSeeOther)
			return
		}
		log.Println("Error handling user:", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	if err := createSession(w, r, user); err != nil {
		log.Println("Error creating session:", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func validateOAuthState(r *http.Request) error {
	oauthState, err := r.Cookie("oauthstate")
	if err != nil {
		return fmt.Errorf("missing OAuth state cookie: %w", err)
	}
	if r.FormValue("state") != oauthState.Value {
		return fmt.Errorf("OAuth state mismatch: possible CSRF attack")
	}
	return nil
}

func fetchGoogleUserInfo(code string) (*oauth2api.Userinfo, error) {
	token, err := googleOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		return nil, err
	}

	client := googleOAuthConfig.Client(context.Background(), token)
	oauth2Service, err := oauth2api.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return oauth2Service.Userinfo.Get().Do()
}

var ErrUnauthorizedUser = errors.New("user is not authorized to access this application")

func (h *AppHandler) getOrCreateUser(userInfo *oauth2api.Userinfo) (models.User, error) {
	user, err := models.GetUserByEmail(h.DB, userInfo.Email)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, models.ErrUserNotFound) {
		return models.User{}, err
	}

	userCount, err := models.GetUserCount(h.DB)
	if err != nil {
		return models.User{}, err
	}

	if userCount == 0 {
		log.Println("Creating first user as admin:", userInfo.Email)
		if err := models.CreateUser(h.DB, userInfo.Email, userInfo.Name, models.AdminRole, false); err != nil {
			return models.User{}, err
		}

		user, err = models.GetUserByEmail(h.DB, userInfo.Email)
		if err != nil {
			return models.User{}, err
		}
		return user, nil
	}

	return models.User{}, ErrUnauthorizedUser
}

func createSession(w http.ResponseWriter, r *http.Request, user models.User) error {
	// Invalidate any existing session to prevent session fixation
	if oldSession, err := store.Get(r, "session-name"); err == nil {
		oldSession.Options.MaxAge = -1
		_ = oldSession.Save(r, w)
	}

	session, _ := store.New(r, "session-name")
	session.Values["user_id"] = user.ID
	session.Values["user_email"] = user.Email
	session.Values["role"] = string(user.Role)

	return session.Save(r, w)
}
