package chat

import (
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestChatPromptIncludesProductContext(t *testing.T) {
	prompt := chatPrompt([]jobs.ChatMessage{
		{Role: "user", Content: "这个产品适合怎么裂变？"},
	}, "产品名称：测试产品\n\n# 卖点\n高频复购")

	if !strings.Contains(prompt, "产品资料库上下文") {
		t.Fatal("expected product context heading")
	}
	if !strings.Contains(prompt, "高频复购") {
		t.Fatal("expected product Markdown content")
	}
	if !strings.Contains(prompt, "这个产品适合怎么裂变？") {
		t.Fatal("expected chat message")
	}
}
