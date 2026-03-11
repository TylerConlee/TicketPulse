package middlewares

import (
	"net/http"
	"strings"
)

// SetupChecker provides the interface to check if setup is complete.
type SetupChecker interface {
	Get(key string) (string, error)
}

// SetupMiddleware redirects to the setup wizard if initial setup hasn't been completed.
// It allows access to /setup, /logout, /events, and static assets without redirect.
func SetupMiddleware(checker SetupChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasPrefix(path, "/setup") ||
				strings.HasPrefix(path, "/static/") ||
				path == "/logout" ||
				path == "/events" ||
				path == "/healthz" ||
				path == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}

			val, err := checker.Get("setup_complete")
			if err != nil || val != "true" {
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
