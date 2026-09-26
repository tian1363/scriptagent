package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	chatpkg "github.com/tian1363/scriptagent/internal/chat"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
	"github.com/tian1363/scriptagent/internal/userctx"
)

type toolboxStub struct {
	ChatResponder
	user  string
	calls int
}

func (s *toolboxStub) GenerateToolboxDraft(ctx context.Context, in chatpkg.ToolboxDraftInput) (*chatpkg.ToolboxDraft, error) {
	s.user = userctx.UserID(ctx)
	s.calls++
	return &chatpkg.ToolboxDraft{Candidates: []chatpkg.ToolboxCandidate{{Title: "草稿", Text: "测试文案"}}}, nil
}
func TestToolboxRouteRequiresAuthAndRejectsInvalidInput(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stub := &toolboxStub{}
	h := NewHandler(Config{}, store, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, stub, nil)
	server := httptest.NewServer(h.Routes())
	defer server.Close()
	body := `{"mode":"copy","action":"rewrite","text":"现有文案"}`
	r := doRequest(t, http.DefaultClient, http.MethodPost, server.URL+"/api/toolbox/draft", body)
	r.Body.Close()
	if r.StatusCode != http.StatusUnauthorized || stub.calls != 0 {
		t.Fatal("unauthenticated request reached AI")
	}
	client := clientWithJar(t)
	user := registerTestUser(t, client, server.URL, "toolbox@example.com", "")
	for _, bad := range []string{`{}`, body + ` {}`, `{"mode":"copy","action":"rewrite","text":"x","user_id":"other"}`, strings.Repeat("x", 66000)} {
		r = doRequest(t, client, http.MethodPost, server.URL+"/api/toolbox/draft", bad)
		r.Body.Close()
		if r.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid input returned %d", r.StatusCode)
		}
	}
	r = doRequest(t, client, http.MethodPost, server.URL+"/api/toolbox/draft", body)
	r.Body.Close()
	if r.StatusCode != http.StatusOK || stub.user != user.ID || stub.calls != 1 {
		t.Fatalf("status=%d stub=%+v", r.StatusCode, stub)
	}
	for i := 0; i < 9; i++ {
		r = doRequest(t, client, http.MethodPost, server.URL+"/api/toolbox/draft", body)
		r.Body.Close()
	}
	r = doRequest(t, client, http.MethodPost, server.URL+"/api/toolbox/draft", body)
	r.Body.Close()
	if r.StatusCode != http.StatusTooManyRequests || stub.calls != 10 {
		t.Fatal("AI request limiter did not apply")
	}
}
