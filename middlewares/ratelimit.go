package middlewares

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type visitor struct {
	tokens    float64
	lastSeen  time.Time
	maxTokens float64
	rate      float64
}

// RateLimiter implements a per-IP token bucket rate limiter.
type RateLimiter struct {
	mu           sync.Mutex
	visitors     map[string]*visitor
	rate         float64 // tokens per second
	burst        float64 // max tokens
	cleanupAt    time.Time
	trustedCIDRs []*net.IPNet
}

// NewRateLimiter creates a rate limiter. rate is requests/second, burst is max burst.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    float64(burst),
	}
}

// SetTrustedProxies configures CIDR ranges that are trusted to set X-Forwarded-For.
func (rl *RateLimiter) SetTrustedProxies(cidrs []string) {
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		rl.trustedCIDRs = append(rl.trustedCIDRs, ipNet)
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	if now.After(rl.cleanupAt) {
		for k, v := range rl.visitors {
			if now.Sub(v.lastSeen) > 5*time.Minute {
				delete(rl.visitors, k)
			}
		}
		rl.cleanupAt = now.Add(time.Minute)
	}

	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{
			tokens:    rl.burst - 1,
			lastSeen:  now,
			maxTokens: rl.burst,
			rate:      rl.rate,
		}
		return true
	}

	elapsed := now.Sub(v.lastSeen).Seconds()
	v.tokens += elapsed * v.rate
	if v.tokens > v.maxTokens {
		v.tokens = v.maxTokens
	}
	v.lastSeen = now

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

// Middleware returns an HTTP middleware that applies rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.clientIP(r)

		if !rl.allow(ip) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) clientIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}

	if len(rl.trustedCIDRs) == 0 {
		return remoteHost
	}

	remoteIP := net.ParseIP(remoteHost)
	if remoteIP == nil {
		return remoteHost
	}

	trusted := false
	for _, cidr := range rl.trustedCIDRs {
		if cidr.Contains(remoteIP) {
			trusted = true
			break
		}
	}

	if !trusted {
		return remoteHost
	}

	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		clientIP := strings.TrimSpace(parts[0])
		if clientIP != "" {
			return clientIP
		}
	}

	return remoteHost
}
