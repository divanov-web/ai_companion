package tts

import (
	"context"
	"io"
)

// Synthesizer абстракция TTS. Метод выполняет синтез и возвращает поток аудио и его формат,
// а воспроизведение выполняется вызывающей стороной.
// cfg — провайдер-специфичная конфигурация (например, config.YandexTTSConfig).
// prompt — опциональный системный промпт для провайдера (используется только Gemini; для остальных пустой).
type Synthesizer interface {
	Synthesize(ctx context.Context, text string, prompt string, cfg any) (format string, rc io.ReadCloser, err error)
}
