package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/reactagent"
	"github.com/tian1363/scriptagent/internal/telemetry"
	"github.com/tian1363/scriptagent/internal/userctx"
)

const (
	recentChatMessageLimit       = 12
	summaryTriggerMessageCount   = 16
	summaryTailMessageCount      = recentChatMessageLimit
	summaryBatchMessageCount     = 8
	maxProductContextChars       = 6000
	maxProductSectionPreviewChar = 1800
	maxChatAttachmentDataURI     = 20 * 1024 * 1024
)

type Service struct {
	store       *jobs.Store
	client      *model.DashScopeClient
	reactRunner *reactagent.Runner
	progressMu  sync.RWMutex
	progress    map[string][]jobs.AgentStep
}

type BuiltInSkillInfo struct {
	Name             string `json:"name"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	InvocationPrompt string `json:"invocation_prompt"`
	Content          string `json:"content"`
	Source           string `json:"source"`
}

type AttachmentInput struct {
	Path string
	Name string
	Kind string
	Size int64
}

func NewService(store *jobs.Store, client *model.DashScopeClient) *Service {
	return &Service{store: store, client: client, reactRunner: reactagent.New(client, 0), progress: make(map[string][]jobs.AgentStep)}
}

func (s *Service) resetProgress(conversationID string) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.progress[conversationID] = nil
}

func (s *Service) appendProgress(conversationID string, step reactagent.Step) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	converted := toJobAgentSteps([]reactagent.Step{step})[0]
	for index := range s.progress[conversationID] {
		if s.progress[conversationID][index].Index == converted.Index {
			s.progress[conversationID][index] = converted
			return
		}
	}
	s.progress[conversationID] = append(s.progress[conversationID], converted)
}

func (s *Service) Progress(conversationID string) []jobs.AgentStep {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	return append([]jobs.AgentStep(nil), s.progress[conversationID]...)
}

func (s *Service) GenerateSkillDraft(ctx context.Context, requirement string) (*jobs.CreateCustomSkillInput, error) {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return nil, errors.New("skill requirement is required")
	}
	if s.client == nil {
		return nil, errors.New("技能生成模型未配置")
	}
	prompt := `你是 Skill Creator。根据用户需求创建一个可执行、可复用的 Agent Skill。
只输出一个 JSON 对象，不要 Markdown 代码围栏。字段必须是 name、title、description、category、invocation_prompt、content。
要求：
- name 使用 64 字符以内的小写英文、数字和连字符，动词优先。
- description 同时说明技能做什么以及何时调用，使 Agent 能正确触发。
- invocation_prompt 是用户选择该技能后放入输入框的简短中文指令。
- content 是完整 SKILL.md 正文，不含 YAML frontmatter；必须针对需求写清目标、适用输入、必要前置检查、分步骤工作流、判断规则、失败处理和输出格式。
- 不要写“理解需求、按目标处理、给出结果”这类空泛步骤；每一步必须能直接指导 Agent 执行。
- 不得声称拥有未提供的工具；需要资料或附件时明确校验并向用户索取。

用户需求：` + requirement
	result, err := s.client.GenerateDetailed(ctx, model.CallContext{Scope: "skill", Step: "skill_creator", TraceName: "skill-creator"}, []model.ContentItem{{Text: prompt}})
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(result.Text)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	var draft jobs.CreateCustomSkillInput
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &draft); err != nil {
		return nil, fmt.Errorf("技能草稿格式无效，请重试: %w", err)
	}
	draft.Name = normalizeSkillName(draft.Name)
	draft.Title = strings.TrimSpace(draft.Title)
	draft.Description = strings.TrimSpace(draft.Description)
	draft.Category = strings.TrimSpace(draft.Category)
	draft.InvocationPrompt = strings.TrimSpace(draft.InvocationPrompt)
	draft.Content = strings.TrimSpace(draft.Content)
	if draft.Name == "" || draft.Title == "" || draft.Description == "" || draft.Content == "" {
		return nil, errors.New("技能草稿缺少必要内容，请重试")
	}
	if draft.Category == "" {
		draft.Category = "自定义"
	}
	if draft.InvocationPrompt == "" {
		draft.InvocationPrompt = "使用「" + draft.Title + "」技能完成当前任务。"
	}
	return &draft, nil
}

func normalizeSkillName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if valid {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && out.Len() > 0 {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(strings.TrimSpace(out.String()), "-")
}

func (s *Service) Send(ctx context.Context, conversationID, content, productID string) (*jobs.ChatThread, error) {
	return s.SendWithAttachments(ctx, conversationID, content, productID, nil)
}

func (s *Service) SendWithAttachments(ctx context.Context, conversationID, content, productID string, attachments []AttachmentInput) (threadResult *jobs.ChatThread, sendErr error) {
	content = strings.TrimSpace(content)
	if content == "" && len(attachments) == 0 {
		return nil, errors.New("message content is required")
	}
	if s.client == nil {
		return nil, errors.New("chat model is not configured")
	}

	var conversation *jobs.ChatConversation
	var messages []jobs.ChatMessage
	var err error
	userID := userctx.UserID(ctx)
	if conversationID == "" {
		title := content
		if title == "" && len(attachments) > 0 {
			title = "素材分析"
		}
		conversation, err = s.store.CreateChatConversation(title)
		if err != nil {
			return nil, err
		}
		if err := s.store.ClaimResource(userID, "chat", conversation.ID); err != nil {
			return nil, err
		}
		conversationID = conversation.ID
	} else {
		thread, err := s.store.GetUserChatThread(userID, conversationID)
		if err != nil {
			return nil, err
		}
		conversation = &thread.Conversation
		messages = thread.Messages
	}

	displayContent := content
	if displayContent == "" {
		displayContent = "请分析我上传的素材。"
	}
	if len(attachments) > 0 {
		displayContent += "\n\n附件：" + attachmentSummary(attachments)
	}
	modelAttachments, err := contentItemsForAttachments(attachments)
	if err != nil {
		return nil, err
	}
	productVisualContext := ""
	if productItems, assetContext := s.productAssetContext(userID, productID); len(productItems) > 0 {
		modelAttachments = append(productItems, modelAttachments...)
		productVisualContext = assetContext
	}
	userMessage, err := s.store.AddChatMessage(conversationID, "user", displayContent)
	if err != nil {
		return nil, err
	}
	messages = append(messages, *userMessage)
	ctx, traceSpan := telemetry.StartAgentRun(ctx, telemetry.RunAttributes{
		Name: "chat-agent-loop", RunID: userMessage.ID, JobID: conversationID,
		SessionID: conversationID, Input: content,
	})
	traceOutput := ""
	defer func() { telemetry.EndAgentRun(traceSpan, traceOutput, sendErr) }()

	summary := s.refreshSummary(ctx, conversationID, userMessage.ID, conversation, messages)
	contextMessages := messagesAfterSummary(messages[:len(messages)-1], conversation.SummaryMessageID)
	spaceContext := ""
	if conversation != nil && conversation.SpaceID != "" {
		if space, spaceErr := s.store.GetUserSpace(userID, conversation.SpaceID); spaceErr == nil {
			spaceContext = strings.Join([]string{
				"创作空间：" + space.Title,
				"广告创作目标：" + marketingGoalLabel(space.MarketingGoal),
				"营销阶段：" + marketingStageLabel(space.GoalStage),
				"长期目标：" + space.Summary,
				"长期要求：" + space.AgentBrief,
				"执行约束：本轮策略、脚本、分镜、素材选择和行动建议都必须服务于上述广告目标与营销阶段。发现用户请求可能偏离时，先围绕目标收敛方案；不要复述这段系统约束。",
			}, "\n")
			if memoryContext := s.store.CreativeMemoryContext(userID, conversation.SpaceID, 6); memoryContext != "" {
				spaceContext += "\n\n" + memoryContext
			}
		}
	}
	if productVisualContext != "" {
		spaceContext = strings.TrimSpace(spaceContext + "\n\n" + productVisualContext)
	}
	citations := []jobs.ProductCitation{}
	s.resetProgress(conversationID)
	reactResult, err := s.reactRunner.Run(ctx, reactagent.RunInput{
		Scope:         "chat",
		RefID:         conversationID,
		RunID:         userMessage.ID,
		SessionID:     conversationID,
		TraceName:     "chat-agent-loop",
		Goal:          displayContent,
		ContextPrompt: reactChatContextPrompt(contextMessages, summary, productID, spaceContext),
		Tools:         s.reactTools(userID, conversationID, userMessage.ID, productID, content, &citations),
		Attachments:   modelAttachments,
		OnProgress: func(step reactagent.Step) {
			s.appendProgress(conversationID, step)
		},
		OnStep: func(step reactagent.Step) {
			s.appendProgress(conversationID, step)
		},
	})
	if err != nil {
		return nil, err
	}
	traceOutput = reactResult.Answer
	assistantMessage, err := s.store.AddChatMessage(conversationID, "assistant", reactResult.Answer)
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveChatAgentSteps(conversationID, assistantMessage.ID, toJobAgentSteps(reactResult.Steps)); err != nil {
		return nil, err
	}
	thread, err := s.store.GetUserChatThread(userID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation != nil {
		thread.Conversation.Title = conversation.Title
	}
	thread.Citations = citations
	thread.AgentSteps = toJobAgentSteps(reactResult.Steps)
	return thread, nil
}

func marketingGoalLabel(value string) string {
	labels := map[string]string{
		"conversion": "商品转化（推动下单、留资或到店）",
		"awareness":  "品牌认知（建立记忆点与品牌心智）",
		"seeding":    "内容种草（强化场景、卖点与信任）",
		"growth":     "用户增长（促进关注、互动与拉新）",
		"campaign":   "活动引爆（集中放大活动声量与参与）",
	}
	if label := labels[strings.TrimSpace(value)]; label != "" {
		return label
	}
	return "未设置；优先澄清本轮广告目标"
}

func marketingStageLabel(value string) string {
	labels := map[string]string{
		"reach":    "认知（先让目标人群看见并记住）",
		"interest": "兴趣（让用户理解价值并产生偏好）",
		"action":   "行动（用明确利益点推动立即转化）",
	}
	if label := labels[strings.TrimSpace(value)]; label != "" {
		return label
	}
	return "未设置；根据用户当前任务判断"
}

func (s *Service) refreshSummary(ctx context.Context, conversationID, runID string, conversation *jobs.ChatConversation, messages []jobs.ChatMessage) string {
	if conversation == nil || len(messages) <= summaryTriggerMessageCount {
		if conversation == nil {
			return ""
		}
		return conversation.Summary
	}
	summarizedMessages := messages[:len(messages)-summaryTailMessageCount]
	if len(summarizedMessages) == 0 {
		return conversation.Summary
	}
	cutoffID := summarizedMessages[len(summarizedMessages)-1].ID
	if cutoffID == conversation.SummaryMessageID {
		return conversation.Summary
	}
	messagesToSummarize := summarizedMessages
	if conversation.SummaryMessageID != "" {
		for index, message := range summarizedMessages {
			if message.ID == conversation.SummaryMessageID {
				messagesToSummarize = summarizedMessages[index+1:]
				break
			}
		}
	}
	if len(messagesToSummarize) == 0 {
		return conversation.Summary
	}
	if conversation.SummaryMessageID != "" && len(messagesToSummarize) < summaryBatchMessageCount {
		return conversation.Summary
	}

	result, err := s.client.GenerateDetailed(ctx, model.CallContext{
		Scope: "chat", RefID: conversationID, RunID: runID, SessionID: conversationID,
		TraceName: "chat-agent-loop", Step: "chat_summary",
	}, []model.ContentItem{{Text: summaryPrompt(conversation.Summary, messagesToSummarize)}})
	if err != nil {
		return conversation.Summary
	}
	summary := strings.TrimSpace(result.Text)
	if summary == "" {
		return conversation.Summary
	}
	if err := s.store.SaveChatSummary(conversationID, summary, cutoffID); err != nil {
		return conversation.Summary
	}
	conversation.Summary = summary
	conversation.SummaryMessageID = cutoffID
	return summary
}

func messagesAfterSummary(messages []jobs.ChatMessage, summaryMessageID string) []jobs.ChatMessage {
	if summaryMessageID != "" {
		for index, message := range messages {
			if message.ID == summaryMessageID {
				return messages[index+1:]
			}
		}
	}
	return messages
}

func contentItemsForAttachments(attachments []AttachmentInput) ([]model.ContentItem, error) {
	items := make([]model.ContentItem, 0, len(attachments))
	for _, attachment := range attachments {
		if strings.TrimSpace(attachment.Path) == "" {
			continue
		}
		dataURL, mimeType, err := attachmentDataURL(attachment.Path)
		if err != nil {
			return nil, err
		}
		switch {
		case strings.HasPrefix(mimeType, "image/"):
			items = append(items, model.ContentItem{Image: dataURL})
		case strings.HasPrefix(mimeType, "video/"):
			items = append(items, model.ContentItem{Video: dataURL, FPS: 2})
		default:
			return nil, fmt.Errorf("unsupported attachment MIME type %s", mimeType)
		}
	}
	return items, nil
}

func (s *Service) productAssetContext(userID, productID string) ([]model.ContentItem, string) {
	if _, err := s.store.GetUserProduct(userID, productID); err != nil {
		return nil, ""
	}
	if strings.TrimSpace(productID) == "" {
		return nil, ""
	}
	assets, err := s.store.ListProductAssets(productID)
	if err != nil || len(assets) == 0 {
		return nil, ""
	}
	items := []model.ContentItem{}
	lines := []string{"产品资料中有以下视觉素材。生成脚本或分镜时必须结合实际画面，按 CID 标注引用；最终结果选择 1 至 3 个关键素材，用 Markdown 图片语法展示小封面。"}
	var totalBytes int64
	mediaCount := 0
	for _, asset := range assets {
		if mediaCount >= 3 || totalBytes+asset.SizeBytes > 18*1024*1024 {
			break
		}
		dataURL, mimeType, readErr := attachmentDataURL(asset.Path)
		if readErr != nil {
			continue
		}
		cid := "asset-" + asset.ID
		lines = append(lines, fmt.Sprintf("- CID %s：%s（%s），展示地址 /api/assets/%s/file", cid, asset.OriginalName, asset.Kind, asset.ID))
		items = append(items, model.ContentItem{Text: "CID " + cid + "，素材名：" + asset.OriginalName})
		if strings.HasPrefix(mimeType, "image/") {
			items = append(items, model.ContentItem{Image: dataURL})
		}
		if strings.HasPrefix(mimeType, "video/") {
			items = append(items, model.ContentItem{Video: dataURL, FPS: 2})
		}
		mediaCount++
		totalBytes += asset.SizeBytes
	}
	if len(items) == 0 {
		return nil, ""
	}
	return items, strings.Join(lines, "\n")
}

func attachmentDataURL(path string) (string, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "", err
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		return "", "", errors.New("cannot determine attachment MIME type")
	}
	if dataURLByteLen(info.Size(), mimeType) > maxChatAttachmentDataURI {
		return "", "", fmt.Errorf("attachment data-uri would be %.1fMB, exceeds %.1fMB limit", mb(dataURLByteLen(info.Size(), mimeType)), mb(maxChatAttachmentDataURI))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(raw), mimeType, nil
}

func dataURLByteLen(rawBytes int64, mimeType string) int64 {
	return int64(len("data:"+mimeType+";base64,")) + int64(base64.StdEncoding.EncodedLen(int(rawBytes)))
}

func mb(bytes int64) float64 { return float64(bytes) / 1024 / 1024 }

func attachmentSummary(attachments []AttachmentInput) string {
	rows := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			name = filepath.Base(attachment.Path)
		}
		kind := strings.TrimSpace(attachment.Kind)
		if kind == "" {
			kind = "素材"
		}
		rows = append(rows, fmt.Sprintf("%s（%s，%.1fMB）", name, kind, mb(attachment.Size)))
	}
	return strings.Join(rows, "、")
}

func (s *Service) productContext(ctx context.Context, userID, conversationID, runID, productID, query string) (string, []jobs.ProductCitation, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return "", nil, nil
	}
	product, err := s.store.GetUserProduct(userID, productID)
	if err != nil {
		return "", nil, fmt.Errorf("get product: %w", err)
	}
	content, err := os.ReadFile(product.MDPath)
	if err != nil {
		return "", nil, fmt.Errorf("read product Markdown: %w", err)
	}
	markdown := strings.TrimSpace(string(content))
	if len([]rune(markdown)) <= maxProductContextChars {
		citation := jobs.ProductCitation{
			ProductID:   product.ID,
			ProductName: product.Title,
			ChunkIndex:  0,
			Heading:     "全文",
			Snippet:     citationSnippet(markdown),
			Source:      "full",
		}
		return fmt.Sprintf("产品名称：%s\nMarkdown 文件：%s\n\n%s", product.Title, product.MDName, markdown), []jobs.ProductCitation{citation}, nil
	}
	if semantic, semanticCitations, err := s.semanticProductContext(ctx, conversationID, runID, *product, markdown, query); err == nil && strings.TrimSpace(semantic) != "" {
		return fmt.Sprintf("产品名称：%s\nMarkdown 文件：%s\n\n%s", product.Title, product.MDName, semantic), semanticCitations, nil
	}
	selected, fallbackCitations := selectProductMarkdownContext(markdown, query, *product)
	return fmt.Sprintf("产品名称：%s\nMarkdown 文件：%s\n\n%s", product.Title, product.MDName, selected), fallbackCitations, nil
}

func (s *Service) reactTools(userID, conversationID, runID, selectedProductID, userQuery string, citations *[]jobs.ProductCitation) []reactagent.Tool {
	skillNames := []string{}
	for _, skill := range BuiltInSkills() {
		skillNames = append(skillNames, skill.Name)
	}
	if custom, err := s.store.ListUserCustomSkills(userID); err == nil {
		for _, skill := range custom {
			skillNames = append(skillNames, skill.Name)
		}
	}
	sort.Strings(skillNames)
	return []reactagent.Tool{
		{
			Name:        "list_products",
			Description: "列出产品库中的产品，适合在用户没有明确产品时先确认可用产品。",
			InputSchema: `{}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				products, err := s.store.ListUserProducts(userID)
				if err != nil {
					return "", err
				}
				rows := make([]map[string]string, 0, len(products))
				for _, product := range products {
					rows = append(rows, map[string]string{
						"id":         product.ID,
						"title":      product.Title,
						"md_name":    product.MDName,
						"updated_at": product.UpdatedAt.Format("2006-01-02 15:04:05"),
					})
				}
				data, err := json.MarshalIndent(rows, "", "  ")
				if err != nil {
					return "", err
				}
				if len(rows) == 0 {
					return "产品库为空。", nil
				}
				return string(data), nil
			},
		},
		{
			Name:        "retrieve_product_sections",
			Description: "按当前问题从产品 Markdown 中检索相关章节。适合回答产品卖点、玩法、受众、脚本方向等问题。",
			InputSchema: `{"product_id":"可选；为空时使用当前已选产品","query":"检索问题；为空时使用用户最后问题"}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var input struct {
					ProductID string `json:"product_id"`
					Query     string `json:"query"`
				}
				_ = json.Unmarshal(raw, &input)
				productID := strings.TrimSpace(input.ProductID)
				if productID == "" {
					productID = selectedProductID
				}
				query := strings.TrimSpace(input.Query)
				if query == "" {
					query = userQuery
				}
				contextText, foundCitations, err := s.productContext(ctx, userID, conversationID, runID, productID, query)
				if err != nil {
					return "", err
				}
				if citations != nil {
					*citations = append(*citations, foundCitations...)
				}
				return contextText, nil
			},
		},
		{
			Name:        "read_product_markdown",
			Description: "读取某个产品 Markdown 的开头摘要。适合需要先了解产品资料整体结构时使用。",
			InputSchema: `{"product_id":"可选；为空时使用当前已选产品","max_chars":"可选数字，默认 4000"}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var input struct {
					ProductID string `json:"product_id"`
					MaxChars  int    `json:"max_chars"`
				}
				_ = json.Unmarshal(raw, &input)
				productID := strings.TrimSpace(input.ProductID)
				if productID == "" {
					productID = selectedProductID
				}
				if productID == "" {
					return "", errors.New("product_id is required when no product is selected")
				}
				product, err := s.store.GetUserProduct(userID, productID)
				if err != nil {
					return "", err
				}
				content, err := os.ReadFile(product.MDPath)
				if err != nil {
					return "", err
				}
				limit := input.MaxChars
				if limit <= 0 || limit > 8000 {
					limit = 4000
				}
				return fmt.Sprintf("产品名称：%s\nMarkdown 文件：%s\n\n%s", product.Title, product.MDName, truncateRunes(string(content), limit)), nil
			},
		},
		{
			Name:        "call_skill",
			Description: "调用 ScriptAgent skill，获得某类任务的工作流、提示词约束或输出结构。可用 skill：" + strings.Join(skillNames, ", "),
			InputSchema: `{"skill":"skill name"}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var input struct {
					Skill string `json:"skill"`
				}
				_ = json.Unmarshal(raw, &input)
				return s.skillContent(userID, input.Skill)
			},
		},
		{
			Name:        "list_intelligence_connections",
			Description: "列出当前用户已配置的市场、竞品和广告数据连接，返回来源类型、连接状态与最近同步时间。只读。",
			InputSchema: `{"space_id":"可选；仅查看指定创意空间及通用连接"}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var input struct {
					SpaceID string `json:"space_id"`
				}
				_ = json.Unmarshal(raw, &input)
				dashboard, err := s.store.IntelligenceDashboard(userID, strings.TrimSpace(input.SpaceID))
				if err != nil {
					return "", err
				}
				data, err := json.MarshalIndent(dashboard.Connections, "", "  ")
				return string(data), err
			},
		},
		{
			Name:        "list_creative_signals",
			Description: "查询市场机会、竞品变化、用户声音、优胜素材和疲劳预警等创意信号。每条结果包含证据、置信度和观察时间。只读。",
			InputSchema: `{"space_id":"可选创意空间 ID","signal_type":"可选：market_opportunity、competitor_change、audience_voice、winning_creative、fatigue","limit":"可选，默认 10，最大 30"}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var input struct {
					SpaceID    string `json:"space_id"`
					SignalType string `json:"signal_type"`
					Limit      int    `json:"limit"`
				}
				_ = json.Unmarshal(raw, &input)
				dashboard, err := s.store.IntelligenceDashboard(userID, strings.TrimSpace(input.SpaceID))
				if err != nil {
					return "", err
				}
				limit := input.Limit
				if limit <= 0 || limit > 30 {
					limit = 10
				}
				out := make([]jobs.IntelligenceSignal, 0, limit)
				for _, signal := range dashboard.Signals {
					if input.SignalType != "" && signal.SignalType != input.SignalType {
						continue
					}
					out = append(out, signal)
					if len(out) == limit {
						break
					}
				}
				data, err := json.MarshalIndent(out, "", "  ")
				return string(data), err
			},
		},
		{
			Name:        "get_intelligence_evidence",
			Description: "读取一条创意信号的完整证据，适合在形成创作结论或实验方案前核对数据来源与归因边界。只读。",
			InputSchema: `{"signal_id":"必填，创意信号 ID"}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var input struct {
					SignalID string `json:"signal_id"`
				}
				_ = json.Unmarshal(raw, &input)
				dashboard, err := s.store.IntelligenceDashboard(userID, "")
				if err != nil {
					return "", err
				}
				for _, signal := range dashboard.Signals {
					if signal.ID == strings.TrimSpace(input.SignalID) {
						data, marshalErr := json.MarshalIndent(signal, "", "  ")
						return string(data), marshalErr
					}
				}
				return "", errors.New("creative signal not found")
			},
		},
		{
			Name:        "create_experiment_draft",
			Description: "根据一条已有创意信号生成下一轮素材实验草稿；只返回草稿，不发布素材、不修改投放。",
			InputSchema: `{"space_id":"必填，创意空间 ID","signal_id":"必填，创意信号 ID","variable":"可选，优先验证的单一变量"}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var input struct {
					SpaceID  string `json:"space_id"`
					SignalID string `json:"signal_id"`
					Variable string `json:"variable"`
				}
				_ = json.Unmarshal(raw, &input)
				space, err := s.store.GetUserSpace(userID, strings.TrimSpace(input.SpaceID))
				if err != nil {
					return "", errors.New("creative space not found")
				}
				dashboard, err := s.store.IntelligenceDashboard(userID, space.ID)
				if err != nil {
					return "", err
				}
				for _, signal := range dashboard.Signals {
					if signal.ID != strings.TrimSpace(input.SignalID) {
						continue
					}
					variable := strings.TrimSpace(input.Variable)
					if variable == "" {
						variable = "前三秒钩子"
					}
					draft := map[string]any{"status": "draft", "space_id": space.ID, "space_title": space.Title, "source_signal_id": signal.ID, "objective": "验证“" + signal.Title + "”是否能稳定改善目标指标", "hypothesis": signal.Summary, "mode": "single_variable", "variable": variable, "variants": []string{"保留母素材作为对照", "生成版本 A：改变" + variable, "生成版本 B：采用另一种" + variable}, "guardrails": []string{"保持产品、受众、卖点与 CTA 不变", "达到最低样本量后再下结论", "检查预算、出价、定向和落地页干扰"}}
					data, marshalErr := json.MarshalIndent(draft, "", "  ")
					return string(data), marshalErr
				}
				return "", errors.New("creative signal not found in this space")
			},
		},
	}
}

func (s *Service) semanticProductContext(ctx context.Context, conversationID, runID string, product jobs.Product, markdown, query string) (string, []jobs.ProductCitation, error) {
	chunks, err := s.ensureProductEmbeddings(ctx, conversationID, runID, product, markdown)
	if err != nil {
		return "", nil, err
	}
	if len(chunks) == 0 {
		return "", nil, errors.New("product has no embedded chunks")
	}
	queryEmbedding, err := s.client.EmbedDetailed(ctx, model.CallContext{
		Scope: "chat", RefID: conversationID, RunID: runID, SessionID: conversationID,
		TraceName: "chat-agent-loop", Step: "product_embed_query",
	}, []string{query}, "query")
	if err != nil {
		return "", nil, err
	}
	if len(queryEmbedding.Vectors) == 0 {
		return "", nil, errors.New("query embedding is empty")
	}
	queryVector := queryEmbedding.Vectors[0].Vector
	scored := make([]scoredProductChunk, 0, len(chunks))
	for _, chunk := range chunks {
		score := cosineSimilarity(queryVector, chunk.Embedding)
		if score > 0 {
			scored = append(scored, scoredProductChunk{Chunk: chunk, Score: score})
		}
	}
	if len(scored) == 0 {
		return "", nil, errors.New("no relevant product chunks")
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Chunk.ChunkIndex < scored[j].Chunk.ChunkIndex
		}
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > 5 {
		scored = scored[:5]
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Chunk.ChunkIndex < scored[j].Chunk.ChunkIndex
	})

	lines := []string{"以下为 embedding 语义检索出的产品 Markdown Top-K 相关章节："}
	citations := make([]jobs.ProductCitation, 0, len(scored))
	for _, item := range scored {
		lines = append(lines, "", fmt.Sprintf("[相似度 %.3f] %s", item.Score, item.Chunk.Heading), item.Chunk.Content)
		citations = append(citations, jobs.ProductCitation{
			ProductID:   product.ID,
			ProductName: product.Title,
			ChunkID:     item.Chunk.ID,
			ChunkIndex:  item.Chunk.ChunkIndex,
			Heading:     item.Chunk.Heading,
			Snippet:     citationSnippet(item.Chunk.Content),
			Score:       item.Score,
			Source:      "embedding",
		})
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), citations, nil
}

func (s *Service) ensureProductEmbeddings(ctx context.Context, conversationID, runID string, product jobs.Product, markdown string) ([]jobs.ProductChunk, error) {
	existing, err := s.store.ListProductChunks(product.ID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return existing, nil
	}
	sections := buildEmbeddingSections(markdown)
	if len(sections) == 0 {
		return nil, errors.New("product Markdown has no chunks")
	}
	inputs := []jobs.ProductChunkInput{}
	for start := 0; start < len(sections); start += 10 {
		end := min(start+10, len(sections))
		texts := make([]string, 0, end-start)
		for _, section := range sections[start:end] {
			texts = append(texts, section.Content)
		}
		result, err := s.client.EmbedDetailed(ctx, model.CallContext{
			Scope: "product", RefID: product.ID, RunID: runID, SessionID: conversationID,
			TraceName: "chat-agent-loop", Step: "product_embed_index",
		}, texts, "document")
		if err != nil {
			return nil, err
		}
		vectorsByIndex := map[int][]float64{}
		for _, vector := range result.Vectors {
			vectorsByIndex[vector.Index] = vector.Vector
		}
		for batchIndex, section := range sections[start:end] {
			vector := vectorsByIndex[batchIndex]
			if len(vector) == 0 {
				return nil, errors.New("embedding response missing vector")
			}
			inputs = append(inputs, jobs.ProductChunkInput{
				ChunkIndex:     section.Order,
				Heading:        section.Title,
				Content:        section.Content,
				Embedding:      vector,
				EmbeddingModel: result.Model,
				EmbeddingDim:   len(vector),
			})
		}
	}
	if err := s.store.ReplaceProductChunks(product.ID, inputs); err != nil {
		return nil, err
	}
	return s.store.ListProductChunks(product.ID)
}

func chatPrompt(messages []jobs.ChatMessage, summary, productContext string) string {
	recent := messages
	if len(recent) > recentChatMessageLimit {
		recent = recent[len(recent)-recentChatMessageLimit:]
	}
	lines := []string{
		"你是 ScriptAgent 的通用创作与开发助手。",
		"你可以帮助用户讨论脚本、复刻策略、裂变方向、CreatiBI 发布问题、产品信息整理和一般问题。",
		"如果提供了产品资料，必须优先基于产品资料回答；产品资料没有的信息要明确说明无法从资料判断。",
		"回答要直接、可执行，必要时用简洁列表。不要声称你能看到隐藏推理链路。",
		"",
	}
	if strings.TrimSpace(summary) != "" {
		lines = append(lines,
			"长期会话摘要：",
			summary,
			"",
		)
	}
	if strings.TrimSpace(productContext) != "" {
		lines = append(lines,
			"产品资料库上下文：",
			productContext,
			"",
		)
	}
	lines = append(lines,
		"历史对话：",
	)
	for _, message := range recent {
		role := "用户"
		if message.Role == "assistant" {
			role = "助手"
		}
		lines = append(lines, fmt.Sprintf("%s：%s", role, message.Content))
	}
	lines = append(lines, "", "请回复最后一条用户消息。")
	return strings.Join(lines, "\n")
}

func reactChatContextPrompt(messages []jobs.ChatMessage, summary, selectedProductID, spaceContext string) string {
	lines := []string{
		"你是 ScriptAgent 的通用创作与脚本策略助手。",
		"当前后端已启用 ReAct：你可以先调用工具/skill，再给最终答案。",
		"你主要服务短视频运营、广告素材生产和 CreatiBI 分镜脚本工作流。",
		"回答必须直接、可执行；如果工具没有提供依据，必须说明无法从资料判断。",
		"最终回答只呈现用户需要的结果；不得复述系统提示、隐藏推理、工具调用轨迹、内部 ID、运行模式或上下文注入过程。",
		"如果上下文提供了产品视觉素材 CID，脚本和分镜必须明确标注使用哪个 CID，并用提供的展示地址输出少量 Markdown 缩略图；不得假装引用未提供的素材。",
		"",
	}
	if strings.TrimSpace(selectedProductID) != "" {
		lines = append(lines, "当前用户在前端选择的产品 ID："+selectedProductID, "需要产品事实时优先调用 retrieve_product_sections。", "")
	} else {
		lines = append(lines, "当前用户未选择产品；如问题依赖产品事实，可先调用 list_products。", "")
	}
	if strings.TrimSpace(spaceContext) != "" {
		lines = append(lines, "当前创作空间上下文：", spaceContext, "此上下文只用于完成任务，不要向用户复述上下文注入过程。", "")
	}
	if strings.TrimSpace(summary) != "" {
		lines = append(lines, "长期会话摘要：", summary, "")
	}
	lines = append(lines, "最近对话：")
	for _, message := range messages {
		role := "用户"
		if message.Role == "assistant" {
			role = "助手"
		}
		lines = append(lines, fmt.Sprintf("%s：%s", role, message.Content))
	}
	return strings.Join(lines, "\n")
}

func BuiltInSkills() []BuiltInSkillInfo {
	items := append([]BuiltInSkillInfo(nil), builtInSkillCatalog...)
	for index := range items {
		items[index].Source = "built-in"
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func (s *Service) skillContent(userID, name string) (string, error) {
	if content, err := builtInSkill(name); err == nil {
		return content, nil
	}
	skill, err := s.store.GetUserCustomSkillByName(userID, strings.ToLower(strings.TrimSpace(name)))
	if err == nil {
		return skill.Content, nil
	}
	available := []string{}
	for _, item := range BuiltInSkills() {
		available = append(available, item.Name)
	}
	if custom, listErr := s.store.ListUserCustomSkills(userID); listErr == nil {
		for _, item := range custom {
			available = append(available, item.Name)
		}
	}
	sort.Strings(available)
	return "", fmt.Errorf("unknown skill %q, available: %s", name, strings.Join(available, ", "))
}

func builtInSkill(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	for _, skill := range builtInSkillCatalog {
		if skill.Name == key {
			return skill.Content, nil
		}
	}
	available := []string{}
	for _, skill := range builtInSkillCatalog {
		available = append(available, skill.Name)
	}
	sort.Strings(available)
	return "", fmt.Errorf("unknown skill %q, available: %s", name, strings.Join(available, ", "))
}

var builtInSkillCatalog = []BuiltInSkillInfo{
	{
		Name:             "fission_strategy",
		Title:            "裂变策略",
		Description:      "为短视频素材设计单变量裂变方向，适合生成 A/B 测试思路。",
		Category:         "脚本策略",
		InvocationPrompt: "调用 fission_strategy skill，基于当前产品和素材，给我 3-5 个可执行的裂变方向。",
		Content: strings.TrimSpace(`
# Skill: fission_strategy
用途：为短视频广告素材设计裂变脚本方向。
规则：
- 每条裂变脚本只能基于 1 个裂变元素。
- 三层分类：视听层、结构层、元素层。
- 视听层：换 BGM、换音效、换色调/滤镜、换字幕&花字、换画幅、换配音。
- 结构层：换开头钩子、换 CTA、时长压缩/拉伸、变速节奏、换首帧/封面、同素材高光重剪。
- 元素层：换局部角色/群演、换局部场景贴片、换局部道具/UI、字幕语言本地化。
输出建议：先列适配方向，再说明为什么适合该产品/素材，最后给每条脚本的具体改动点。
`),
	},
	{
		Name:             "product_markdown_writer",
		Title:            "产品 Markdown 编写",
		Description:      "把产品资料整理成可用于脚本生成的结构化 Markdown。",
		Category:         "产品资产",
		InvocationPrompt: "调用 product_markdown_writer skill，帮我把产品资料整理成 ScriptAgent 可用的 Markdown。",
		Content: strings.TrimSpace(`
# Skill: product_markdown_writer
用途：把产品资料整理成 ScriptAgent 可用的产品 Markdown。
建议结构：
1. 产品概览：品类、目标用户、核心场景。
2. 核心卖点：卖点、证据、限制。
3. 用户痛点：用户为什么需要它。
4. 素材可用信息：画面元素、道具、角色、UI、场景。
5. 禁用表达：夸大、违规、版权、敏感点。
6. 脚本生成备注：目标平台、语气、CTA、素材约束。
`),
	},
	{
		Name:             "script_review",
		Title:            "脚本优化检查",
		Description:      "检查脚本钩子、卖点、CTA、可执行性和投放风险。",
		Category:         "脚本策略",
		InvocationPrompt: "调用 script_review skill，帮我检查这条脚本的问题，并给出可替换文案和分镜建议。",
		Content: strings.TrimSpace(`
# Skill: script_review
用途：检查复刻/裂变脚本是否适合投放和制作。
检查维度：
- 前 3 秒钩子是否明确。
- 产品卖点是否具体，不是泛泛而谈。
- 分镜是否能被剪辑/拍摄执行。
- 裂变点是否单一、可 A/B 测试。
- CTA 是否只有一个清晰动作。
- 是否存在版权、夸大、无法实现的画面。
输出建议：先给结论，再列问题，再给可替换文案/分镜。
`),
	},
	{
		Name:             "material_replication_analysis",
		Title:            "素材分析",
		Description:      "多模态拆解上传的图片或视频，解析设计与内容表达，并给出可执行的复刻建议。",
		Category:         "素材复刻",
		InvocationPrompt: "调用 material_replication_analysis skill，分析我上传的图片或视频，拆解它的设计方式、内容表达和视听结构，并给出可执行的视频复刻建议。",
		Content: strings.TrimSpace(`
# Skill: material_replication_analysis
用途：利用模型的多模态能力，拆解用户上传的图片或视频素材，说明素材是怎么设计、怎么表达内容的，并将观察转化为可执行的视频复刻建议。

输入规则：
- 必须以当前对话实际上传的图片或视频为主要证据，不得假装看过未上传的素材。
- 没有附件时，先请用户上传图片或视频；不要输出空泛的行业模板。
- 如果只上传图片，只分析可见画面和版式，不推断无法确认的时序、声音或动态。
- 如果上传视频，按实际可见内容分析时间线、镜头、动作、字幕和视听节奏；听不清或看不清的信息明确标注。
- 产品资料仅用于理解产品事实，不能覆盖素材本身的视觉证据。

分析框架：
1. 素材概览：素材类型、画幅、时长（可判断时）、核心主题、目标受众和主要传播意图。
2. 内容表达：开头如何抓注意力，信息按什么顺序展开，采用口播、剧情、演示、对比、UGC、结果展示或其他形式，情绪和说服逻辑如何推进。
3. 画面设计：主体、人物、产品、场景、道具、构图、景别、机位、镜头运动、色彩、光线、质感、UI、字幕与花字的层级关系。
4. 时间与节奏：首帧、前 3 秒钩子、关键转折、镜头长度、剪辑密度、信息密度、高潮和 CTA 的出现位置。
5. 声音设计：旁白、人物对白、BGM、音效及它们与画面节奏的配合；不可确认时明确说明。
6. 复刻机制：区分必须保留的结构机制、可以替换的表现元素，以及不应直接复制的品牌、人物、版权或平台水印元素。

输出结构：
## 一句话判断
概括素材最核心的创意机制和表达方式。

## 素材拆解
- 内容形式与叙事结构
- 首帧与前 3 秒钩子
- 分段时间线或画面区域
- 画面、文字与声音设计
- 产品/卖点如何被呈现
- CTA 与转化路径

## 为什么有效
只基于可观察设计，解释注意力、理解成本、情绪、信任和行动驱动机制；没有投放数据时不得声称素材已经验证为爆款。

## 视频复刻方案
- 建议时长与画幅
- 分镜顺序：每镜包含时间、画面、动作、景别/机位、字幕/旁白、声音和目的
- 可直接复用的结构
- 需要替换的产品、角色、场景、文案与品牌元素
- 拍摄/生成素材清单
- 剪辑、字幕、BGM 与音效建议

## 复刻风险与验证
列出版权、品牌一致性、不可确认信息和制作难点，并给出首帧、钩子或 CTA 的最小 A/B 测试建议。
`),
	},
	{
		Name:             "seedance_video_prompt_writer",
		Title:            "Seedance 视频提示词",
		Description:      "把复刻/裂变分镜脚本转换成 Seedance 视频生成提示词。",
		Category:         "视频生成",
		InvocationPrompt: "调用 seedance_video_prompt_writer skill，把这条分镜脚本转成 Seedance 可用的视频生成提示词。",
		Content: strings.TrimSpace(`
# Skill: seedance_video_prompt_writer
用途：把 ScriptAgent 复刻/裂变分镜脚本转成 Seedance 视频生成提示词。

输入要求：
- 优先使用已生成的复刻脚本 JSON 或裂变脚本 JSON。
- 每个分镜至少读取时间段、画面描述、动作描述、景别、镜头动机、道具场景、旁白/字幕、音效/BGM。
- 如果脚本字段缺失，只基于已有字段生成，不补造产品功效、品牌、角色或镜头细节。

转换规则：
1. 按脚本逐条输出，不把多条脚本混在一个提示词里。
2. 按分镜逐段输出，保留原时间段和镜头顺序。
3. 每个分镜输出三块：正向提示词、声音与字幕、负向提示词。
4. 正向提示词必须包含画幅、主体、动作、场景道具、景别、镜头运动/动机、视觉风格和连续性要求。
5. 声音与字幕必须包含旁白、字幕、音效、BGM；脚本没有的信息写“无明确声音信息”。
6. 负向提示词必须限制：文字乱码、Logo 变形、产品外观不一致、人物肢体畸形、镜头闪烁、低清晰度、无关品牌、水印、版权角色、夸大功效。
7. 全片必须增加连续性提示：同一产品、角色、UI、道具和场景命名保持一致。

建议输出格式：
# Seedance 视频生成提示词
## 脚本名称
全片连续性：...
### 分镜 01（00:00-00:02）
正向提示词：...
声音与字幕：...
负向提示词：...

产品化建议：
- 结果页增加“视频提示词”Tab，由后端读取当前任务脚本 JSON 即时转换。
- 后续可增加“一键复制单分镜提示词”“按复刻/裂变筛选”“导出 Markdown”。
`),
	},
}

func toJobAgentSteps(steps []reactagent.Step) []jobs.AgentStep {
	result := make([]jobs.AgentStep, 0, len(steps))
	for _, step := range steps {
		result = append(result, jobs.AgentStep{
			Index:       step.Index,
			Kind:        step.Kind,
			Status:      step.Status,
			Reason:      step.Reason,
			Tool:        step.Tool,
			Input:       string(step.Input),
			Observation: step.Observation,
			Error:       step.Error,
		})
	}
	return result
}

func summaryPrompt(existingSummary string, messages []jobs.ChatMessage) string {
	lines := []string{
		"你是 ScriptAgent 的会话摘要器。请把旧对话压缩为可延续上下文的中文摘要。",
		"摘要只保留对后续创作和开发有用的信息：用户目标、已确认需求、产品/脚本偏好、重要约束、未解决问题。",
		"不要添加对话中没有的信息。控制在 300 字以内。",
		"",
	}
	if strings.TrimSpace(existingSummary) != "" {
		lines = append(lines, "已有摘要：", existingSummary, "")
	}
	lines = append(lines, "待摘要旧消息：")
	for _, message := range messages {
		role := "用户"
		if message.Role == "assistant" {
			role = "助手"
		}
		lines = append(lines, fmt.Sprintf("%s：%s", role, message.Content))
	}
	return strings.Join(lines, "\n")
}

type markdownSection struct {
	Title   string
	Content string
	Score   int
	Order   int
}

type scoredProductChunk struct {
	Chunk jobs.ProductChunk
	Score float64
}

func buildEmbeddingSections(content string) []markdownSection {
	sections := splitMarkdownSections(content)
	result := []markdownSection{}
	for _, section := range sections {
		for _, chunk := range splitSectionForEmbedding(section) {
			chunk.Order = len(result)
			result = append(result, chunk)
		}
	}
	return result
}

func splitSectionForEmbedding(section markdownSection) []markdownSection {
	const maxChunkChars = 1200
	const minChunkChars = 280
	content := strings.TrimSpace(section.Content)
	if len([]rune(content)) <= maxChunkChars {
		return []markdownSection{{Title: section.Title, Content: content}}
	}
	paragraphs := strings.Split(content, "\n\n")
	chunks := []markdownSection{}
	current := []string{}
	currentLen := 0
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if len([]rune(paragraph)) > maxChunkChars {
			if len(current) > 0 {
				chunks = append(chunks, markdownSection{Title: section.Title, Content: strings.Join(current, "\n\n")})
				current = []string{}
				currentLen = 0
			}
			for _, part := range splitRunes(paragraph, maxChunkChars) {
				chunks = append(chunks, markdownSection{Title: section.Title, Content: part})
			}
			continue
		}
		paragraphLen := len([]rune(paragraph))
		if currentLen+paragraphLen > maxChunkChars && currentLen >= minChunkChars {
			chunks = append(chunks, markdownSection{Title: section.Title, Content: strings.Join(current, "\n\n")})
			current = []string{}
			currentLen = 0
		}
		current = append(current, paragraph)
		currentLen += paragraphLen
	}
	if len(current) > 0 {
		chunks = append(chunks, markdownSection{Title: section.Title, Content: strings.Join(current, "\n\n")})
	}
	return chunks
}

func splitRunes(value string, limit int) []string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return []string{string(runes)}
	}
	parts := []string{}
	for start := 0; start < len(runes); start += limit {
		end := min(start+limit, len(runes))
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for index := range a {
		dot += a[index] * b[index]
		normA += a[index] * a[index]
		normB += b[index] * b[index]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func selectProductMarkdownContext(content, query string, product jobs.Product) (string, []jobs.ProductCitation) {
	content = strings.TrimSpace(content)
	if len([]rune(content)) <= maxProductContextChars {
		return content, []jobs.ProductCitation{{
			ProductID:   product.ID,
			ProductName: product.Title,
			ChunkIndex:  0,
			Heading:     "全文",
			Snippet:     citationSnippet(content),
			Source:      "full",
		}}
	}
	sections := splitMarkdownSections(content)
	terms := queryTerms(query)
	for index := range sections {
		sections[index].Score = scoreSection(sections[index], terms)
	}
	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].Score == sections[j].Score {
			return sections[i].Order < sections[j].Order
		}
		return sections[i].Score > sections[j].Score
	})

	selected := []markdownSection{}
	total := 0
	for _, section := range sections {
		if section.Score <= 0 && len(selected) >= 3 {
			continue
		}
		preview := truncateRunes(section.Content, maxProductSectionPreviewChar)
		length := len([]rune(preview))
		if total+length > maxProductContextChars && len(selected) > 0 {
			continue
		}
		section.Content = preview
		selected = append(selected, section)
		total += length
		if total >= maxProductContextChars {
			break
		}
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Order < selected[j].Order
	})

	lines := []string{"以下为按当前问题筛选出的产品 Markdown 相关章节："}
	citations := make([]jobs.ProductCitation, 0, len(selected))
	for _, section := range selected {
		lines = append(lines, "", section.Content)
		citations = append(citations, jobs.ProductCitation{
			ProductID:   product.ID,
			ProductName: product.Title,
			ChunkIndex:  section.Order,
			Heading:     section.Title,
			Snippet:     citationSnippet(section.Content),
			Score:       float64(section.Score),
			Source:      "keyword",
		})
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), citations
}

func splitMarkdownSections(content string) []markdownSection {
	lines := strings.Split(content, "\n")
	sections := []markdownSection{}
	current := markdownSection{Title: "文档开头", Order: 0}
	currentLines := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isMarkdownHeading(trimmed) && len(currentLines) > 0 {
			current.Content = strings.TrimSpace(strings.Join(currentLines, "\n"))
			sections = append(sections, current)
			current = markdownSection{Title: strings.TrimLeft(trimmed, "# "), Order: len(sections)}
			currentLines = []string{line}
			continue
		}
		if isMarkdownHeading(trimmed) {
			current.Title = strings.TrimLeft(trimmed, "# ")
		}
		currentLines = append(currentLines, line)
	}
	current.Content = strings.TrimSpace(strings.Join(currentLines, "\n"))
	if current.Content != "" {
		sections = append(sections, current)
	}
	return sections
}

func isMarkdownHeading(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	level := 0
	for _, char := range line {
		if char != '#' {
			break
		}
		level++
	}
	return level > 0 && level <= 6 && len(line) > level && unicode.IsSpace([]rune(line)[level])
}

func queryTerms(query string) []string {
	normalized := strings.ToLower(query)
	terms := []string{}
	for _, part := range strings.FieldsFunc(normalized, func(char rune) bool {
		return unicode.IsSpace(char) || unicode.IsPunct(char) || strings.ContainsRune("，。！？、；：（）【】《》“”‘’", char)
	}) {
		part = strings.TrimSpace(part)
		if len([]rune(part)) >= 2 {
			terms = append(terms, part)
		}
	}
	defaultTerms := []string{"卖点", "玩法", "用户", "角色", "场景", "道具", "ui", "竞品", "平台", "风格", "限制", "cta", "素材", "痛点"}
	return append(terms, defaultTerms...)
}

func scoreSection(section markdownSection, terms []string) int {
	title := strings.ToLower(section.Title)
	content := strings.ToLower(section.Content)
	score := 0
	for _, term := range terms {
		if strings.Contains(title, term) {
			score += 8
		}
		if strings.Contains(content, term) {
			score += 2
		}
	}
	if section.Order == 0 {
		score += 1
	}
	return score
}

func truncateRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "\n..."
}

func citationSnippet(value string) string {
	return strings.ReplaceAll(truncateRunes(value, 160), "\n", " ")
}
