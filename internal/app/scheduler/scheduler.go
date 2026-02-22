package scheduler

import (
	"OpenAIClient/internal/app/requester"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/image"
	"OpenAIClient/internal/service/speech"
	"OpenAIClient/internal/service/tts"
	"OpenAIClient/internal/service/tts/gemini"
	"OpenAIClient/internal/service/tts/google"
	"OpenAIClient/internal/service/tts/player"
	"OpenAIClient/internal/service/tts/yandex"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Overlap policies
const (
	overlapSkip    = "skip"
	overlapPreempt = "preempt"
)

type Scheduler struct {
	cfg     *config.Config
	req     *requester.Requester
	speech  *speech.Speech
	tts     tts.Synthesizer
	logger  *zap.SugaredLogger
	cleaner *image.Cleaner

	running    atomic.Bool
	mu         sync.Mutex
	cancelPrev context.CancelFunc
	gen        int64 // Счётчик текущего тика

	consecutiveErrors int // счётчик ошибок
}

func New(cfg *config.Config, req *requester.Requester, sp *speech.Speech, logger *zap.SugaredLogger) *Scheduler {
	// Инициализируем плеер: для Yandex — учитываем внешнюю громкость; для Google — отдаём на откуп VolumeGainDb
	var p *player.Default
	service := strings.ToLower(strings.TrimSpace(cfg.TTSService))
	switch service {
	case "yandex", "yc", "speechkit":
		v := max(0, min(100, cfg.YandexTTS.Volume))
		volDB := float64(v-100) / 5.0
		p = player.NewWithVolume(volDB)
	default: // google/gemini по умолчанию — громкость регулируется на стороне провайдера
		p = player.New()
		if service == "" {
			service = "google"
		}
	}

	// Конкретный клиент
	var synth tts.Synthesizer
	switch service {
	case "yandex":
		synth = yandex.New(p)
	case "gemini", "google-gemini":
		synth = gemini.New(p, logger)
	default: // google
		synth = google.New(p, logger)
	}

	s := &Scheduler{cfg: cfg, req: req, speech: sp, tts: synth, logger: logger, cleaner: image.NewCleaner(logger)}
	s.logger.Infow("TTS selected", "service", service)
	return s
}

// Run запускает бесконечный цикл до отмены контекста или достижения лимита ошибок.
// Первый запуск выполняется по истечении первого интервала (initial delay = interval).
func (s *Scheduler) Run(ctx context.Context) error {
	base := time.Duration(s.cfg.TimerIntervalSeconds) * time.Second
	if base <= 0 {
		base = 10 * time.Second
	}

	// Фоновая задача очистки изображений по TTL
	cleanInterval := base
	ttl := time.Duration(s.cfg.ImagesTTLSeconds) * time.Second
	stopClean := make(chan struct{})
	go func() {
		t := time.NewTicker(cleanInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				close(stopClean)
				return
			case <-t.C:
				s.cleaner.Clean(s.cfg.ImagesSourceDir, ttl, s.cfg.DebugMode)
			}
		}
	}()

	// Ждём первый интервал перед первой сработкой
	s.logger.Infow("Scheduler started", "interval", base.String(), "overlap", s.cfg.OverlapPolicy)

	// Основной цикл ожидания: базовый таймер И сигналы от Speech для раннего тика
	for {
		// Фиксированная задержка без джиттера
		t := time.NewTimer(base)
		earlyCh := (<-chan struct{})(nil)
		if s.speech != nil && s.cfg.EnableEarlyTick {
			earlyCh = s.speech.NotifyCh()
		}
		firedEarly := false
		select {
		case <-ctx.Done():
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			s.stopPrev()
			<-stopClean
			return context.Cause(ctx)
		case <-t.C:
			// обычный тик по таймеру
		case <-earlyCh:
			firedEarly = true
			if !t.Stop() {
				// слить, если уже сработал
				select {
				case <-t.C:
				default:
				}
			}
		}

		if err := s.runTick(ctx); err != nil {
			s.consecutiveErrors++
			if firedEarly {
				s.logger.Errorw("Early tick failed", "error", err, "consecutiveErrors", s.consecutiveErrors)
			} else {
				s.logger.Errorw("Tick failed", "error", err, "consecutiveErrors", s.consecutiveErrors)
			}
			if s.consecutiveErrors >= max(1, s.cfg.MaxConsecutiveErrors) {
				s.logger.Errorw("Stopping due to consecutive errors threshold", "threshold", s.cfg.MaxConsecutiveErrors)
				s.stopPrev()
				return err
			}
		} else {
			s.consecutiveErrors = 0
		}
	}
}

func (s *Scheduler) runTick(parent context.Context) error {
	// Политика overlap
	if s.running.Load() {
		switch s.cfg.OverlapPolicy {
		case overlapPreempt:
			s.logger.Infow("Preempting previous tick")
			s.stopPrev()
		default: // skip
			s.logger.Infow("Skipping tick due to overlap")
			return nil
		}
	}

	// Создаём контекст тика с тайм-аутом
	timeout := time.Duration(s.cfg.TickTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tickCtx, cancel := context.WithTimeoutCause(parent, timeout, errors.New("tick timeout"))

	// Сохраняем cancel как текущий исполняемый тик и увеличиваем поколение
	s.mu.Lock()
	s.gen++
	localGen := s.gen
	s.cancelPrev = cancel
	s.mu.Unlock()

	s.running.Store(true)
	defer func() {
		s.running.Store(false)
		cancel()
		s.mu.Lock()
		if s.gen == localGen {
			s.cancelPrev = nil
		}
		s.mu.Unlock()
	}()

	start := time.Now()
	s.logger.Infow("Tick start")

	// Запрос через requester: выбор текста теперь происходит в Requester
	resp, err := s.req.SendMessage(tickCtx)
	if err != nil {
		return err
	}

	// Проигрываем TTS, если есть ответ
	if resp != "" {
		s.logger.Infow(resp)
		// Выбор конфига под текущий сервис
		var ttsCfg any
		prompt := ""
		switch strings.ToLower(strings.TrimSpace(s.cfg.TTSService)) {
		case "yandex":
			ttsCfg = s.cfg.YandexTTS
		case "gemini", "google-gemini":
			ttsCfg = s.cfg.GeminiTTS
			prompt = s.cfg.GeminiTTS.Prompt
		default: // google
			ttsCfg = s.cfg.GoogleTTS
		}
		if ttsErr := s.tts.Synthesize(tickCtx, resp, prompt, ttsCfg); ttsErr != nil {
			// Ошибка TTS трактуем как ошибку тика?
			// По ТЗ: «TTS проигрывается при каждом тике, если был ответ» — ошибок TTS не указано отдельно,
			// логируем и считаем ошибкой тика, чтобы не зациклиться в немом режиме.
			return ttsErr
		}
	}

	s.logger.Infow("Tick done", "duration", time.Since(start).String())
	return nil
}

func (s *Scheduler) stopPrev() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelPrev != nil {
		s.cancelPrev()
		s.cancelPrev = nil
	}
}
