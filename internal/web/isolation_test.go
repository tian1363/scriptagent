package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
)

func TestPrivateAPIsDoNotExposeAnotherUsersResources(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "isolation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Config{RegistrationMode: "invite", InviteCodes: []string{"second-user"}}, store, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, nil, nil)
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	ownerClient := clientWithJar(t)
	owner := registerTestUser(t, ownerClient, server.URL, "owner@example.com", "")
	otherClient := clientWithJar(t)
	registerTestUser(t, otherClient, server.URL, "other@example.com", "second-user")

	markdownPath := filepath.Join(dir, "owner.md")
	assetPath := filepath.Join(dir, "owner.png")
	if err := os.WriteFile(markdownPath, []byte("owner only"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(assetPath, []byte("not-a-real-image"), 0o600); err != nil {
		t.Fatal(err)
	}
	product, _ := store.CreateProduct(jobs.CreateProductInput{Title: "owner-product", MDPath: markdownPath, MDName: "owner.md"})
	mustClaim(t, store, owner.ID, "product", product.ID)
	asset, _ := store.CreateProductAsset(jobs.ProductAsset{ProductID: product.ID, Kind: "image", Path: assetPath, OriginalName: "owner.png", MimeType: "image/png"})
	space, _ := store.CreateSpace(jobs.CreateSpaceInput{Title: "owner-space", ProductID: product.ID})
	mustClaim(t, store, owner.ID, "space", space.ID)
	job, _ := store.CreateJob(jobs.CreateJobInput{Title: "owner-job", Industry: "test", FissionCount: 1, SpaceID: space.ID})
	mustClaim(t, store, owner.ID, "job", job.ID)
	chat, _ := store.CreateChatConversationWithContext("owner-chat", space.ID, product.ID)
	mustClaim(t, store, owner.ID, "chat", chat.ID)
	skill, _ := store.CreateCustomSkill(jobs.CreateCustomSkillInput{Name: "owner-skill", Title: "Owner", Description: "private", Category: "test", InvocationPrompt: "use", Content: "# Owner"})
	mustClaim(t, store, owner.ID, "skill", skill.ID)
	video, _ := store.CreateVideoGeneration(jobs.CreateVideoGenerationInput{UserID: owner.ID, ProductID: product.ID, SpaceID: space.ID, ConversationID: chat.ID, Mode: "text", Prompt: "owner-video", Model: "wan3.0-video", Resolution: "720P", Ratio: "9:16", Duration: 5})

	for _, path := range []string{"/api/products", "/api/spaces", "/api/jobs", "/api/chats", "/api/videos", "/api/skills"} {
		response := doRequest(t, otherClient, http.MethodGet, server.URL+path, "")
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		for _, secret := range []string{product.ID, space.ID, job.ID, chat.ID, video.ID, skill.ID, "owner-product", "owner-space", "owner-job", "owner-chat"} {
			if strings.Contains(string(body), secret) {
				t.Fatalf("%s leaked another user's resource %q: %s", path, secret, body)
			}
		}
	}

	cases := []struct{ method, path, body string }{
		{http.MethodGet, "/api/products/" + product.ID + "/markdown", ""},
		{http.MethodGet, "/api/products/" + product.ID + "/assets", ""},
		{http.MethodGet, "/api/assets/" + asset.ID + "/file", ""},
		{http.MethodDelete, "/api/assets/" + asset.ID, ""},
		{http.MethodPut, "/api/products/" + product.ID, `{"title":"changed","content":"changed"}`},
		{http.MethodGet, "/api/spaces/" + space.ID + "/observability", ""},
		{http.MethodPut, "/api/spaces/" + space.ID, `{"title":"changed"}`},
		{http.MethodDelete, "/api/spaces/" + space.ID, ""},
		{http.MethodGet, "/api/jobs/" + job.ID, ""},
		{http.MethodPost, "/api/jobs/" + job.ID + "/retry", "{}"},
		{http.MethodGet, "/api/chats/" + chat.ID, ""},
		{http.MethodGet, "/api/chats/" + chat.ID + "/progress", ""},
		{http.MethodGet, "/api/videos/" + video.ID, ""},
		{http.MethodGet, "/api/videos/" + video.ID + "/file", ""},
		{http.MethodPut, "/api/skills/" + skill.ID, `{"name":"owner-skill","title":"Changed","description":"private","content":"# Changed"}`},
	}
	for _, test := range cases {
		response := doRequest(t, otherClient, test.method, server.URL+test.path, test.body)
		response.Body.Close()
		if response.StatusCode < 400 || response.StatusCode >= 500 {
			t.Fatalf("%s %s returned %d; expected a non-disclosing client error", test.method, test.path, response.StatusCode)
		}
	}
}

func clientWithJar(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func registerTestUser(t *testing.T, client *http.Client, baseURL, email, invite string) jobs.User {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"email": email, "password": "safe-password", "name": email, "invite_code": invite})
	response := doRequest(t, client, http.MethodPost, baseURL+"/api/auth/register", string(payload))
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("register %s failed: %d %s", email, response.StatusCode, body)
	}
	var user jobs.User
	if err := json.NewDecoder(response.Body).Decode(&user); err != nil {
		t.Fatal(err)
	}
	return user
}

func doRequest(t *testing.T, client *http.Client, method, url, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func mustClaim(t *testing.T, store *jobs.Store, userID, kind, id string) {
	t.Helper()
	if err := store.ClaimResource(userID, kind, id); err != nil {
		t.Fatal(err)
	}
}
