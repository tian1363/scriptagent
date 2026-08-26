package telemetry

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestLangfuseTraceEndpoint(t *testing.T) {
	tests := map[string]string{
		"":                                      "https://cloud.langfuse.com/api/public/otel/v1/traces",
		"https://us.cloud.langfuse.com/":        "https://us.cloud.langfuse.com/api/public/otel/v1/traces",
		"http://localhost:3000/api/public/otel": "http://localhost:3000/api/public/otel/v1/traces",
		"http://localhost:3000/api/public/otel/v1/traces": "http://localhost:3000/api/public/otel/v1/traces",
	}
	for input, expected := range tests {
		if got := langfuseTraceEndpoint(input); got != expected {
			t.Fatalf("endpoint %q: expected %q, got %q", input, expected, got)
		}
	}
}

func TestLangfuseExporterSendsV4OTLPTrace(t *testing.T) {
	type receivedRequest struct {
		path, authorization, version string
		body                         []byte
	}
	var mu sync.Mutex
	var received receivedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = receivedRequest{path: r.URL.Path, authorization: r.Header.Get("Authorization"), version: r.Header.Get("x-langfuse-ingestion-version"), body: body}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	shutdown, enabled, err := InitLangfuse(context.Background(), Config{PublicKey: "pk-test", SecretKey: "sk-test", BaseURL: server.URL, Environment: "test"})
	if err != nil || !enabled {
		t.Fatalf("expected tracing enabled, enabled=%v err=%v", enabled, err)
	}
	ctx, root := StartAgentRun(context.Background(), RunAttributes{Name: "test-agent", RunID: "run-1", Input: "input"})
	_, child := StartGeneration(ctx, GenerationAttributes{Name: "test-generation", TraceName: "test-agent", RunID: "run-1", Model: "test-model", Input: "prompt"})
	if root.SpanContext().TraceID() != child.SpanContext().TraceID() {
		t.Fatal("expected generation to be a child of the agent trace")
	}
	EndGeneration(child, "answer", 2, 1, 3, nil)
	EndAgentRun(root, "done", nil)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	got := received
	mu.Unlock()
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk-test:sk-test"))
	if got.path != "/api/public/otel/v1/traces" || got.authorization != expectedAuth || got.version != "4" || len(got.body) == 0 {
		t.Fatalf("unexpected OTLP request: path=%q auth=%q version=%q body=%d", got.path, got.authorization, got.version, len(got.body))
	}
}

func TestContentCaptureDefaultsToRedacted(t *testing.T) {
	captureContent.Store(false)
	if got := Content("secret prompt"); got != "[content capture disabled]" {
		t.Fatalf("expected redacted content, got %q", got)
	}
	captureContent.Store(true)
	if got := Content("secret prompt"); got != "secret prompt" {
		t.Fatalf("expected captured content, got %q", got)
	}
	captureContent.Store(false)
}

func TestInitLangfuseRequiresBothKeys(t *testing.T) {
	if _, _, err := InitLangfuse(context.Background(), Config{PublicKey: "pk-only"}); err == nil {
		t.Fatal("expected incomplete credentials to fail")
	}
	shutdown, enabled, err := InitLangfuse(context.Background(), Config{})
	if err != nil || enabled {
		t.Fatalf("expected missing credentials to disable tracing, enabled=%v err=%v", enabled, err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestUsageJSON(t *testing.T) {
	if got := usageJSON(10, 4, 14); got != `{"input":10,"output":4,"total":14}` {
		t.Fatalf("unexpected usage JSON: %s", got)
	}
}
