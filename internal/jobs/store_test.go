package jobs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tian1363/scriptagent/internal/model"
)

func TestStoreProducts(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	created, err := store.CreateProduct(CreateProductInput{
		Title:  "测试产品",
		MDPath: "/tmp/product.md",
		MDName: "product.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.GetProduct(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "测试产品" || got.MDName != "product.md" {
		t.Fatalf("unexpected product: %+v", got)
	}

	products, err := store.ListProducts()
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
}

func TestStoreModelCallRunAssociation(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	space, err := store.CreateSpace(CreateSpaceInput{Title: "Observed space"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := store.CreateJob(CreateJobInput{Title: "Observed job", Industry: "game", FissionCount: 1, SpaceID: space.ID})
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.StartAgentRun(*job)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordModelCall(context.Background(), model.CallRecord{
		Scope: "job", RefID: job.ID, SpaceID: space.ID, RunID: run.ID, Step: "video_analysis", Model: "test",
	}); err != nil {
		t.Fatal(err)
	}
	calls, err := store.ListModelCalls(job.ID, space.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].RunID != run.ID || calls[0].SpaceID != space.ID {
		t.Fatalf("unexpected model call association: %+v", calls)
	}
	step, err := store.StartAgentStep(run.ID, 1, "video_analysis", "workflow", "start")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishAgentStep(step.ID, "completed", "done", ""); err != nil {
		t.Fatal(err)
	}
	observability, err := store.GetSpaceObservability(space.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(observability.Steps) != 1 || observability.Steps[0].RunID != run.ID || observability.Steps[0].OutputSummary != "done" {
		t.Fatalf("unexpected observable steps: %+v", observability.Steps)
	}
}

func TestStorePreventsConcurrentRunsAndAllowsRecovery(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	space, err := store.CreateSpace(CreateSpaceInput{Title: "Runtime space"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := store.CreateJob(CreateJobInput{Title: "Runtime job", Industry: "game", FissionCount: 1, SpaceID: space.ID})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.StartAgentRun(*job)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartAgentRun(*job); err == nil {
		t.Fatal("expected concurrent run to be rejected")
	}
	if err := store.FailActiveAgentRuns(job.ID, "restart recovery"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartAgentRun(*job); err != nil {
		t.Fatalf("expected a new run after recovery, got %v", err)
	}
	runs, err := store.ListAgentRuns(space.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("unexpected recovered runs: %+v", runs)
	}
	var recovered *AgentRun
	for index := range runs {
		if runs[index].ID == first.ID {
			recovered = &runs[index]
			break
		}
	}
	if recovered == nil || recovered.Status != "failed" || recovered.Error != "restart recovery" {
		t.Fatalf("unexpected recovered runs: %+v", runs)
	}
}

func TestStoreProductChunks(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	product, err := store.CreateProduct(CreateProductInput{
		Title:  "测试产品",
		MDPath: "/tmp/product.md",
		MDName: "product.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceProductChunks(product.ID, []ProductChunkInput{
		{
			ChunkIndex:     0,
			Heading:        "玩法",
			Content:        "三消排序",
			Embedding:      []float64{0.1, 0.2, 0.3},
			EmbeddingModel: "text-embedding-v4",
			EmbeddingDim:   3,
		},
	}); err != nil {
		t.Fatal(err)
	}

	chunks, err := store.ListProductChunks(product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Heading != "玩法" || chunks[0].EmbeddingDim != 3 || len(chunks[0].Embedding) != 3 {
		t.Fatalf("unexpected chunk: %+v", chunks[0])
	}
}

func TestStoreCreativeReports(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	product, err := store.CreateProduct(CreateProductInput{
		Title:  "测试产品",
		MDPath: "/tmp/product.md",
		MDName: "product.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateCreativeReport(CreateCreativeReportInput{
		ProductID:        product.ID,
		ProductTitle:     product.Title,
		SourceConfigJSON: `{"range":"30d"}`,
		ReportMarkdown:   "# 报告\n创意方向",
		ReportSummary:    "创意方向摘要",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetCreativeReport(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProductID != product.ID || got.ReportSummary != "创意方向摘要" {
		t.Fatalf("unexpected report: %+v", got)
	}
	reports, err := store.ListCreativeReports(product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
}

func TestStoreModelRuntimeConfig(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.SaveModelSettings(ModelSettings{
		APIKey:   "sk-test",
		Endpoint: "https://example.com/api",
		Model:    "qwen-test",
	}); err != nil {
		t.Fatal(err)
	}

	runtime, err := store.GetModelRuntimeConfig(context.Background(), "text")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.APIKey != "sk-test" || runtime.Endpoint != "https://example.com/api" || runtime.Model != "qwen-test" || runtime.Source != "byok" {
		t.Fatalf("unexpected runtime config: %+v", runtime)
	}
}

func TestStoreCustomSkills(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	created, err := store.CreateCustomSkill(CreateCustomSkillInput{
		Name: "review-hooks", Title: "钩子检查", Description: "检查短视频开头钩子。",
		Category: "脚本策略", InvocationPrompt: "调用 review-hooks skill。", Content: "# 钩子检查\n\n检查前三秒。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Source != "custom" {
		t.Fatalf("unexpected source: %s", created.Source)
	}

	got, err := store.GetCustomSkillByName("review-hooks")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != created.Content || got.Title != "钩子检查" {
		t.Fatalf("unexpected skill: %+v", got)
	}
	items, err := store.ListCustomSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "review-hooks" {
		t.Fatalf("unexpected skills: %+v", items)
	}
	updated, err := store.UpdateCustomSkill(created.ID, CreateCustomSkillInput{
		Name: "review-hooks", Title: "钩子深度检查", Description: "检查并改写前三秒钩子。",
		Category: "脚本策略", InvocationPrompt: "调用 review-hooks skill 深度检查。", Content: "# 钩子深度检查\n\n给出三个替换方案。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "钩子深度检查" || updated.Content == created.Content {
		t.Fatalf("unexpected updated skill: %+v", updated)
	}
}

func TestSpaceChatContextAndProductUpdate(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, err := store.CreateProduct(CreateProductInput{Title: "产品一", MDPath: "/tmp/one.md", MDName: "one.md"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateProduct(CreateProductInput{Title: "产品二", MDPath: "/tmp/two.md", MDName: "two.md"})
	if err != nil {
		t.Fatal(err)
	}
	space, err := store.CreateSpace(CreateSpaceInput{Title: "内容空间", Summary: "长期目标", ProductID: first.ID, MarketingGoal: "awareness", GoalStage: "reach"})
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := store.CreateChatConversationWithContext("继续空间", space.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddChatMessage(conversation.ID, "user", "历史消息"); err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdateSpace(space.ID, UpdateSpaceInput{Title: space.Title, Summary: space.Summary, ProductID: second.ID, MarketingGoal: "conversion", GoalStage: "action"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProductID != second.ID {
		t.Fatalf("expected updated product, got %s", updated.ProductID)
	}
	if updated.MarketingGoal != "conversion" || updated.GoalStage != "action" {
		t.Fatalf("expected updated marketing goal, got %+v", updated)
	}
	thread, err := store.GetChatThread(conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if thread.Conversation.SpaceID != space.ID || thread.Conversation.ProductID != first.ID {
		t.Fatalf("historical snapshot changed: %+v", thread.Conversation)
	}
	if len(thread.Messages) != 1 || thread.Messages[0].Content != "历史消息" {
		t.Fatalf("historical messages changed: %+v", thread.Messages)
	}
}

func TestChatAgentStepsPersistPerAssistantMessage(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	conversation, err := store.CreateChatConversation("多轮对话")
	if err != nil {
		t.Fatal(err)
	}
	first, _ := store.AddChatMessage(conversation.ID, "assistant", "第一轮")
	second, _ := store.AddChatMessage(conversation.ID, "assistant", "第二轮")
	if err := store.SaveChatAgentSteps(conversation.ID, first.ID, []AgentStep{{Index: 1, Kind: "tool", Tool: "list_products", Reason: "读取产品"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChatAgentSteps(conversation.ID, second.ID, []AgentStep{{Index: 1, Kind: "final", Reason: "整理结果"}}); err != nil {
		t.Fatal(err)
	}
	thread, err := store.GetChatThread(conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if thread.AgentTraces[first.ID][0].Tool != "list_products" || thread.AgentTraces[second.ID][0].Kind != "final" {
		t.Fatalf("unexpected per-message traces: %+v", thread.AgentTraces)
	}
}

func TestDeleteSpace(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	space, err := store.CreateSpace(CreateSpaceInput{Title: "待删除空间", Summary: "临时目标"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteSpace(space.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSpace(space.ID); err == nil {
		t.Fatal("expected deleted space to be missing")
	}
}

func TestVideoGenerationKeepsReferenceAssetOrder(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateUser(CreateUserInput{Email: "video@example.com", Name: "Video", PasswordHash: "test", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateVideoGeneration(CreateVideoGenerationInput{
		UserID: user.ID, SourceAssetIDs: []string{"asset-2", "asset-1"}, Mode: "image", Prompt: "图1与图2",
		Model: "wan3.0-video-prime", Resolution: "720P", Ratio: "9:16", Duration: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetUserVideoGeneration(user.ID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceAssetIDs) != 2 || got.SourceAssetIDs[0] != "asset-2" || got.SourceAssetIDs[1] != "asset-1" {
		t.Fatalf("reference order was not preserved: %+v", got.SourceAssetIDs)
	}
}
