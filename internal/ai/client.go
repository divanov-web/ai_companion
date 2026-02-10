package ai

import "context"

// Client интерфейс для взаимодействия с AI. Все реализации должны быть взаимозаменяемыми.
type Client interface {
	SendRequest(ctx context.Context, text string, imageURL string) (string, error)
}
