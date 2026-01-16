package handlers

import (
	"context"
	"log"
	"net/http"

	"github.com/TylerConlee/TicketPulse/models"
)

type contextKey string

const userIDKey contextKey = "user_id"

// AuthMiddleware is a package-level function for backward compatibility.
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

// AuthMiddleware is a method version for handler-based middleware chaining.
func (h *AppHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// AdminMiddleware is a package-level function for backward compatibility.
func AdminMiddleware(next http.Handler) http.Handler {
	return adminMiddlewareHandler(next)
}

// AdminMiddleware is a method version for handler-based middleware chaining.
func (h *AppHandler) AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "session-name")

		userID, ok := session.Values["user_id"].(int)
		if !ok {
			log.Println("User ID key does not exist in session")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Fetch user from database to check role
		user, err := models.GetUserByID(h.DB, userID)
		if err != nil {
			log.Printf("Error fetching user: %v\n", err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if user.Role != models.AdminRole {
			log.Println("User does not have admin privileges")
			http.Redirect(w, r, "/unauthorized", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// adminMiddlewareHandler is the original implementation for the package-level function.
func adminMiddlewareHandler(next http.Handler) http.Handler {
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

// LoginHandler serves the login page.
func (h *AppHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	session, _ := store.Get(r, "session-name")
	if _, ok := session.Values["user_id"].(int); ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	// Render login template
	http.ServeFile(w, r, "templates/login.html")
}
