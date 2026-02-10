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
	_ = openai.NewClient() // для реальных клиентов, если понадобятся

	sugar.Infow(
		"Starting app",
		"DebugMode", cfg.DebugMode,
	)

	// Инициализируем вариант заглушку согласно требованию
	aiClient := ai.NewStubClient()

	imageData, err := os.ReadFile("images/1.jpg")
	if err != nil {
		log.Fatalf("failed to read image file: %v", err)
	}
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	imageURL := fmt.Sprintf("data:image/jpeg;base64,%s", base64Image)

	text := "Это скриншот игры мира кораблей. Внизу панель с моим вооружением и под цифрой три торпеды. Сколько секунд они ещё будут перезарежаться?"

	resp, err := aiClient.SendRequest(ctx, text, imageURL)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp)
}
