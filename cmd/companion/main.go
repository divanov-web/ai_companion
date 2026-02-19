package main

import (
	chatadapter "OpenAIClient/internal/adapter/chat/twitch"
	"OpenAIClient/internal/adapter/conversation"
	"OpenAIClient/internal/adapter/message"
	"OpenAIClient/internal/app/requester"
	"OpenAIClient/internal/app/scheduler"
	"OpenAIClient/internal/app/screenshotter"
	"OpenAIClient/internal/config"
	chatsvc "OpenAIClient/internal/service/chat"
	"OpenAIClient/internal/service/companion"
	"OpenAIClient/internal/service/notify"
	"OpenAIClient/internal/service/speech"
	"OpenAIClient/internal/service/stt/handy"
	"context"
	"errors"
	"os"
	"os/signal"

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// клиента OpenAI (использует переменные окружения OPENAI_API_KEY)
	oClient := openai.NewClient()

	sugar.Infow(
		"Starting app",
		"DebugMode", cfg.DebugMode,
	)

	convAdapter := conversation.New(&oClient, sugar)
	msgAdapter := message.New(&oClient, sugar)
	comp := companion.NewCompanion(convAdapter, msgAdapter)

	// Speech — буфер сообщений из STT
	sp := speech.New(cfg.SpeechMax)

	// Chat — буфер сообщений из Twitch-чата
	ch := chatsvc.New(cfg.ChatMax)

	// STT Handy listener — фоновый запуск
	stt := handy.New(handy.Config{HandyWindow: cfg.STTHandyWindow, HotkeyDelay: cfg.STTHotkeyDelay})
	go func() {
		if err := stt.Run(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				sugar.Infow("STT service stopped", "reason", "context canceled")
			} else {
				sugar.Errorw("STT service stopped", "error", err)
			}
		}
	}()
	// Подписка на события STT
	go func() {
		for ev := range stt.Events() {
			// Обрабатываем только финальный текст от Handy
			if ev.Type != handy.EventHandyFinalText {
				continue
			}
			sp.Add(ev.Text)
		}
	}()

	// Нотификатор звука — путь берём из конфига (env/флаг), конструктор сам найдёт дефолт, если пусто
	notifier := notify.NewSoundNotifier(sugar, cfg.NotificationSoundPath)
	// Запуск Twitch IRC слушателя фоновой горутиной (если конфигурация задана)
	go func() {
		_ = chatadapter.Run(ctx, sugar, chatadapter.Config{
			Username: cfg.TwitchUsername,
			OAuth:    cfg.TwitchOAuthToken,
			Channel:  cfg.TwitchChannel,
		}, ch)
	}()

	req := requester.New(cfg, comp, sp, ch, notifier, sugar)
	// запускаем скриншоттер в отдельной горутине
	scr := screenshotter.New(cfg, sugar)
	go scr.Run(ctx)
	sch := scheduler.New(cfg, req, sp, sugar)
	if err := sch.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			sugar.Infow("Scheduler stopped", "reason", "context canceled")
			return
		}
		sugar.Fatalw("Scheduler stopped with error", "error", err)
	}
}
