package ai

import (
	"OpenAIClient/internal/config"
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

// VisionClient отправляет текст и картинку в OpenAI
type VisionClient struct {
	client *openai.Client
	model  string
}

func NewVisionClient(client *openai.Client, cfg *config.Config) *VisionClient {
	return &VisionClient{
		client: client,
		model:  string(openai.ChatModelGPT4o),
	}
}

func (c *VisionClient) SendRequest(ctx context.Context, text string, imageURL string) (string, error) {
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
						{
							OfInputImage: &responses.ResponseInputImageParam{
								Detail:   responses.ResponseInputImageDetailAuto,
								ImageURL: openai.String(imageURL),
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
