package middlewares

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
)

type cspNonceKey struct{}

// GenerateCSPNonce creates a cryptographically random nonce for CSP.
func GenerateCSPNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// GetCSPNonce retrieves the CSP nonce from the request context.
func GetCSPNonce(r *http.Request) string {
	if nonce, ok := r.Context().Value(cspNonceKey{}).(string); ok {
		return nonce
	}
	return ""
}

// SecurityHeadersMiddleware adds security headers to all responses.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := GenerateCSPNonce()
		ctx := context.WithValue(r.Context(), cspNonceKey{}, nonce)
		r = r.WithContext(ctx)

		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			fmt.Sprintf(
				"default-src 'self'; "+
					"script-src 'self' 'nonce-%s' https://cdn.tailwindcss.com https://cdn.jsdelivr.net; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: https://singlecolorimage.com; "+
					"connect-src 'self'; "+
					"font-src 'self'; "+
					"form-action 'self' https://accounts.google.com; "+
					"frame-ancestors 'none'",
				nonce))
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		next.ServeHTTP(w, r)
	})
}
