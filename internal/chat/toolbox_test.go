package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/userctx"
)

type toolboxConfig struct {
	endpoint   string
	user       string
	capability string
}

func (p *toolboxConfig) GetModelRuntimeConfig(ctx context.Context, capability string) (model.RuntimeConfig, error) {
	p.user, p.capability = userctx.UserID(ctx), capability
	return model.RuntimeConfig{APIKey: "test-only-key", Endpoint: p.endpoint, Model: "test-model", Source: "byok"}, nil
}

func TestToolboxUsesAuthenticatedTextModelAndValidatesResult(t *testing.T) {
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Input struct {
				Messages []struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"messages"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		prompt = request.Input.Messages[0].Content[0].Text
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"output": map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": []any{map[string]string{"text": `{"candidates":[{"title":"自然分享","text":"轻松出门，随手带上你的随行杯。"}]}`}}}}}}})
	}))
	defer server.Close()
	provider := &toolboxConfig{endpoint: server.URL}
	service := NewService(nil, model.NewDashScopeClient(model.DashScopeConfig{Provider: provider}))
	result, err := service.GenerateToolboxDraft(userctx.WithUser(context.Background(), userctx.User{ID: "user-7"}), ToolboxDraftInput{Mode: "copy", Action: "rewrite", Text: "随行杯，方便携带"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || provider.user != "user-7" || provider.capability != "text" {
		t.Fatalf("result=%+v provider=%+v", result, provider)
	}
	if !strings.Contains(prompt, "随行杯") || !strings.Contains(prompt, "没有查看图片或视频") {
		t.Fatal("missing grounding instructions")
	}
}

func TestToolboxRejectsBadRequestsAndGeneratedContent(t *testing.T) {
	for _, in := range []ToolboxDraftInput{
		{Mode: "model", Action: "rewrite", Text: "test"},
		{Mode: "copy", Action: "rewrite"},
		{Mode: "copy", Action: "shorten", Instruction: "shorter"},
		{Mode: "product", Action: "claims", Text: strings.Repeat("字", 3001)},
	} {
		if in.Validate() == nil {
			t.Fatalf("accepted invalid input: %+v", in)
		}
	}
	for _, raw := range []string{
		`not json`,
		`{"candidates":[]}`,
		`{"candidates":[{"title":"x","text":""}]}`,
		`{"candidates":[{"title":"x","text":"one"},{"title":"y","text":"one"},{"title":"z","text":"two"}]}`,
	} {
		if _, err := parseToolboxDraft(raw, 3); err == nil {
			t.Fatalf("accepted invalid output: %s", raw)
		}
	}
	if _, err := parseToolboxDraft("```json\n{\"candidates\":[{\"title\":\"x\",\"text\":\"one\"}]}\n```", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(nil, nil).GenerateToolboxDraft(context.Background(), ToolboxDraftInput{Mode: "copy", Action: "rewrite", Text: "test"}); err == nil {
		t.Fatal("missing model must not return mock content")
	}
}
