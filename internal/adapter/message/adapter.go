package message

import (
	"OpenAIClient/internal/service/image"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"go.uber.org/zap"
)

type Adapter struct {
	client *openai.Client
	logger *zap.SugaredLogger
}

// New возвращает адаптер отправки сообщений.
func New(client *openai.Client, logger *zap.SugaredLogger) *Adapter {
	return &Adapter{client: client, logger: logger}
}

// SendTextWithImage отправляет:
// - systemPrompt (опционально) как system;
// - assistantPrompt (опционально) отдельным сообщением ассистента;
// - userPrompt Текущий текст пользователя с картинками.
// История должна быть заранее слита в `text` на уровне вызова.
func (a *Adapter) SendTextWithImage(ctx context.Context, systemPrompt string, assistantPrompt string, userPrompt string, images []image.ProcessedImage) (string, error) {
	// Контент пользовательского сообщения: текст, затем изображения
	content := make(responses.ResponseInputMessageContentListParam, 0, len(images)+1)
	content = append(content, responses.ResponseInputContentParamOfInputText(userPrompt))
	for _, img := range images {
		dataURL, err := makeImageDataURL(img)
		if err != nil {
			return "", err
		}
		imageParam := responses.ResponseInputContentParamOfInputImage(responses.ResponseInputImageDetailAuto)
		imageParam.OfInputImage.ImageURL = openai.String(dataURL)
		content = append(content, imageParam)
	}

	// Собираем входные элементы: system (если есть), assistant (если есть), user
	inputItems := make(responses.ResponseInputParam, 0, 3)
	if st := strings.TrimSpace(systemPrompt); st != "" {
		inputItems = append(inputItems,
			responses.ResponseInputItemParamOfMessage(
				responses.ResponseInputMessageContentListParam{
					{OfInputText: &responses.ResponseInputTextParam{Text: st}},
				},
				responses.EasyInputMessageRoleSystem,
			),
		)
	}
	// assistantPrompt добавляем как output_message с output_text
	if ap := strings.TrimSpace(assistantPrompt); ap != "" {
		var out responses.ResponseOutputTextParam
		out.Text = ap
		out.Annotations = nil
		assistantContent := []responses.ResponseOutputMessageContentUnionParam{
			{OfOutputText: &out},
		}
		inputItems = append(inputItems,
			responses.ResponseInputItemParamOfOutputMessage(
				assistantContent,
				"",
				responses.ResponseOutputMessageStatusCompleted,
			),
		)
	}
	inputItems = append(inputItems,
		responses.ResponseInputItemParamOfMessage(
			content,
			responses.EasyInputMessageRoleUser,
		),
	)

	params := responses.ResponseNewParams{
		Model: openai.ChatModelGPT4o,
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: inputItems},
	}

	start := time.Now()
	a.logger.Infow("Запрос в OpenAI...")
	resp, err := a.client.Responses.New(ctx, params)
	dur := time.Since(start)
	if err != nil {
		a.logger.Errorw("Ошибка ответа OpenAI", "duration", dur.String(), "error", err)
	} else {
		a.logger.Infow("Ответ OpenAI получен", "duration", dur.String())
	}
	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}

func makeImageDataURL(img image.ProcessedImage) (string, error) {
	contentType := img.MimeType
	if contentType == "" {
		contentType = "image/jpeg"
	}
	data, err := os.ReadFile(img.Path)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("image file is empty: %s", img.Path)
	}
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data)), nil
}
