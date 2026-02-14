package scheduler

import (
	"OpenAIClient/internal/app/requester"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/tts/player"
	"OpenAIClient/internal/service/tts/yandex"
	"context"
	"errors"
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
	cfg    *config.Config
	req    *requester.Requester
	tts    *yandex.Client
	logger *zap.SugaredLogger

	running    atomic.Bool
	mu         sync.Mutex
	cancelPrev context.CancelFunc
	gen        int64 // Счётчик текущего тика

	consecutiveErrors int // счётчик ошибок
}

func New(cfg *config.Config, req *requester.Requester, logger *zap.SugaredLogger) *Scheduler {
	p := player.New()
	yc := yandex.New(p)
	return &Scheduler{cfg: cfg, req: req, tts: yc, logger: logger}
}

// Run запускает бесконечный цикл до отмены контекста или достижения лимита ошибок.
// Первый запуск выполняется по истечении первого интервала (initial delay = interval).
func (s *Scheduler) Run(ctx context.Context) error {
	base := time.Duration(s.cfg.TimerIntervalSeconds) * time.Second
	if base <= 0 {
		base = 10 * time.Second
	}

	// Ждём первый интервал перед первой сработкой
	s.logger.Infow("Scheduler started", "interval", base.String(), "overlap", s.cfg.OverlapPolicy)

	// Основной цикл без использования time.Ticker, чтобы учитывать джиттер на каждый тик
	for {
		// Фиксированная задержка без джиттера
		delay := base

		select {
		case <-ctx.Done():
			s.stopPrev()
			return context.Cause(ctx)
		case <-time.After(delay):
			// время тика
		}

		if err := s.runTick(ctx); err != nil {
			s.consecutiveErrors++
			s.logger.Errorw("Tick failed", "error", err, "consecutiveErrors", s.consecutiveErrors)
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

	// Запрос через requester
	resp, err := s.req.SendMessage(tickCtx, s.cfg.FixedMessage)
	if err != nil {
		return err
	}

	// Проигрываем TTS, если есть ответ
	if resp != "" {
		s.logger.Infow(resp)
		if ttsErr := s.tts.Synthesize(tickCtx, resp, s.cfg.YandexTTS); ttsErr != nil {
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
