package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"os"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/gorilla/sessions"
	_ "github.com/joho/godotenv/autoload"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

var (
	googleOAuthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}
	store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))
)

func init() {
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
	}
}

func (h *AppHandler) GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	http.SetCookie(w, &http.Cookie{
		Name:  "oauthstate",
		Value: state,
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

// Helper functions
func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func validateOAuthState(r *http.Request) error {
	oauthState, err := r.Cookie("oauthstate")
	if err != nil || r.FormValue("state") != oauthState.Value {
		return err
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

func (h *AppHandler) getOrCreateUser(userInfo *oauth2api.Userinfo) (models.User, error) {
	user, err := models.GetUserByEmail(h.DB, userInfo.Email)
	if err != nil {
		return models.User{}, err
	}

	if user.ID == 0 {
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
		} else {
			return models.User{}, nil
		}
	}

	return user, nil
}

func createSession(w http.ResponseWriter, r *http.Request, user models.User) error {
	session, _ := store.Get(r, "session-name")
	session.Values["user_id"] = user.ID
	session.Values["user_email"] = user.Email
	session.Values["role"] = user.Role // Store as models.Role type

	return session.Save(r, w)
}
