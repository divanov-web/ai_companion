package main

import (
	"OpenAIClient/internal/service/stt/handy"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Конфигурация из флагов (пока локально в main, как по ТЗ)
	winDur := flag.Duration("window", time.Second, "окно времени для совпадения буфера и Ctrl+Enter (например, 1s)")
	hkDelay := flag.Duration("hotkey-delay", 100*time.Millisecond, "задержка реакции на Ctrl+Enter перед фиксацией текста (например, 500ms)")
	flag.Parse()

	fmt.Println("Программа запущена")

	ctx, stop := signal.NotifyContext(context.Background(), osInterruptSignals()...)
	defer stop()

	svc := handy.New(handy.Config{HandyWindow: *winDur, HotkeyDelay: *hkDelay})

	// Потребитель событий — печать в консоль
	go func() {
		for ev := range svc.Events() {
			ts := ev.At.Format("15:04:05.000")
			switch ev.Type {
			case handy.EventClipboardChanged:
				fmt.Printf("[CLIPBOARD %s] %s\n", ts, preview(ev.Text, 500))
			case handy.EventCtrlEnter:
				fmt.Printf("[CTRL+ENTER %s]\n", ts)
			case handy.EventHandyFinalText:
				fmt.Printf("Текст пойман: %s\n", ev.Text)
			}
		}
	}()

	if err := svc.Run(ctx); err != nil {
		fmt.Printf("Сервис завершился с ошибкой: %v\n", err)
	}
}

func preview(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func osInterruptSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}
