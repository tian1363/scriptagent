package chat

import (
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestChatPromptIncludesProductContext(t *testing.T) {
	prompt := chatPrompt([]jobs.ChatMessage{
		{Role: "user", Content: "这个产品适合怎么裂变？"},
	}, "", "产品名称：测试产品\n\n# 卖点\n高频复购")

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

func TestChatPromptIncludesSummary(t *testing.T) {
	prompt := chatPrompt([]jobs.ChatMessage{
		{Role: "user", Content: "继续刚才的方向"},
	}, "用户确认目标平台是 TikTok，脚本节奏要快。", "")

	if !strings.Contains(prompt, "长期会话摘要") {
		t.Fatal("expected summary heading")
	}
	if !strings.Contains(prompt, "TikTok") {
		t.Fatal("expected summary content")
	}
}

func TestSelectProductMarkdownContextPrefersRelevantSections(t *testing.T) {
	longFiller := strings.Repeat("无关内容。\n", 1500)
	content := strings.Join([]string{
		"# 信息概览",
		longFiller,
		"## 玩法机制",
		"三消排序，货架整理，关卡推进。",
		"## 付费点",
		"礼包、金币、体力。",
	}, "\n")

	selected := selectProductMarkdownContext(content, "这个游戏的玩法机制是什么？")
	if !strings.Contains(selected, "三消排序") {
		t.Fatal("expected relevant gameplay section")
	}
}
