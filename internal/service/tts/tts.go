package tts

import "context"

// Synthesizer абстракция TTS. Метод воспроизводит речь и не возвращает контент.
// cfg — провайдер-специфичная конфигурация (например, config.YandexTTSConfig).
// prompt — опциональный системный промпт для провайдера (используется только Gemini; для остальных пустой).
type Synthesizer interface {
	Synthesize(ctx context.Context, text string, prompt string, cfg any) error
}
