package chat

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
)

const (
	recentChatMessageLimit       = 12
	summaryTriggerMessageCount   = 16
	summaryTailMessageCount      = 8
	maxProductContextChars       = 6000
	maxProductSectionPreviewChar = 1800
)

type Service struct {
	store  *jobs.Store
	client *model.DashScopeClient
}

func NewService(store *jobs.Store, client *model.DashScopeClient) *Service {
	return &Service{store: store, client: client}
}

func (s *Service) Send(ctx context.Context, conversationID, content, productID string) (*jobs.ChatThread, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("message content is required")
	}
	if s.client == nil {
		return nil, errors.New("chat model is not configured")
	}

	var conversation *jobs.ChatConversation
	var messages []jobs.ChatMessage
	var err error
	if conversationID == "" {
		conversation, err = s.store.CreateChatConversation(content)
		if err != nil {
			return nil, err
		}
		conversationID = conversation.ID
	} else {
		thread, err := s.store.GetChatThread(conversationID)
		if err != nil {
			return nil, err
		}
		conversation = &thread.Conversation
		messages = thread.Messages
	}

	userMessage, err := s.store.AddChatMessage(conversationID, "user", content)
	if err != nil {
		return nil, err
	}
	messages = append(messages, *userMessage)

	summary := s.refreshSummary(ctx, conversationID, conversation, messages)
	productContext, citations, err := s.productContext(ctx, conversationID, productID, content)
	if err != nil {
		return nil, err
	}

	reply, err := s.client.GenerateDetailed(ctx, model.CallContext{
		Scope: "chat",
		RefID: conversationID,
		Step:  "chat_reply",
	}, []model.ContentItem{{Text: chatPrompt(messages, summary, productContext)}})
	if err != nil {
		return nil, err
	}
	if _, err := s.store.AddChatMessage(conversationID, "assistant", reply.Text); err != nil {
		return nil, err
	}
	thread, err := s.store.GetChatThread(conversationID)
	if err != nil {
		return nil, err
	}
	if conversation != nil {
		thread.Conversation.Title = conversation.Title
	}
	thread.Citations = citations
	return thread, nil
}

func (s *Service) refreshSummary(ctx context.Context, conversationID string, conversation *jobs.ChatConversation, messages []jobs.ChatMessage) string {
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

	result, err := s.client.GenerateDetailed(ctx, model.CallContext{
		Scope: "chat",
		RefID: conversationID,
		Step:  "chat_summary",
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

func (s *Service) productContext(ctx context.Context, conversationID, productID, query string) (string, []jobs.ProductCitation, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return "", nil, nil
	}
	product, err := s.store.GetProduct(productID)
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
	if semantic, semanticCitations, err := s.semanticProductContext(ctx, conversationID, *product, markdown, query); err == nil && strings.TrimSpace(semantic) != "" {
		return fmt.Sprintf("产品名称：%s\nMarkdown 文件：%s\n\n%s", product.Title, product.MDName, semantic), semanticCitations, nil
	}
	selected, fallbackCitations := selectProductMarkdownContext(markdown, query, *product)
	return fmt.Sprintf("产品名称：%s\nMarkdown 文件：%s\n\n%s", product.Title, product.MDName, selected), fallbackCitations, nil
}

func (s *Service) semanticProductContext(ctx context.Context, conversationID string, product jobs.Product, markdown, query string) (string, []jobs.ProductCitation, error) {
	chunks, err := s.ensureProductEmbeddings(ctx, product, markdown)
	if err != nil {
		return "", nil, err
	}
	if len(chunks) == 0 {
		return "", nil, errors.New("product has no embedded chunks")
	}
	queryEmbedding, err := s.client.EmbedDetailed(ctx, model.CallContext{
		Scope: "chat",
		RefID: conversationID,
		Step:  "product_embed_query",
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

func (s *Service) ensureProductEmbeddings(ctx context.Context, product jobs.Product, markdown string) ([]jobs.ProductChunk, error) {
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
			Scope: "product",
			RefID: product.ID,
			Step:  "product_embed_index",
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
