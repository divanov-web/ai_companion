package service

import (
	"OpenAIClient/internal/service/image"
	"context"
)

type ConversationAdapter interface {
	NewConversation(ctx context.Context, contextText string, metadata map[string]string) (string, error)
}

type MessageAdapter interface {
	SendTextWithImage(ctx context.Context, conversationID string, text string, images []image.ProcessedImage) (string, error)
}

type Companion struct {
	conversations ConversationAdapter
	messages      MessageAdapter
}

// NewCompanion создаёт сервис оркестрации.
func NewCompanion(conversations ConversationAdapter, messages MessageAdapter) *Companion {
	return &Companion{conversations: conversations, messages: messages}
}

// StartConversation создаёт новый диалог.
func (c *Companion) StartConversation(ctx context.Context, contextText string, metadata map[string]string) (string, error) {
	return c.conversations.NewConversation(ctx, contextText, metadata)
}

// SendMessageWithImage отправляет сообщение с картинкой.
func (c *Companion) SendMessageWithImage(ctx context.Context, conversationID string, text string, images []image.ProcessedImage) (string, error) {
	return c.messages.SendTextWithImage(ctx, conversationID, text, images)
}
