package main

import (
	"OpenAIClient/internal/adapter/conversation"
	"OpenAIClient/internal/adapter/message"
	"OpenAIClient/internal/app/requester"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/companion"
	"OpenAIClient/internal/service/tts/player"
	"OpenAIClient/internal/service/tts/yandex"
	"context"

	"github.com/openai/openai-go/v3"
	"go.uber.org/zap"
)

func main() {

	cfg := config.NewConfig()
	// создаём предустановленный регистратор zap
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	// делаем регистратор SugaredLogger
	sugar := logger.Sugar()
	//сброс буфера логгера
	defer func() {
		if err := logger.Sync(); err != nil {
			sugar.Errorw("Failed to sync logger", "error", err)
		}
	}()

	ctx := context.Background()
	// клиента OpenAI (использует переменные окружения OPENAI_API_KEY)
	oClient := openai.NewClient()

	sugar.Infow(
		"Starting app",
		"DebugMode", cfg.DebugMode,
	)

	convAdapter := conversation.New(&oClient)
	msgAdapter := message.New(&oClient)
	comp := companion.NewCompanion(convAdapter, msgAdapter)

	req := requester.New(cfg, comp, sugar)
	resp, err := req.SendMessage(ctx, "какой результат боя? на каком корабле я играл? предположи, почему проиграли? сейчас можно ответить в 1-3 предложений.")
	if err != nil {
		sugar.Fatalw("Request failed", "error", err)
	}

	// Воспроизводим ответ ассистента через Yandex TTS
	if resp != "" {
		p := player.New()
		yc := yandex.New(p)
		sugar.Info(resp)
		if err := yc.Synthesize(ctx, resp, cfg.YandexTTS); err != nil {
			sugar.Fatalw("TTS playback failed", "error", err)
		}
	}
}
