package main

import (
	"OpenAIClient/internal/adapter/conversation"
	"OpenAIClient/internal/adapter/message"
	"OpenAIClient/internal/app/requester"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service"
	"context"
	"fmt"

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
	companion := service.NewCompanion(convAdapter, msgAdapter)

	req := requester.New(cfg, companion, sugar)
	resp, err := req.RunOnce(ctx, "какой результат боя? на каком корабле я играл? предположи, почему проиграли? сейчас можно ответить в 4-6 предложений.")
	if err != nil {
		sugar.Fatalw("Request failed", "error", err)
	}
	fmt.Printf("Assistant response: %s\n", resp)
}
