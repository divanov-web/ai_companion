package tts

import "context"

// Synthesizer абстракция TTS. Метод воспроизводит речь и не возвращает контент.
// cfg — провайдер-специфичная конфигурация (например, config.YandexTTSConfig).
type Synthesizer interface {
	Synthesize(ctx context.Context, text string, cfg any) error
}
