package main

import (
	"OpenAIClient/internal/adapter/conversation"
	"OpenAIClient/internal/adapter/message"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	_ = cfg

	convAdapter := conversation.New(&oClient)
	msgAdapter := message.New(&oClient)
	companion := service.NewCompanion(convAdapter, msgAdapter)

	mkImageDataURL := func(path string) (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}

		switch strings.ToLower(filepath.Ext(path)) {
		case ".png":
			return fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(b)), nil
		case ".jpg", ".jpeg":
			return fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(b)), nil
		default:
			return "", fmt.Errorf("unsupported image extension: %s", filepath.Ext(path))
		}
	}

	tryLoad := func(paths ...string) (string, error) {
		var lastErr error
		for _, p := range paths {
			dataURL, err := mkImageDataURL(p)
			if err == nil {
				return dataURL, nil
			}
			lastErr = err
		}
		return "", errors.Join(errors.New("failed to load image"), lastErr)
	}

	convID, err := companion.StartConversation(ctx)
	if err != nil {
		sugar.Fatalw("Failed to start conversation", "error", err)
	}

	img1, err := tryLoad("images\\1.jpg", "images\\1.jpeg", "images\\1.png")
	if err != nil {
		sugar.Fatalw("Failed to load image 1", "error", err)
	}
	img2, err := tryLoad("images\\2.jpg", "images\\2.jpeg", "images\\2.png")
	if err != nil {
		sugar.Fatalw("Failed to load image 2", "error", err)
	}

	resp1, err := companion.SendMessageWithImage(ctx, convID, "Подходит ли этот боец в плавание", img1)
	if err != nil {
		sugar.Fatalw("Message 1 failed", "error", err)
	}
	fmt.Printf("Assistant response (1): %s\n", resp1)

	resp2, err := companion.SendMessageWithImage(ctx, convID, "А поплывём на этом", img2)
	if err != nil {
		sugar.Fatalw("Message 2 failed", "error", err)
	}
	fmt.Printf("Assistant response (2): %s\n", resp2)
}
