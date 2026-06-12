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

	selected, citations := selectProductMarkdownContext(content, "这个游戏的玩法机制是什么？", jobs.Product{ID: "p1", Title: "测试产品"})
	if !strings.Contains(selected, "三消排序") {
		t.Fatal("expected relevant gameplay section")
	}
	if len(citations) == 0 || citations[0].ProductID != "p1" {
		t.Fatalf("expected product citations, got %+v", citations)
	}
}

func TestBuildEmbeddingSectionsSplitsLongMarkdown(t *testing.T) {
	content := "# 概览\n" + strings.Repeat("这一段用于测试分块。\n\n", 120)
	sections := buildEmbeddingSections(content)
	if len(sections) < 2 {
		t.Fatalf("expected long markdown to split into multiple chunks, got %d", len(sections))
	}
	if sections[0].Order != 0 || sections[1].Order != 1 {
		t.Fatalf("unexpected section order: %+v", sections[:2])
	}
}

func TestCosineSimilarity(t *testing.T) {
	same := cosineSimilarity([]float64{1, 0}, []float64{1, 0})
	if same < 0.99 {
		t.Fatalf("expected same vectors to be close to 1, got %f", same)
	}
	orthogonal := cosineSimilarity([]float64{1, 0}, []float64{0, 1})
	if orthogonal != 0 {
		t.Fatalf("expected orthogonal vectors to be 0, got %f", orthogonal)
	}
}

func TestBuiltInSkillIncludesDataEyeHotMaterialAnalysis(t *testing.T) {
	skill, err := builtInSkill("dataeye_hot_material_analysis")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(skill, "DataEye") {
		t.Fatal("expected DataEye guidance")
	}
	if !strings.Contains(skill, "近 30 天") {
		t.Fatal("expected recent 30 day analysis guidance")
	}
	if !strings.Contains(skill, "不得编造") {
		t.Fatal("expected metric anti-fabrication rule")
	}
}
