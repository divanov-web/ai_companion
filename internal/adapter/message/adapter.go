package message

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

type Adapter struct {
	client *openai.Client
}

func New(client *openai.Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) SendTextWithImage(ctx context.Context, conversationID string, text string, imageDataURL string) (string, error) {
	params := responses.ResponseNewParams{
		Model: openai.ChatModelGPT4o,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						{
							OfInputText: &responses.ResponseInputTextParam{Text: text},
						},
						{
							OfInputImage: &responses.ResponseInputImageParam{
								Detail:   responses.ResponseInputImageDetailAuto,
								ImageURL: openai.String(imageDataURL),
							},
						},
					},
					responses.EasyInputMessageRoleUser,
				),
			},
		},
	}
	if conversationID != "" {
		params.Conversation = responses.ResponseNewParamsConversationUnion{OfString: openai.String(conversationID)}
	}

	resp, err := a.client.Responses.New(ctx, params)
	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}
