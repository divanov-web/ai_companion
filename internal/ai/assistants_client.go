//go:build ignore

package ai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/assistants"
	"github.com/openai/openai-go/v3/threads"
)

// AssistantsDialogueClient реализует серверный контекст диалога через OpenAI Assistants (Threads).
type AssistantsDialogueClient struct {
	client      *openai.Client
	model       openai.ChatModel
	assistantID string
}

// NewAssistantsDialogueClient создаёт клиента. Если assistantID пустой, клиент создаст ассистента при первом диалоге.
func NewAssistantsDialogueClient(client *openai.Client, model openai.ChatModel, assistantID string) *AssistantsDialogueClient {
	return &AssistantsDialogueClient{client: client, model: model, assistantID: assistantID}
}

// CreateConversation создаёт ассистента (если нужно) и новый thread. Инструкции сохраняются в ассистенте.
func (c *AssistantsDialogueClient) CreateConversation(ctx context.Context, instructions string) (string, error) {
	if c.client == nil {
		return "", errors.New("nil openai client")
	}

	// Создаём ассистента однажды, если ID не задан.
	if c.assistantID == "" {
		asst, err := c.client.Assistants.New(ctx, assistants.AssistantNewParams{
			Model:        c.model,
			Instructions: openai.String(instructions),
		})
		if err != nil {
			return "", fmt.Errorf("create assistant: %w", err)
		}
		c.assistantID = asst.ID
	}

	th, err := c.client.Threads.New(ctx, threads.ThreadNewParams{})
	if err != nil {
		return "", fmt.Errorf("create thread: %w", err)
	}
	return th.ID, nil
}

// SendMessage добавляет сообщение (текст + опциональные картинки) в thread и запускает Run.
func (c *AssistantsDialogueClient) SendMessage(ctx context.Context, conversationID string, text string, imageURLs []string) (string, error) {
	if c.client == nil {
		return "", errors.New("nil openai client")
	}
	if conversationID == "" {
		return "", errors.New("empty conversation id")
	}
	if c.assistantID == "" {
		return "", errors.New("assistant is not initialized; call CreateConversation first")
	}

	// Готовим контент сообщения: текст + изображения как image_url.
	content := make([]threads.MessageNewParamsContentItemUnion, 0, 1+len(imageURLs))
	content = append(content,
		threads.MessageNewParamsContentItemUnion{
			OfInputText: &threads.MessageNewParamsContentItemTextParam{Text: text},
		},
	)
	for _, url := range imageURLs {
		u := url // копия для захвата адреса
		content = append(content,
			threads.MessageNewParamsContentItemUnion{
				OfInputImageURL: &threads.MessageNewParamsContentItemImageURLParam{
					ImageURL: openai.String(u),
				},
			},
		)
	}

	// Добавляем сообщение в тред (роль user).
	if _, err := c.client.Threads.Messages.New(ctx, conversationID, threads.MessageNewParams{
		Role:    threads.MessageRoleUser,
		Content: content,
	}); err != nil {
		return "", fmt.Errorf("add message: %w", err)
	}

	// Запускаем Run.
	run, err := c.client.Threads.Runs.New(ctx, conversationID, threads.RunNewParams{
		AssistantID: c.assistantID,
	})
	if err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}

	// Простейший опрос статуса до завершения.
	// Для реального кода предпочтительнее streaming/callbacks.
	deadline := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var lastStatus threads.RunStatus
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline:
			return "", errors.New("run timeout")
		case <-ticker.C:
			r, err := c.client.Threads.Runs.Get(ctx, conversationID, run.ID)
			if err != nil {
				return "", fmt.Errorf("get run: %w", err)
			}
			lastStatus = r.Status
			switch lastStatus {
			case threads.RunStatusCompleted:
				// Получаем последние сообщения ассистента.
				msgs, err := c.client.Threads.Messages.List(ctx, conversationID, nil)
				if err != nil {
					return "", fmt.Errorf("list messages: %w", err)
				}
				// Берём самое новое сообщение роли assistant.
				for _, m := range msgs.Data {
					if m.Role == threads.MessageRoleAssistant {
						return m.ContentText(), nil
					}
				}
				return "", errors.New("no assistant message found")
			case threads.RunStatusFailed, threads.RunStatusCancelled, threads.RunStatusExpired:
				return "", fmt.Errorf("run ended with status %s", lastStatus)
			default:
				// ждём
			}
		}
	}
}
