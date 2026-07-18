package websearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTavilySearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tvly-test" {
			t.Fatalf("unexpected authorization %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["query"] != "短视频趋势" || body["search_depth"] != "basic" || body["max_results"] != float64(3) {
			t.Fatalf("unexpected body %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{
			"title": "趋势报告", "url": "https://example.com/report", "content": "短视频内容趋势摘要", "score": 0.86,
		}}})
	}))
	defer server.Close()

	client := NewClient(Config{Provider: "tavily", BaseURL: server.URL, APIKey: "tvly-test", MaxResults: 5})
	response, err := client.Search(context.Background(), Query{Text: "短视频趋势", MaxResults: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].Title != "趋势报告" {
		t.Fatalf("unexpected response %+v", response)
	}
}

func TestSearXNGSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "广告创意" || r.URL.Query().Get("format") != "json" {
			t.Fatalf("unexpected query %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{
			"title": "创意案例", "url": "https://example.org/case", "content": "广告创意案例摘要",
		}}})
	}))
	defer server.Close()

	client := NewClient(Config{Provider: "searxng", BaseURL: server.URL, MaxResults: 5})
	response, err := client.Search(context.Background(), Query{Text: "广告创意"})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Provider != "searxng" {
		t.Fatalf("unexpected response %+v", response)
	}
}

func TestRejectsInvalidResultURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{
			"title": "bad", "url": "file:///etc/passwd", "content": "bad result",
		}}})
	}))
	defer server.Close()
	client := NewClient(Config{Provider: "tavily", BaseURL: server.URL, APIKey: "test"})
	response, err := client.Search(context.Background(), Query{Text: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 0 {
		t.Fatalf("expected invalid URL to be removed: %+v", response.Results)
	}
}
