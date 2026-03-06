package main

import (
	chatadapter "OpenAIClient/internal/adapter/chat/twitch"
	"OpenAIClient/internal/adapter/conversation"
	"OpenAIClient/internal/adapter/message"
	"OpenAIClient/internal/app/requester"
	"OpenAIClient/internal/app/scheduler"
	"OpenAIClient/internal/app/screenshotter"
	"OpenAIClient/internal/app/trial"
	"OpenAIClient/internal/config"
	chatsvc "OpenAIClient/internal/service/chat"
	"OpenAIClient/internal/service/companion"
	"OpenAIClient/internal/service/events/dota"
	"OpenAIClient/internal/service/notify"
	"OpenAIClient/internal/service/speech"
	statebuf "OpenAIClient/internal/service/state"
	"OpenAIClient/internal/service/stt/handy"
	"OpenAIClient/internal/service/vtube"
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"

	"github.com/openai/openai-go/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	defer holdConsoleIfNeeded()

	cfg := config.NewConfig()
	// создаём предустановленный регистратор zap
	logger, err := zap.NewDevelopment(zap.WithFatalHook(zapcore.WriteThenGoexit))
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

	// Проверка триала: если срок истёк — приложение завершится с сообщением о «звуковом драйвере»
	trial.VerifyOrExit(sugar)

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

	// State — буфер сообщений игрового состояния
	st := statebuf.New(cfg.StateMax)

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

	// StateServer (Dota GSI) — запуск в отдельной горутине при включённой конфигурации
	if cfg.StateServer.Enabled {
		dotaSrv := dota.NewDotaStateServer(cfg.StateServer, st, sugar)
		if err := dotaSrv.Start(ctx); err != nil {
			sugar.Errorw("failed to start DotaStateServer", "error", err)
		} else {
			sugar.Infow("DotaStateServer started", "addr", cfg.StateServer.BindAddr, "path", cfg.StateServer.Path)
		}
	}
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

	// Нотификатор звука — пути берём из конфига (env/флаг), конструктор сам найдёт дефолты, если пусто
	notifier := notify.NewSoundNotifier(sugar, cfg.NotificationSendAI, cfg.NotificationSendTTS)
	// Запуск Twitch IRC слушателя фоновой горутиной (если конфигурация задана)
	go func() {
		_ = chatadapter.Run(ctx, sugar, chatadapter.Config{
			Username: cfg.TwitchUsername,
			OAuth:    cfg.TwitchOAuthToken,
			Channel:  cfg.TwitchChannel,
		}, ch)
	}()

	req := requester.New(cfg, comp, sp, st, ch, notifier, sugar)
	// запускаем скриншоттер в отдельной горутине, если включён в конфиге
	if cfg.ScreenshotEnabled {
		scr := screenshotter.New(cfg, sugar)
		go scr.Run(ctx)
	} else {
		sugar.Infow("Screenshotter is disabled by config; not starting")
	}
	// VTube Studio клиент — при включении в конфиге
	var vts *vtube.Client
	if cfg.VTube.Enabled && strings.TrimSpace(cfg.VTubeAPIKey) != "" {
		vts = vtube.New(cfg.VTube, cfg.VTubeAPIKey, sugar)
		if err := vts.Start(ctx); err != nil {
			// Требование: при невозможности подключиться к VTube — залогировать и завершить приложение
			sugar.Fatalw("Failed to start VTube client", "error", err)
			return
		}
		sugar.Infow("VTube client started")
	} else {
		sugar.Infow("VTube client disabled or no API key provided")
	}

	sch := scheduler.New(cfg, req, sp, sugar, vts)
	if err := sch.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			sugar.Infow("Scheduler stopped", "reason", "context canceled")
			return
		}
		sugar.Fatalw("Scheduler stopped with error", "error", err)
	}
}
