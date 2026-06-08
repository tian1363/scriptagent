package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
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

	productContext, err := s.productContext(productID)
	if err != nil {
		return nil, err
	}

	userMessage, err := s.store.AddChatMessage(conversationID, "user", content)
	if err != nil {
		return nil, err
	}
	messages = append(messages, *userMessage)

	reply, err := s.client.GenerateDetailed(ctx, model.CallContext{
		Scope: "chat",
		RefID: conversationID,
		Step:  "chat_reply",
	}, []model.ContentItem{{Text: chatPrompt(messages, productContext)}})
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
	return thread, nil
}

func (s *Service) productContext(productID string) (string, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return "", nil
	}
	product, err := s.store.GetProduct(productID)
	if err != nil {
		return "", fmt.Errorf("get product: %w", err)
	}
	content, err := os.ReadFile(product.MDPath)
	if err != nil {
		return "", fmt.Errorf("read product Markdown: %w", err)
	}
	return fmt.Sprintf("产品名称：%s\nMarkdown 文件：%s\n\n%s", product.Title, product.MDName, strings.TrimSpace(string(content))), nil
}

func chatPrompt(messages []jobs.ChatMessage, productContext string) string {
	recent := messages
	if len(recent) > 12 {
		recent = recent[len(recent)-12:]
	}
	lines := []string{
		"你是 ScriptAgent 的通用创作与开发助手。",
		"你可以帮助用户讨论脚本、复刻策略、裂变方向、CreatiBI 发布问题、产品信息整理和一般问题。",
		"如果提供了产品资料，必须优先基于产品资料回答；产品资料没有的信息要明确说明无法从资料判断。",
		"回答要直接、可执行，必要时用简洁列表。不要声称你能看到隐藏推理链路。",
		"",
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
