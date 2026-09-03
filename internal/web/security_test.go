package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/auth"
)

func TestSameOriginWritesRejectCrossSiteRequest(t *testing.T) {
	called := false
	handler := sameOriginWrites(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	req := httptest.NewRequest(http.MethodPost, "https://scriptagent.example/api/chats", strings.NewReader("{}"))
	req.Host = "scriptagent.example"
	req.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden || called {
		t.Fatalf("cross-site request was not rejected: code=%d called=%v", response.Code, called)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, name := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if response.Header().Get(name) == "" {
			t.Fatalf("missing security header %s", name)
		}
	}
}

func TestSecureSessionCookieCanBeForcedBehindProxy(t *testing.T) {
	handler := &Handler{cfg: Config{SecureCookies: true}}
	response := httptest.NewRecorder()
	handler.setSessionCookie(response, httptest.NewRequest(http.MethodPost, "http://internal/auth", nil), "token", time.Now().Add(time.Hour))
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != auth.CookieName() || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("unexpected session cookie: %+v", cookies)
	}
}

func TestRequestLimiterRejectsExcessAttempts(t *testing.T) {
	limiter := newRequestLimiter()
	if !limiter.allow("login:127.0.0.1", 2, time.Minute) || !limiter.allow("login:127.0.0.1", 2, time.Minute) {
		t.Fatal("allowed requests were rejected")
	}
	if limiter.allow("login:127.0.0.1", 2, time.Minute) {
		t.Fatal("excess request was allowed")
	}
}
