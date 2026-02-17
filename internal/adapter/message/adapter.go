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

// New создаёт адаптер сообщений.
func New(client *openai.Client, logger *zap.SugaredLogger) *Adapter {
	return &Adapter{client: client, logger: logger}
}

// SendTextWithImage отправляет системное сообщение (если задано), затем опциональный
// assistantPrompt отдельным сообщением ассистента, историю и текущий текст пользователя с картинками (stateless).
func (a *Adapter) SendTextWithImage(ctx context.Context, systemText string, assistantPrompt string, text string, responseHistory []string, images []image.ProcessedImage) (string, error) {
	// Контент одного пользовательского сообщения: сначала история (если есть), затем текущий текст, затем изображения
	content := make(responses.ResponseInputMessageContentListParam, 0, len(images)+2)
	if len(responseHistory) > 0 {
		historyText := strings.Join(responseHistory, "\n")
		content = append(content, responses.ResponseInputContentParamOfInputText(historyText))
	}
	content = append(content, responses.ResponseInputContentParamOfInputText(text))
	for _, img := range images {
		dataURL, err := makeImageDataURL(img)
		if err != nil {
			return "", err
		}
		imageParam := responses.ResponseInputContentParamOfInputImage(responses.ResponseInputImageDetailAuto)
		imageParam.OfInputImage.ImageURL = openai.String(dataURL)
		content = append(content, imageParam)
	}

	// Формируем список input items: опционально системное сообщение + пользовательское сообщение
	inputItems := make(responses.ResponseInputParam, 0, 3)
	if st := strings.TrimSpace(systemText); st != "" {
		inputItems = append(inputItems,
			responses.ResponseInputItemParamOfMessage(
				responses.ResponseInputMessageContentListParam{
					{OfInputText: &responses.ResponseInputTextParam{Text: st}},
				},
				responses.EasyInputMessageRoleSystem,
			),
		)
	}
	// Добавляем assistantPrompt отдельным сообщением ассистента, если он задан.
	// В Responses API для сообщений ассистента используется output_message
	// с контентом типа output_text.
	if ap := strings.TrimSpace(assistantPrompt); ap != "" {
		var out responses.ResponseOutputTextParam
		out.Text = ap
		// Аннотации могут быть пустыми, поле помечено omitzero,required — пустой массив валиден.
		out.Annotations = nil
		assistantContent := []responses.ResponseOutputMessageContentUnionParam{
			{OfOutputText: &out},
		}
		inputItems = append(inputItems,
			responses.ResponseInputItemParamOfOutputMessage(
				assistantContent,
				"", // id не обязателен для входного output_message
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
