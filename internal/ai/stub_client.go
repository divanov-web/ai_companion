package ai

import "context"

// StubClient заглушка, которая не делает реальных запросов
type StubClient struct{}

func NewStubClient() *StubClient { return &StubClient{} }

func (c *StubClient) SendRequest(_ context.Context, _, _ string) (string, error) {
	return "запрос получен", nil
}
