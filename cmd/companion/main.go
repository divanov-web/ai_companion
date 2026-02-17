package main

import (
	"OpenAIClient/internal/adapter/conversation"
	"OpenAIClient/internal/adapter/message"
	"OpenAIClient/internal/app/requester"
	"OpenAIClient/internal/app/scheduler"
	"OpenAIClient/internal/app/screenshotter"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/companion"
	"OpenAIClient/internal/service/speech"
	"OpenAIClient/internal/service/stt/handy"
	"context"
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

	// STT Handy listener — фоновый запуск
	stt := handy.New(handy.Config{HandyWindow: cfg.STTHandyWindow, HotkeyDelay: cfg.STTHotkeyDelay})
	go func() {
		if err := stt.Run(ctx); err != nil {
			sugar.Errorw("STT service stopped", "error", err)
		}
	}()
	// Подписка на события STT
	go func() {
		for ev := range stt.Events() {
			ts := ev.At.Format("15:04:05.000")
			switch ev.Type {
			case handy.EventClipboardChanged:
				if cfg.DebugMode {
					sugar.Infow("[CLIPBOARD]", "ts", ts, "len", len(ev.Text))
				}
			case handy.EventCtrlEnter:
				if cfg.DebugMode {
					sugar.Infow("[CTRL+ENTER]", "ts", ts)
				}
			case handy.EventHandyFinalText:
				sp.Add(ev.Text)
				sugar.Infow("Текст пойман", "text", ev.Text)
			}
		}
	}()

	req := requester.New(cfg, comp, sp, sugar)
	// запускаем скриншоттер в отдельной горутине
	scr := screenshotter.New(cfg, sugar)
	go scr.Run(ctx)
	sch := scheduler.New(cfg, req, sp, sugar)
	if err := sch.Run(ctx); err != nil {
		sugar.Fatalw("Scheduler stopped with error", "error", err)
	}
}
