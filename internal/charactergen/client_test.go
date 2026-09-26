package charactergen

import (
	"context"
	"encoding/json"
	"github.com/tian1363/scriptagent/internal/model"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func TestReferenceControlsReachProvider(t *testing.T) {
	controls := Controls{Age: 28, Description: "must not change face", Hair: "ignored hairstyle", Outfit: "blue shirt", Size: "720*1280", Seed: 42}
	calls := 0
	client := New()
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := "image-bytes"
		if r.Method == "POST" {
			var req struct {
				Parameters map[string]any `json:"parameters"`
				Input      struct {
					Messages []struct {
						Content []map[string]string `json:"content"`
					} `json:"messages"`
				} `json:"input"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.Parameters["seed"] != float64(42) || req.Parameters["prompt_extend"] != false || req.Parameters["size"] != "720*1280" {
				t.Fatalf("wrong controls: %+v", req.Parameters)
			}
			content := req.Input.Messages[0].Content
			if !strings.HasPrefix(content[0]["image"], "data:") || strings.Contains(content[1]["text"], "ignored hairstyle") {
				t.Fatal("reference not preserved")
			}
			body = `{"output":{"choices":[{"message":{"content":[{"image":"https://results.oss-cn-beijing.aliyuncs.com/result.png"}]}}]}}`
		} else if r.Header.Get("Authorization") != "" {
			t.Fatal("key leaked to download")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	cfg := model.RuntimeConfig{APIKey: "test", Provider: "dashscope", Model: "qwen-image-edit-plus", Endpoint: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"}
	out, err := client.Generate(context.Background(), cfg, controls, []byte("ref-image"))
	if err != nil || string(out) != "image-bytes" || calls != 2 {
		t.Fatalf("result %q %v calls %d", out, err, calls)
	}
}
func TestCharacterBoundaries(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1/test", "https://aliyuncs.com.evil.test/x", "https://test.aliyuncs.com@evil.test/x", "https://test.aliyuncs.com:8080/x"} {
		if TrustedURL(u) {
			t.Fatal(u)
		}
	}
	c := Controls{Age: 28, Size: "720*1280", Seed: 42}
	if c.Validate() != nil {
		t.Fatal("valid controls rejected")
	}
	c.Seed = -1
	if c.Validate() == nil {
		t.Fatal("negative seed accepted")
	}
	cfg := model.RuntimeConfig{APIKey: "test", Provider: "openai", Model: "other"}
	if _, err := New().Generate(context.Background(), cfg, Controls{Age: 28, Size: "720*1280"}, nil); err == nil {
		t.Fatal("unsupported provider accepted")
	}
}
