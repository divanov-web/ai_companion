package main

import (
	"OpenAIClient/internal/adapter/conversation"
	"OpenAIClient/internal/adapter/message"
	"OpenAIClient/internal/app/requester"
	"OpenAIClient/internal/app/scheduler"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/companion"
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

	convAdapter := conversation.New(&oClient, sugar)
	msgAdapter := message.New(&oClient, sugar)
	comp := companion.NewCompanion(convAdapter, msgAdapter)

	req := requester.New(cfg, comp, sugar)
	sch := scheduler.New(cfg, req, sugar)
	if err := sch.Run(ctx); err != nil {
		sugar.Fatalw("Scheduler stopped with error", "error", err)
	}
}
