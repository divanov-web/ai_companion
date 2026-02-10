package main

import (
	"OpenAIClient/internal/ai"
	"OpenAIClient/internal/config"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

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
	oclient := openai.NewClient()

	sugar.Infow(
		"Starting app",
		"DebugMode", cfg.DebugMode,
	)

	visionClient := ai.NewVisionClient(&oclient, cfg)

	// Читаем демо‑картинку (в проекте доступны images/1.png и images/2.png)
	imageData, err := os.ReadFile("images/1.png")
	if err != nil {
		log.Fatalf("failed to read image file: %v", err)
	}
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	// Важно: правильный MIME‑тип для PNG
	imageURL := fmt.Sprintf("data:image/png;base64,%s", base64Image)

	text := "Это скриншот игры мира кораблей. Внизу панель с моим вооружением и под цифрой три торпеды. Сколько секунд они ещё будут перезарежаться?"

	// 3) Визуальный клиент (текст + картинка)
	if resp, err := visionClient.SendRequest(ctx, text, imageURL); err != nil {
		fmt.Printf("VisionClient error: %v\n", err)
	} else {
		fmt.Printf("VisionClient response: %s\n", resp)
	}
}
