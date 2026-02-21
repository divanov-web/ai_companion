package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Simple HTTP server to receive Dota 2 Game State Integration (GSI) POST callbacks
// and print their JSON payload to the console. Nothing more.
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", gsiHandler)

	srv := &http.Server{
		Addr:              "127.0.0.1:3000",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("GSI test server listening on http://%s/ (POST only)\n", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on Ctrl+C / SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("shutdown timeout"))
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
		_ = srv.Close()
	}
	log.Println("server stopped")
}

func gsiHandler(w http.ResponseWriter, r *http.Request) {
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

	// Print minimal info + raw body as string
	ts := time.Now().Format(time.RFC3339)
	ua := r.Header.Get("User-Agent")
	auth := r.Header.Get("Authorization")
	if auth == "" {
		auth = r.Header.Get("Authentication") // some integrations may use this
	}
	fmt.Printf("[%s] GSI POST from %s UA='%s' Auth='%s' bytes=%d\n%s\n\n", ts, r.RemoteAddr, ua, auth, len(body), string(body))

	// No response body needed; Dota 2 GSI considers 2xx as a success.
	w.WriteHeader(http.StatusNoContent)
}
