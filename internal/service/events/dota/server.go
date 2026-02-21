package dota

import (
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/events"
	st "OpenAIClient/internal/service/state"
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Ensure interface compliance
var _ events.StateServer = (*DotaStateServer)(nil)

type DotaStateServer struct {
	cfg     config.StateServerConfig
	srv     *http.Server
	logger  *zap.SugaredLogger
	running atomic.Bool
	state   *st.State
}

func NewDotaStateServer(cfg config.StateServerConfig, stbuf *st.State, logger *zap.SugaredLogger) *DotaStateServer {
	if cfg.BindAddr == "" {
		cfg.BindAddr = "127.0.0.1:3000"
	}
	if cfg.Path == "" {
		cfg.Path = "/"
	}
	s := &DotaStateServer{cfg: cfg, logger: logger, state: stbuf}

	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Path, s.handleEvent)

	s.srv = &http.Server{
		Addr:              cfg.BindAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return s
}

func (s *DotaStateServer) Start(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return nil
	}
	go func() {
		s.logger.Infow("DotaStateServer listening", "addr", s.srv.Addr, "path", s.cfg.Path)
		if err := s.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			s.logger.Errorw("DotaStateServer stopped with error", "error", err)
		} else {
			s.logger.Infow("DotaStateServer stopped")
		}
	}()

	// Watch for context cancellation to stop the server
	go func() {
		<-ctx.Done()
		_ = s.Stop(context.WithoutCancel(ctx))
	}()
	return nil
}

func (s *DotaStateServer) Stop(ctx context.Context) error {
	if !s.running.CompareAndSwap(true, false) {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeoutCause(ctx, 5*time.Second, errors.New("dota-event-server shutdown timeout"))
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		s.logger.Warnw("graceful shutdown error", "error", err)
		return s.srv.Close()
	}
	return nil
}

func (s *DotaStateServer) Addr() string { return s.cfg.BindAddr }

func (s *DotaStateServer) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed; use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	// Преобразуем сырое GSI-сообщение в компактную структуру "Eyes"
	if s.state != nil {
		if eyes, err := TransformToEyes(body); err == nil && len(eyes) > 0 {
			// сохраняем уже обработанный компактный JSON
			s.state.Add(string(eyes))
		} else if err == nil {
			// пустой результат — ничего не добавляем
		} else {
			// в случае ошибки трансформации не падаем — пишем сырой json как fallback
			s.state.Add(string(body))
		}
	}

	// По требованию: выводить в консоль весь JSON вместо отдельного поля map.
	// Дополнительное действия не требуются, так как выше уже напечатано поле "raw" с полным телом.
	// Оставляем только ответ 204.
	w.WriteHeader(http.StatusNoContent)
}
