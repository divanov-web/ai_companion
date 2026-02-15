package companion

import (
	"OpenAIClient/internal/service/image"
	"context"
)

type ConversationAdapter interface {
	NewConversation(ctx context.Context, systemText string, contextText string, metadata map[string]string) (string, error)
}

type MessageAdapter interface {
	SendTextWithImage(ctx context.Context, conversationID string, systemText string, text string, images []image.ProcessedImage) (string, error)
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
func (c *Companion) StartConversation(ctx context.Context, systemText string, contextText string, metadata map[string]string) (string, error) {
	return c.conversations.NewConversation(ctx, systemText, contextText, metadata)
}

// SendMessageWithImage отправляет сообщение с картинкой.
func (c *Companion) SendMessageWithImage(ctx context.Context, conversationID string, systemText string, text string, images []image.ProcessedImage) (string, error) {
	return c.messages.SendTextWithImage(ctx, conversationID, systemText, text, images)
}
