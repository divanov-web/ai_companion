package events

import "context"

// EventServer описывает сервис приёма игровых событий по HTTP.
// Предполагается несколько реализаций (Dota, CS2 и т.п.).
type EventServer interface {
	// Start запускает сервер в отдельной горутине и немедленно возвращается.
	// Должен реагировать на отмену контекста и завершать работу.
	Start(ctx context.Context) error

	// Stop инициирует graceful shutdown с использованием контекста.
	Stop(ctx context.Context) error

	// Addr возвращает адрес, на котором слушает сервер.
	Addr() string
}
