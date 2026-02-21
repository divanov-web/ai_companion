package dota

import (
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/events"
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Ensure interface compliance
var _ events.EventServer = (*DotaEventServer)(nil)

type DotaEventServer struct {
	cfg     config.EventServerConfig
	srv     *http.Server
	logger  *zap.SugaredLogger
	running atomic.Bool
}

func NewDotaEventServer(cfg config.EventServerConfig, logger *zap.SugaredLogger) *DotaEventServer {
	if cfg.BindAddr == "" {
		cfg.BindAddr = "127.0.0.1:3000"
	}
	if cfg.Path == "" {
		cfg.Path = "/"
	}
	s := &DotaEventServer{cfg: cfg, logger: logger}

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

func (s *DotaEventServer) Start(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return nil
	}
	go func() {
		s.logger.Infow("DotaEventServer listening", "addr", s.srv.Addr, "path", s.cfg.Path)
		if err := s.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			s.logger.Errorw("DotaEventServer stopped with error", "error", err)
		} else {
			s.logger.Infow("DotaEventServer stopped")
		}
	}()

	// Watch for context cancellation to stop the server
	go func() {
		<-ctx.Done()
		_ = s.Stop(context.WithoutCancel(ctx))
	}()
	return nil
}

func (s *DotaEventServer) Stop(ctx context.Context) error {
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

func (s *DotaEventServer) Addr() string { return s.cfg.BindAddr }

func (s *DotaEventServer) handleEvent(w http.ResponseWriter, r *http.Request) {
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

	// Log raw JSON
	ts := time.Now().Format(time.RFC3339)
	ua := r.Header.Get("User-Agent")
	s.logger.Infow("GSI event received",
		"ts", ts,
		"remote", r.RemoteAddr,
		"ua", ua,
		"bytes", len(body),
		"raw", string(body),
	)

	// По требованию: выводить в консоль весь JSON вместо отдельного поля map.
	// Дополнительное действия не требуются, так как выше уже напечатано поле "raw" с полным телом.
	// Оставляем только ответ 204.
	w.WriteHeader(http.StatusNoContent)
}
