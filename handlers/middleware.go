package handlers

import (
	"context"
	"log"
	"net/http"

	"github.com/TylerConlee/TicketPulse/models"
)

type contextKey string

const userIDKey contextKey = "user_id"

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Example: Get user ID from session or token
		session, err := store.Get(r, "session-name")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userID, ok := session.Values["user_id"].(int)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Add the user ID to the context
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(userIDKey).(int)
	return userID, ok
}
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "session-name")

		roleValue, exists := session.Values["role"]
		if !exists {
			log.Println("Role key does not exist in session")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userRole, ok := roleValue.(models.Role)
		if !ok {
			log.Printf("Role found, but could not be cast to models.Role: %v\n", roleValue)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if userRole != models.AdminRole {
			log.Println("User does not have admin privileges")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
