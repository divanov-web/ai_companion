package conversation

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/conversations"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

type Adapter struct {
	client *openai.Client
}

// New создаёт адаптер диалогов.
func New(client *openai.Client) *Adapter {
	return &Adapter{client: client}
}

// NewConversation создаёт диалог и возвращает его ID.
func (a *Adapter) NewConversation(ctx context.Context, contextText string, metadata map[string]string) (string, error) {
	params := conversations.ConversationNewParams{}
	if contextText != "" {
		params.Items = []responses.ResponseInputItemUnionParam{
			responses.ResponseInputItemParamOfMessage(
				responses.ResponseInputMessageContentListParam{
					{
						OfInputText: &responses.ResponseInputTextParam{Text: contextText},
					},
				},
				responses.EasyInputMessageRoleDeveloper,
			),
		}
	}
	if len(metadata) > 0 {
		params.Metadata = shared.Metadata(metadata)
	}

	conv, err := a.client.Conversations.New(ctx, params)
	if err != nil {
		return "", err
	}

	return conv.ID, nil
}
