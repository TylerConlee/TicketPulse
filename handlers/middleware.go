package handlers

import (
	"log"
	"net/http"

	"github.com/TylerConlee/TicketPulse/models"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "session-name")
		_, ok := session.Values["user_id"]
		if !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "session-name")
		log.Printf("Session Data in AdminMiddleware: %+v\n", session.Values)
		// Retrieve and log the role type
		roleValue, exists := session.Values["role"]
		if !exists {
			log.Println("Role key does not exist in session")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Printf("Role value type: %T\n", roleValue)

		// Handle role as models.Role type
		userRole, ok := roleValue.(models.Role)
		if !ok {
			log.Printf("Role found, but could not be cast to models.Role: %v\n", roleValue)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Printf("Retrieved Role in AdminMiddleware: %s\n", userRole)

		// Check for admin privileges
		if userRole != models.AdminRole {
			log.Println("User does not have admin privileges")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
