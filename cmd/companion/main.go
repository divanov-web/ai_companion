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
	oClient := openai.NewClient()

	sugar.Infow(
		"Starting app",
		"DebugMode", cfg.DebugMode,
	)

	// Пример использования диалогового клиента (Responses‑основанный)
	dlg := ai.NewResponsesDialogueClient(&oClient, openai.ChatModelGPT4o)

	// 1. Создать диалог с параметрами (инструкциями)
	instructions := "Ты старый пират и во все ответы добавляешь характерные комментарии, лексику"
	convID, err := dlg.CreateConversation(ctx, instructions)
	if err != nil {
		sugar.Errorw("failed to create conversation", "error", err)
		return
	}

	// Подготовим data URL для изображений (используем имеющиеся images/1.png и images/2.png)
	mkDataURL := func(path string) (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("data:image/%s;base64,%s", "png", base64.StdEncoding.EncodeToString(b)), nil
	}

	img1, err := mkDataURL("images/1.png")
	if err != nil {
		log.Fatalf("failed to read first image: %v", err)
	}
	/*img2, err := mkDataURL("images/2.png")
	if err != nil {
		log.Fatalf("failed to read second image: %v", err)
	}*/

	// 2. Отправить сообщение с картинкой 1 и текстом
	msg1 := "Что изображено на картинке?"
	if resp, err := dlg.SendMessage(ctx, convID, msg1, []string{img1}); err != nil {
		fmt.Printf("DialogueClient error (msg1): %v\n", err)
	} else {
		fmt.Printf("Assistant response (msg1): %s\n", resp)
	}

	// 3. Отправить сообщение с картинкой 2 и текстом, ссылаясь на первую
	/*msg2 := "Что изображено на картинке? Как нам взять с собой животное из первой картинки"
	if resp, err := dlg.SendMessage(ctx, convID, msg2, []string{img2}); err != nil {
		fmt.Printf("DialogueClient error (msg2): %v\n", err)
	} else {
		fmt.Printf("Assistant response (msg2): %s\n", resp)
	}*/
}
