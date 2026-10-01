package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/tian1363/scriptagent/internal/model"
)

func TestHookReplicationSeparatesVideoEvidenceFromScriptGeneration(t *testing.T) {
	var mu sync.Mutex
	requests := [][]model.ContentItem{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input struct {
				Messages []struct {
					Content []model.ContentItem `json:"content"`
				} `json:"messages"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if len(body.Input.Messages) != 1 {
			t.Errorf("expected one message, got %d", len(body.Input.Messages))
			return
		}
		mu.Lock()
		requests = append(requests, body.Input.Messages[0].Content)
		index := len(requests)
		mu.Unlock()
		answer := "观察：街头主持人持麦，随后切到受访者。"
		if index == 2 {
			answer = "## 原创分镜表\n街头逐个采访。\n## 视频生成提示词\n街头主持人持麦采访。"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"output":{"choices":[{"message":{"content":[{"text":%q}]}}]}}`, answer)
	}))
	defer server.Close()
	client := model.NewDashScopeClient(model.DashScopeConfig{APIKey: "test", Endpoint: server.URL, Model: "test-model"})
	service := NewService(nil, client)
	answer, _, steps, err := service.runHookReplication(context.Background(), "chat", "run", "", "", "user", "写 15 秒脚本", "", "产品图", nil, model.ContentItem{Video: "data:video/mp4;base64,AQID", FPS: 2}, []model.ContentItem{{Image: "data:image/png;base64,AQID"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || !strings.Contains(answer, "视频生成提示词") {
		t.Fatalf("unexpected workflow result: steps=%d answer=%q", len(steps), answer)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 || len(requests[0]) != 2 || requests[0][1].Video == "" {
		t.Fatalf("analysis call must receive reference video: %+v", requests)
	}
	if len(requests[1]) != 2 || requests[1][0].Video != "" || requests[1][1].Image == "" || !strings.Contains(requests[1][0].Text, "观察：街头主持人持麦") {
		t.Fatal("script call must consume textual evidence and product images without reusing the reference video")
	}
}
