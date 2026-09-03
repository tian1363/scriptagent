package web

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type limitWindow struct {
	count int
	reset time.Time
}

type requestLimiter struct {
	mu      sync.Mutex
	windows map[string]limitWindow
	calls   uint64
}

func newRequestLimiter() *requestLimiter {
	return &requestLimiter{windows: map[string]limitWindow{}}
}

func (l *requestLimiter) allow(key string, max int, window time.Duration) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	if l.calls%1024 == 0 {
		for candidate, value := range l.windows {
			if value.reset.Before(now) {
				delete(l.windows, candidate)
			}
		}
	}
	current := l.windows[key]
	if current.reset.Before(now) {
		current = limitWindow{reset: now.Add(window)}
	}
	if current.count >= max {
		return false
	}
	current.count++
	l.windows[key] = current
	return true
}

func (h *Handler) allowAuthRequest(w http.ResponseWriter, r *http.Request, action string, max int, window time.Duration) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if h.authLimit.allow(action+":"+host, max, window) {
		return true
	}
	w.Header().Set("Retry-After", "900")
	writeError(w, http.StatusTooManyRequests, &publicError{"请求过于频繁，请稍后再试"})
	return false
}

type publicError struct{ message string }

func (e *publicError) Error() string { return e.message }

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func sameOriginWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if raw := strings.TrimSpace(r.Header.Get("Origin")); raw != "" {
				origin, err := url.Parse(raw)
				if err != nil || !strings.EqualFold(origin.Host, r.Host) {
					http.Error(w, `{"error":"请求来源无效"}`, http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
