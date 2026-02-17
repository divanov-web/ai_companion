package handy

import (
	"context"
	"time"
)

// EventType описывает типы событий, публикуемых сервисом.
type EventType int

const (
	EventClipboardChanged EventType = iota + 1
	EventCtrlEnter
	EventHandyFinalText
)

// Event универсальное событие сервиса Handy STT.
type Event struct {
	Type EventType
	Text string
	At   time.Time
}

// Service минимальный интерфейс сервиса Handy STT.
type Service interface {
	Run(ctx context.Context) error
	Events() <-chan Event
}

// Config временные параметры (держим локально, без общего конфига по ТЗ).
type Config struct {
	// Окно времени для связывания буфера и Ctrl+Enter
	HandyWindow time.Duration
	// Задержка реакции на Ctrl+Enter перед определением финального текста
	HotkeyDelay time.Duration
}

// New создает сервис с координатором и источниками событий (Windows).
func New(cfg Config) Service {
	c := &coordinator{
		cfg:           cfg,
		out:           make(chan Event, 64),
		clipIn:        make(chan Event, 64),
		hotkeyIn:      make(chan Event, 64),
		lastText:      "",
		lastTextAt:    time.Time{},
		shutdownReady: make(chan struct{}),
	}
	return c
}
