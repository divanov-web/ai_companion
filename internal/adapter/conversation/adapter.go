package conversation

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/conversations"
)

type Adapter struct {
	client *openai.Client
}

func New(client *openai.Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) NewConversation(ctx context.Context) (string, error) {
	conv, err := a.client.Conversations.New(ctx, conversations.ConversationNewParams{})
	if err != nil {
		return "", err
	}

	return conv.ID, nil
}
