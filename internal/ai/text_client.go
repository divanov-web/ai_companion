package ai

import (
	"OpenAIClient/internal/config"
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

// TextClient отправляет только текст в OpenAI
type TextClient struct {
	client *openai.Client
	model  string
}

func NewTextClient(client *openai.Client, cfg *config.Config) *TextClient {
	return &TextClient{
		client: client,
		model:  string(openai.ChatModelGPT4o),
	}
}

func (c *TextClient) SendRequest(ctx context.Context, text string, _ string) (string, error) {
	resp, err := c.client.Responses.New(ctx, responses.ResponseNewParams{
		Model: openai.ChatModelGPT4o,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						{
							OfInputText: &responses.ResponseInputTextParam{
								Text: text,
							},
						},
					},
					responses.EasyInputMessageRoleUser,
				),
			},
		},
	})
	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}
