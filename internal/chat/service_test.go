package chat

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestCustomSkillCanBeResolvedByAgentTool(t *testing.T) {
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.CreateCustomSkill(jobs.CreateCustomSkillInput{Name: "review-hooks", Title: "钩子检查", Description: "检查钩子", Category: "自定义", InvocationPrompt: "调用 review-hooks", Content: "# 钩子检查\n\n检查前三秒。"})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil)
	content, err := service.skillContent("review-hooks")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "检查前三秒") {
		t.Fatalf("unexpected content: %s", content)
	}
}

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

func TestMessagesAfterSummaryKeepsOnlyUnsummarizedHistory(t *testing.T) {
	messages := []jobs.ChatMessage{
		{ID: "m1", Role: "user", Content: "old"},
		{ID: "m2", Role: "assistant", Content: "summary cutoff"},
		{ID: "m3", Role: "user", Content: "pending one"},
		{ID: "m4", Role: "assistant", Content: "pending two"},
	}
	recent := messagesAfterSummary(messages, "m2")
	if len(recent) != 2 || recent[0].ID != "m3" || recent[1].ID != "m4" {
		t.Fatalf("expected only messages after summary cutoff, got %+v", recent)
	}
}

func TestReactContextDoesNotContainCurrentGoal(t *testing.T) {
	messages := []jobs.ChatMessage{
		{ID: "m1", Role: "user", Content: "earlier question"},
		{ID: "m2", Role: "assistant", Content: "earlier answer"},
		{ID: "m3", Role: "user", Content: "CURRENT UNIQUE GOAL"},
	}
	prompt := reactChatContextPrompt(messages[:len(messages)-1], "", "", "")
	if strings.Contains(prompt, "CURRENT UNIQUE GOAL") {
		t.Fatal("current goal must only be passed through RunInput.Goal")
	}
	if !strings.Contains(prompt, "earlier question") {
		t.Fatal("expected earlier history to remain")
	}
}

func TestReactContextKeepsPendingBatchMessages(t *testing.T) {
	messages := make([]jobs.ChatMessage, 0, recentChatMessageLimit+summaryBatchMessageCount-1)
	for index := 0; index < cap(messages); index++ {
		messages = append(messages, jobs.ChatMessage{Role: "user", Content: fmt.Sprintf("pending-%02d", index)})
	}
	prompt := reactChatContextPrompt(messages, "existing summary", "", "")
	if !strings.Contains(prompt, "pending-00") || !strings.Contains(prompt, "pending-18") {
		t.Fatal("messages waiting for the next summary batch must remain in context")
	}
}

func TestReactChatContextIncludesCreativeSpaceWithoutCreatingUserMessage(t *testing.T) {
	prompt := reactChatContextPrompt(nil, "", "product-1", "创作空间：夏季投放\n长期目标：完成 8 条脚本\n长期要求：节奏快")
	if !strings.Contains(prompt, "当前创作空间上下文") || !strings.Contains(prompt, "完成 8 条脚本") {
		t.Fatalf("missing space context: %s", prompt)
	}
	if strings.Contains(prompt, "用户：继续推进创作空间") {
		t.Fatalf("space context should not be a user message: %s", prompt)
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

func TestBuiltInSkillIncludesMaterialReplicationAnalysis(t *testing.T) {
	skill, err := builtInSkill("material_replication_analysis")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(skill, "多模态") {
		t.Fatal("expected multimodal analysis guidance")
	}
	if !strings.Contains(skill, "视频复刻方案") {
		t.Fatal("expected video replication guidance")
	}
	if !strings.Contains(skill, "不得假装看过") {
		t.Fatal("expected attachment evidence rule")
	}
}

func TestBuiltInSkillsExposeUserFacingMetadata(t *testing.T) {
	skills := BuiltInSkills()
	if len(skills) < 5 {
		t.Fatalf("expected at least 5 skills, got %d", len(skills))
	}
	found := map[string]bool{}
	for _, skill := range skills {
		if skill.Name == "" || skill.Title == "" || skill.Description == "" || skill.InvocationPrompt == "" {
			t.Fatalf("skill metadata should be user-facing: %+v", skill)
		}
		found[skill.Name] = true
	}
	for _, name := range []string{"fission_strategy", "material_replication_analysis", "seedance_video_prompt_writer"} {
		if !found[name] {
			t.Fatalf("expected skill %s in catalog", name)
		}
	}
}
