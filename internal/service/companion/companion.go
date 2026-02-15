package companion

import (
	"OpenAIClient/internal/service/image"
	"context"
)

type ConversationAdapter interface {
	NewConversation(ctx context.Context, systemText string, contextText string, metadata map[string]string) (string, error)
}

type MessageAdapter interface {
	// Stateless отправка сообщения: systemText — системные инструкции/характер,
	// startPrompt — стартовый промпт диалога (добавляется в user-сообщение),
	// text — текущий пользовательский ввод, responseHistory — список прошлых ответов ИИ (как есть),
	// images — текущие изображения.
	SendTextWithImage(ctx context.Context, systemText string, startPrompt string, text string, responseHistory []string, images []image.ProcessedImage) (string, error)
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
func (c *Companion) SendMessageWithImage(ctx context.Context, systemText string, startPrompt string, text string, responseHistory []string, images []image.ProcessedImage) (string, error) {
	return c.messages.SendTextWithImage(ctx, systemText, startPrompt, text, responseHistory, images)
}
