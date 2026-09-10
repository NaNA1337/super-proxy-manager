package auth

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NaNA1337/super-proxy-manager/backend/db"
	"golang.org/x/time/rate"
)

type contextKey string

const SessionContextKey contextKey = "auth_session"
const SessionCookieName = "super_proxy_session"

type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	lim := &IPRateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    b,
	}
	go lim.cleanup()
	return lim
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(i.rate, i.burst)
		i.visitors[ip] = limiter
	}
	return limiter
}

func (i *IPRateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		i.mu.Lock()
		i.visitors = make(map[string]*rate.Limiter)
		i.mu.Unlock()
	}
}

var (
	// Login rate limiter: 5 requests / min, burst 5 per IP
	LoginLimiter = NewIPRateLimiter(rate.Every(12*time.Second), 5)
	// API rate limiter: 60 req/min per IP
	APILimiter = NewIPRateLimiter(rate.Every(time.Second), 60)
)

func GetClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// SecurityHeadersMiddleware adds defensive HTTP headers
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// SessionMiddleware extracts and injects session into context
func SessionMiddleware(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string

			// Check Cookie first
			if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
				token = cookie.Value
			} else if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				// Fallback to Bearer token
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
					token = parts[1]
				}
			}

			if token != "" {
				if sess, ok := GetSession(database, token); ok {
					ctx := context.WithValue(r.Context(), SessionContextKey, sess)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth enforces authenticated session and checks forced password change
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := r.Context().Value(SessionContextKey).(*Session)
		if !ok || sess == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// If user must change password, block all routes except change-password, me, logout
		if sess.MustChangePassword {
			path := r.URL.Path
			allowed := path == "/api/auth/change-password" || path == "/api/auth/me" || path == "/api/auth/logout"
			if !allowed {
				http.Error(w, `{"error":"password_change_required"}`, http.StatusForbidden)
				return
			}
		}

		// Enforce CSRF on mutating requests
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			csrfHeader := r.Header.Get("X-CSRF-Token")
			if !ValidateCSRF(sess, csrfHeader) {
				http.Error(w, `{"error":"invalid csrf token"}`, http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// RequireAdmin enforces admin role and checks forced password change
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := r.Context().Value(SessionContextKey).(*Session)
		if !ok || sess == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if sess.MustChangePassword {
			path := r.URL.Path
			allowed := path == "/api/auth/change-password" || path == "/api/auth/me" || path == "/api/auth/logout"
			if !allowed {
				http.Error(w, `{"error":"password_change_required"}`, http.StatusForbidden)
				return
			}
		}
		if sess.Role != RoleAdmin {
			http.Error(w, `{"error":"forbidden: admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SetSessionCookie sets a secure, HttpOnly, SameSite=Strict cookie
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, isSecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie invalidates the session cookie
func ClearSessionCookie(w http.ResponseWriter, isSecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func SendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
