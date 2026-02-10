package ai

import "context"

// DialogueClient описывает клиент с «серверным» контекстом диалога на стороне AI‑провайдера.
// Предназначен для многократных сообщений внутри одного разговора (conversation/thread).
type DialogueClient interface {
	// CreateConversation создаёт новый диалог с начальными инструкциями ассистента ("кто ты такой").
	// Возвращает идентификатор диалога (thread/conversation id), который нужно использовать далее.
	CreateConversation(ctx context.Context, instructions string) (string, error)

	// SendMessage отправляет сообщение пользователя в диалог с опциональными изображениями (как data URL или URL).
	// Возвращает текст ответа ассистента (агрегированный, если ответ мультимодальный).
	SendMessage(ctx context.Context, conversationID string, text string, imageURLs []string) (string, error)
}
