package ai

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

// ResponsesDialogueClient реализует интерфейс DialogueClient поверх Responses API,
// поддерживая разговоры локально (conversationID -> история сообщений + инструкции).
// Это позволяет позже заменить реализацию на Assistants/Threads без изменения интерфейсов.
type ResponsesDialogueClient struct {
	client *openai.Client
	model  openai.ChatModel

	mu    sync.Mutex
	talks map[string]*conversationState
}

type conversationState struct {
	instructions string
	// компактная история последних сообщений (user/assistant) — для MVP достаточно 2-3 итераций
	history []responses.ResponseInputMessageContentListParam
}

func NewResponsesDialogueClient(client *openai.Client, model openai.ChatModel) *ResponsesDialogueClient {
	return &ResponsesDialogueClient{
		client: client,
		model:  model,
		talks:  make(map[string]*conversationState),
	}
}

func (c *ResponsesDialogueClient) CreateConversation(ctx context.Context, instructions string) (string, error) {
	if c.client == nil {
		return "", errors.New("nil openai client")
	}
	id := uuid.NewString()
	c.mu.Lock()
	c.talks[id] = &conversationState{instructions: instructions}
	c.mu.Unlock()
	return id, nil
}

func (c *ResponsesDialogueClient) SendMessage(ctx context.Context, conversationID string, text string, imageURLs []string) (string, error) {
	if c.client == nil {
		return "", errors.New("nil openai client")
	}
	c.mu.Lock()
	st, ok := c.talks[conversationID]
	c.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("unknown conversation: %s", conversationID)
	}

	// Системные инструкции (как первая реплика role=system).
	sys := responses.ResponseInputMessageContentListParam{
		{
			OfInputText: &responses.ResponseInputTextParam{Text: st.instructions},
		},
	}

	// Пользовательский ввод: текст + изображения (как data URL/URL).
	user := responses.ResponseInputMessageContentListParam{
		{
			OfInputText: &responses.ResponseInputTextParam{Text: text},
		},
	}
	for _, u := range imageURLs {
		url := u // ожидается data URL или http(s) URL
		img := responses.ResponseInputMessageContentListParam{
			{
				OfInputImage: &responses.ResponseInputImageParam{
					Detail:   responses.ResponseInputImageDetailAuto,
					ImageURL: openai.String(url),
				},
			},
		}
		user = append(user, img...)
	}

	// Собираем вход: system + (опционально) короткая история + текущее user сообщение.
	inputItems := responses.ResponseInputParam{
		responses.ResponseInputItemParamOfMessage(sys, responses.EasyInputMessageRoleSystem),
	}
	// добавляем последнюю известную историю (усекаем до 2 последних пар)
	if n := len(st.history); n > 0 {
		// оставим максимум 2 записи истории, чтобы не раздувать контекст
		start := 0
		if n > 2 {
			start = n - 2
		}
		for _, h := range st.history[start:] {
			inputItems = append(inputItems, responses.ResponseInputItemParamOfMessage(h, responses.EasyInputMessageRoleUser))
		}
	}
	inputItems = append(inputItems, responses.ResponseInputItemParamOfMessage(user, responses.EasyInputMessageRoleUser))

	resp, err := c.client.Responses.New(ctx, responses.ResponseNewParams{
		Model: c.model,
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: inputItems},
	})
	if err != nil {
		return "", err
	}

	out := resp.OutputText()

	// Обновим историю (храним только пользовательскую часть; ассистента — как текст в отдельной записи при желании).
	c.mu.Lock()
	st.history = append(st.history, user)
	// Для простоты добавим и краткий ответ ассистента в виде текстового сообщения.
	st.history = append(st.history, responses.ResponseInputMessageContentListParam{
		{OfInputText: &responses.ResponseInputTextParam{Text: out}},
	})
	c.mu.Unlock()

	return out, nil
}
