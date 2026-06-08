package chat

import (
	"context"
	"errors"
	"fmt"
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

func (s *Service) Send(ctx context.Context, conversationID, content string) (*jobs.ChatThread, error) {
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

	reply, err := s.client.GenerateDetailed(ctx, model.CallContext{
		Scope: "chat",
		RefID: conversationID,
		Step:  "chat_reply",
	}, []model.ContentItem{{Text: chatPrompt(messages)}})
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

func chatPrompt(messages []jobs.ChatMessage) string {
	recent := messages
	if len(recent) > 12 {
		recent = recent[len(recent)-12:]
	}
	lines := []string{
		"你是 ScriptAgent 的通用创作与开发助手。",
		"你可以帮助用户讨论脚本、复刻策略、裂变方向、CreatiBI 发布问题、产品信息整理和一般问题。",
		"回答要直接、可执行，必要时用简洁列表。不要声称你能看到隐藏推理链路。",
		"",
		"历史对话：",
	}
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
