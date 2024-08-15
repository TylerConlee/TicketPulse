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

var googleOAuthConfig = &oauth2.Config{
	ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
	ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	RedirectURL:  "http://localhost:8080/auth/google/callback",
	Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
	Endpoint:     google.Endpoint,
}

var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

func init() {
	// Optional: Set some options on the session store
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
	}
}

func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
func GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	http.SetCookie(w, &http.Cookie{
		Name:  "oauthstate",
		Value: state,
	})
	url := googleOAuthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func GoogleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	oauthState, _ := r.Cookie("oauthstate")
	if r.FormValue("state") != oauthState.Value {
		log.Println("Invalid OAuth state")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	token, err := googleOAuthConfig.Exchange(context.Background(), r.FormValue("code"))
	if err != nil {
		log.Println("Could not get auth token:", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	client := googleOAuthConfig.Client(context.Background(), token)
	oauth2Service, err := oauth2api.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Println("Could not create OAuth2 service:", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	userinfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		log.Println("Could not get user info:", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Check if the user's email exists in the database
	user, err := models.GetUserByEmail(userinfo.Email)
	log.Println("Checking for user:", userinfo.Email) // Debugging line
	if err != nil {
		log.Println("Error querying user:", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// If the user was not found in the database, check if this is the first user
	if user.ID == 0 {
		userCount, err := models.GetUserCount()
		if err != nil {
			log.Println("Could not get user count:", err)
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		// If no users exist, create the first user as an admin
		if userCount == 0 {
			log.Println("Creating first user as admin:", userinfo.Email)
			err = models.CreateUser(userinfo.Email, userinfo.Name, models.AdminRole, false)
			if err != nil {
				log.Println("Could not create first user:", err)
				http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
				return
			}

			// Re-fetch the user data to ensure we have the correct role
			user, err = models.GetUserByEmail(userinfo.Email)
			if err != nil {
				log.Println("Error re-fetching user:", err)
				http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
				return
			}
		} else {
			log.Println("User not found, redirecting to unauthorized page.") // Debugging line
			http.Redirect(w, r, "/unauthorized", http.StatusTemporaryRedirect)
			return
		}
	}

	// Create a session and save user information
	session, _ := store.Get(r, "session-name")
	session.Values["user_id"] = user.ID
	session.Values["role"] = string(user.Role) // Ensure this is set as a string "admin" or "agent"

	// Log for debugging
	log.Printf("Setting role in session: %s\n", session.Values["role"])
	err = session.Save(r, w)
	if err != nil {
		log.Println("Error saving session:", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Redirect authenticated users to the dashboard
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
