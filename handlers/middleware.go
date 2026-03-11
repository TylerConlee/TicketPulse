package handlers

import (
	"context"
	"log"
	"net/http"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/models"
)

type contextKey string

const userIDKey contextKey = "user_id"

// AuthMiddleware validates session and stores userID in context.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "session-name")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userID, ok := session.Values["user_id"].(int)
		if !ok || userID == 0 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(userIDKey).(int)
	return userID, ok
}

// AdminMiddleware checks that the user has admin role by verifying against the database.
func AdminMiddleware(next http.Handler) http.Handler {
	return AdminMiddlewareWithDB(nil)(next)
}

// AdminMiddlewareWithDB returns an admin middleware that re-validates the role from the database.
func AdminMiddlewareWithDB(database db.Database) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := GetUserIDFromContext(r.Context())

			if database != nil {
				if !ok {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				user, err := models.GetUserByID(database, userID)
				if err != nil {
					log.Printf("AdminMiddleware: failed to look up user %d: %v", userID, err)
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				if user.Role != models.AdminRole {
					log.Printf("AdminMiddleware: user %d is not admin (role=%s)", userID, user.Role)
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			} else {
				session, err := store.Get(r, "session-name")
				if err != nil {
					log.Println("AdminMiddleware: failed to get session:", err)
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}

				roleValue, exists := session.Values["role"]
				if !exists {
					log.Println("Role key does not exist in session")
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}

				var userRole models.Role
				switch v := roleValue.(type) {
				case models.Role:
					userRole = v
				case string:
					userRole = models.Role(v)
				default:
					log.Printf("Role found, but unexpected type %T: %v\n", roleValue, roleValue)
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				if userRole != models.AdminRole {
					log.Println("User does not have admin privileges")
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// LoginHandler serves the login page, redirecting already-authenticated users.
func (h *AppHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	if _, ok := session.Values["user_id"].(int); ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	http.ServeFile(w, r, "templates/login.html")
}
