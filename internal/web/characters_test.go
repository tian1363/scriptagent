package web

import (
	"bytes"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestCharacterLibraryAccountIsolation(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := NewHandler(Config{RegistrationMode: "invite", InviteCodes: []string{"second"}}, store, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, nil, nil)
	server := httptest.NewServer(h.Routes())
	defer server.Close()
	owner := clientWithJar(t)
	user := registerTestUser(t, owner, server.URL, "role1@example.com", "")
	other := clientWithJar(t)
	registerTestUser(t, other, server.URL, "role2@example.com", "second")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "My Character")
	part, _ := writer.CreateFormFile("image", "role.png")
	_ = png.Encode(part, image.NewRGBA(image.Rect(0, 0, 384, 384)))
	writer.Close()
	req, _ := http.NewRequest("POST", server.URL+"/api/characters", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := owner.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatal(resp.StatusCode)
	}
	rows, err := store.ListCharacters(user.ID)
	if err != nil || len(rows) != 1 {
		t.Fatal(err, rows)
	}
	id := rows[0].ID
	for _, path := range []string{"/api/characters/" + id + "/image"} {
		r := doRequest(t, other, "GET", server.URL+path, "")
		r.Body.Close()
		if r.StatusCode != 404 {
			t.Fatal("foreign image accessible")
		}
		r = doRequest(t, owner, "GET", server.URL+path, "")
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatal("own image missing")
		}
	}
	r := doRequest(t, other, "GET", server.URL+"/api/characters", "")
	data, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if strings.Contains(string(data), id) {
		t.Fatal("foreign role listed")
	}
	payload := `{"name":"new","reference_id":"` + id + `","controls":{"age":28,"size":"720*1280","seed":42}}`
	r = doRequest(t, other, "POST", server.URL+"/api/characters/generate", payload)
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatal("foreign reference not rejected")
	}
	r = doRequest(t, owner, "POST", server.URL+"/api/characters/generate", payload)
	r.Body.Close()
	if r.StatusCode != 400 {
		t.Fatal("missing image config not rejected")
	}
}
