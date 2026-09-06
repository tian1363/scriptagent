package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/auth"
	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestOnlyBoundAdministratorCanAccessOperations(t *testing.T) {
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner, err := store.CreateUser(jobs.CreateUserInput{Email: "owner@example.com", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(jobs.CreateUserInput{Email: "other@example.com", Role: "admin", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if owner.Role != "member" {
		t.Fatal("store default role is not member")
	}
	if err := store.SetSoleAdministrator(owner.ID); err != nil {
		t.Fatal(err)
	}
	demoted, err := store.GetUser(other.ID)
	if err != nil || demoted.Role != "member" {
		t.Fatalf("old admin not demoted: %v", err)
	}
	for _, u := range []*jobs.User{owner, other} {
		if _, err := store.CreateSession(jobs.CreateSessionInput{Token: u.ID, UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(Config{AdminUserID: owner.ID, RegistrationMode: "open"}, store, nil, nil, nil, nil, nil)
	for _, route := range []struct{ method, path string }{{"GET", "/api/owner/invites"}, {"POST", "/api/owner/invites"}, {"POST", "/api/owner/invites/test/revoke"}} {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: other.ID})
		out := httptest.NewRecorder()
		h.Routes().ServeHTTP(out, req)
		if out.Code != 403 {
			t.Fatalf("member reached invite management: %d", out.Code)
		}
	}
	for _, test := range []struct {
		token string
		want  int
	}{{"", 401}, {other.ID, 403}, {owner.ID, 200}} {
		req := httptest.NewRequest("GET", "/api/owner/overview", nil)
		if test.token != "" {
			req.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: test.token})
		}
		out := httptest.NewRecorder()
		h.Routes().ServeHTTP(out, req)
		if out.Code != test.want {
			t.Fatalf("got %d want %d: %s", out.Code, test.want, out.Body.String())
		}
	}
	// Even a stale/admin role cannot bypass the server-bound account identity.
	if err := store.SetSoleAdministrator(other.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{owner.ID, other.ID} {
		req := httptest.NewRequest("GET", "/api/owner/overview", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: id})
		out := httptest.NewRecorder()
		h.Routes().ServeHTTP(out, req)
		if out.Code != 403 {
			t.Fatalf("stale or unbound admin granted access: %d", out.Code)
		}
	}
	req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"email":"new@example.com","password":"safe-password","role":"admin"}`))
	out := httptest.NewRecorder()
	h.Routes().ServeHTTP(out, req)
	if out.Code != 201 || !strings.Contains(out.Body.String(), `"role":"member"`) {
		t.Fatalf("registration role injection: %s", out.Body.String())
	}
	if err := store.SetSoleAdministrator(""); err != nil {
		t.Fatal(err)
	}
	users, _ := store.ListUsers()
	for _, u := range users {
		if u.Role == "admin" {
			t.Fatal("unconfigured administrator remained")
		}
	}
}
