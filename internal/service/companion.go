package service

import "context"

type ConversationAdapter interface {
	NewConversation(ctx context.Context) (string, error)
}

type MessageAdapter interface {
	SendTextWithImage(ctx context.Context, conversationID string, text string, imageDataURL string) (string, error)
}

type Companion struct {
	conversations ConversationAdapter
	messages      MessageAdapter
}

func NewCompanion(conversations ConversationAdapter, messages MessageAdapter) *Companion {
	return &Companion{conversations: conversations, messages: messages}
}

func (c *Companion) StartConversation(ctx context.Context) (string, error) {
	return c.conversations.NewConversation(ctx)
}

func (c *Companion) SendMessageWithImage(ctx context.Context, conversationID string, text string, imageDataURL string) (string, error) {
	return c.messages.SendTextWithImage(ctx, conversationID, text, imageDataURL)
}
