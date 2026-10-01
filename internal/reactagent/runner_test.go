package reactagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/model"
)

func TestParseActionFromJSON(t *testing.T) {
	action, err := parseAction(`{"type":"tool","reason":"需要查产品资料","tool":"list_products","input":{}}`)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != "tool" || action.Tool != "list_products" {
		t.Fatalf("unexpected action: %+v", action)
	}
}

func TestParseActionFromFencedJSON(t *testing.T) {
	action, err := parseAction("```json\n{\"type\":\"final\",\"answer\":\"完成\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != "final" || action.Answer != "完成" {
		t.Fatalf("unexpected action: %+v", action)
	}
}

func TestNormalizeRawJSON(t *testing.T) {
	raw := normalizeRawJSON([]byte(`{"b":2}`))
	if string(raw) != `{"b":2}` {
		t.Fatalf("unexpected normalized json: %s", raw)
	}
	empty := normalizeRawJSON(nil)
	if string(empty) != `{}` {
		t.Fatalf("unexpected empty json: %s", empty)
	}
}

func TestDefaultMaxStepsIsFour(t *testing.T) {
	if defaultMaxSteps != 4 {
		t.Fatalf("expected default max steps 4, got %d", defaultMaxSteps)
	}
}

func TestToolObservationCountsOnlyWhenPreparedForNextPrompt(t *testing.T) {
	steps := []Step{{Kind: "tool", Observation: "产品资料", RawObservationChars: 9000}, {Kind: "final"}}
	if steps[0].PromptObservationChars != 0 {
		t.Fatal("a tool result has not yet entered a model prompt")
	}
	markToolObservationsPrompted(steps)
	if steps[0].PromptObservationChars != len([]rune("产品资料")) || steps[0].RawObservationChars != 9000 {
		t.Fatalf("unexpected observation lengths: %+v", steps[0])
	}
}

func TestToolCallKeyNormalizesEquivalentJSON(t *testing.T) {
	first := toolCallKey("retrieve", []byte(`{"b":2,"a":1}`))
	second := toolCallKey(" retrieve ", []byte(`{"a":1,"b":2}`))
	if first != second {
		t.Fatalf("expected equivalent calls to share a key: %q != %q", first, second)
	}
}

func TestFinalPromptUsesSkillResultToAnswerPromptOnlyRequest(t *testing.T) {
	prompt := finalPrompt("调用 `generate-video` skill 输出AI视频生成提示词", "产品：梨汁", []Step{{Index: 1, Kind: "tool", Tool: "call_skill", Observation: "按脚本写视频提示词"}})
	for _, part := range []string{"不得再调用工具", "直接写出可用的提示词", "产品：梨汁", "按脚本写视频提示词"} {
		if !strings.Contains(prompt, part) {
			t.Fatalf("final prompt missing %q", part)
		}
	}
}

func TestRepeatedSkillCallFinishesFromExistingObservation(t *testing.T) {
	requests, toolCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body struct {
			Input struct {
				Messages []struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"messages"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		prompt := body.Input.Messages[0].Content[0].Text
		response := `{"type":"tool","tool":"call_skill","input":{"skill":"generate-video"}}`
		if requests == 3 {
			if !strings.Contains(prompt, "不得再调用工具") || !strings.Contains(prompt, "技能说明") {
				t.Fatalf("final prompt lacks instruction or observation: %s", prompt)
			}
			response = `{"type":"final","answer":"视频生成提示词：产品放在桌面，镜头缓慢推进。"}`
		}
		fmt.Fprintf(w, `{"output":{"choices":[{"message":{"content":[{"text":%q}]}}]}}`, response)
	}))
	defer server.Close()
	client := model.NewDashScopeClient(model.DashScopeConfig{APIKey: "test", Endpoint: server.URL})
	result, err := New(client, 4).Run(context.Background(), RunInput{
		Goal: "调用 `generate-video` skill 输出AI视频生成提示词",
		Tools: []Tool{{Name: "call_skill", Handler: func(context.Context, json.RawMessage) (string, error) {
			toolCalls++
			return "技能说明：根据脚本生成提示词", nil
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if toolCalls != 1 || requests != 3 || !strings.Contains(result.Answer, "产品放在桌面") {
		t.Fatalf("unexpected result: requests=%d toolCalls=%d answer=%q", requests, toolCalls, result.Answer)
	}
}
