package main

import (
	"OpenAIClient/internal/adapter/conversation"
	"OpenAIClient/internal/adapter/message"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service"
	"OpenAIClient/internal/service/image"
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
	// создаём реального клиента OpenAI (использует переменные окружения, напр. OPENAI_API_KEY)
	oClient := openai.NewClient()

	sugar.Infow(
		"Starting app",
		"DebugMode", cfg.DebugMode,
	)

	convAdapter := conversation.New(&oClient)
	msgAdapter := message.New(&oClient)
	companion := service.NewCompanion(convAdapter, msgAdapter)

	startContext := cfg.StartPrompt
	metadata := map[string]string{
		"ship":   "Громовержец",
		"battle": "оценка экипажа",
	}
	convID, err := companion.StartConversation(ctx, startContext, metadata)
	if err != nil {
		sugar.Fatalw("Failed to start conversation", "error", err)
	}

	processor := image.NewProcessor("images\\processed")
	img1, err := processor.Process("images\\sharex\\2026-02-12_12-54-31-656.jpg")
	if err != nil {
		sugar.Fatalw("Failed to process image 1", "error", err)
	}
	img2, err := processor.Process("images\\sharex\\2026-02-12_12-54-21-652.jpg")
	if err != nil {
		sugar.Fatalw("Failed to process image 2", "error", err)
	}

	resp, err := companion.SendMessageWithImage(ctx, convID, "какой результат боя?", []image.ProcessedImage{img1, img2})
	if err != nil {
		sugar.Fatalw("Message failed", "error", err)
	}
	fmt.Printf("Assistant response: %s\n", resp)
}
