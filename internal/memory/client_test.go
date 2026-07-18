package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlatformSearchAndAdd(t *testing.T) {
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Token m0-test" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		if r.URL.Path == "/v3/memories/search/" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			filters := body["filters"].(map[string]any)
			if filters["user_id"] != "user-a" {
				t.Fatalf("unexpected filters: %+v", filters)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"id": "m1", "memory": "用户偏好快节奏脚本", "score": 0.9}},
			})
			return
		}
		if r.URL.Path == "/v3/memories/add/" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["user_id"] != "user-a" || body["agent_id"] != "scriptagent" || body["run_id"] != "chat-1" {
				t.Fatalf("unexpected memory scope: %+v", body)
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"PENDING"}`))
	}))
	defer server.Close()

	client := NewClient(Config{Provider: "platform", BaseURL: server.URL, APIKey: "m0-test"})
	results, err := client.Search(context.Background(), "user-a", "脚本节奏")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Memory != "用户偏好快节奏脚本" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if err := client.Add(context.Background(), "user-a", "chat-1", []Message{{Role: "user", Content: "节奏快一点"}}, map[string]any{"source": "chat"}); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[1] != "/v3/memories/add/" {
		t.Fatalf("unexpected requests: %+v", requests)
	}
}

func TestOSSPaths(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer server.Close()

	client := NewClient(Config{Provider: "oss", BaseURL: server.URL})
	if _, err := client.Search(context.Background(), "user-a", "偏好"); err != nil {
		t.Fatal(err)
	}
	if err := client.Add(context.Background(), "user-a", "chat-1", []Message{{Role: "user", Content: "测试"}}, nil); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/search" || paths[1] != "/memories" {
		t.Fatalf("unexpected paths: %+v", paths)
	}
}
