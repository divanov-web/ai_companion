package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	twitch "github.com/gempir/go-twitch-irc/v4"
)

type ChatMessage struct {
	Channel   string
	User      string
	Text      string
	MessageID string
	Timestamp time.Time
}

func main() {
	username := strings.ToLower(mustEnv("TWITCH_USERNAME"))
	token := mustEnv("TWITCH_OAUTH_TOKEN")
	channel := mustEnv("TWITCH_CHANNEL")

	// Нормализуем токен и канал:
	// - токен для IRC должен иметь префикс "oauth:", но многие уже задают его с префиксом в переменной окружения
	if !strings.HasPrefix(token, "oauth:") {
		token = "oauth:" + token
	}
	// - канал без символа '#', в нижнем регистре
	channel = strings.ToLower(strings.TrimPrefix(channel, "#"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Шина сообщений (буфер, чтобы обработчик не тормозил чтение)
	msgCh := make(chan ChatMessage, 500)

	// Запускаем обработчик
	go func() {
		for m := range msgCh {
			handleMessage(m)
		}
	}()

	// Подключаемся к Twitch IRC
	client := twitch.NewClient(username, token)

	// Логи/события
	client.OnConnect(func() {
		log.Printf("Connected as %s, joining #%s", username, channel)
		client.Join(channel)
	})

	client.OnPrivateMessage(func(msg twitch.PrivateMessage) {
		m := ChatMessage{
			Channel:   msg.Channel,
			User:      msg.User.Name,
			Text:      msg.Message,
			MessageID: msg.ID, // Twitch message id
			Timestamp: time.Now(),
		}

		// Не блокируем чтение: если буфер полон — можно дропнуть или логировать
		select {
		case msgCh <- m:
		default:
			log.Printf("WARN: msg buffer full, dropping message id=%s", m.MessageID)
		}
	})

	// Примечание: в v4 нет OnDisconnect, логи разрыва получим из Connect()/errCh

	// Connect() блокирует текущую горутину, поэтому запускаем в отдельной
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Connect()
	}()

	// Graceful shutdown
	select {
	case <-ctx.Done():
		log.Println("Shutting down...")
		_ = client.Disconnect()
		// Дожидаемся завершения Connect(), чтобы коллбеки не писали в закрытый канал
		select {
		case err := <-errCh:
			if err != nil {
				log.Printf("disconnect result: %v", err)
			}
		case <-time.After(3 * time.Second):
			log.Printf("timeout waiting for disconnect")
		}
		close(msgCh)
	case err := <-errCh:
		// Ошибка при подключении/работе
		log.Fatalf("connect error: %v", err)
	}
}

func handleMessage(m ChatMessage) {
	// ТВОЯ будущая логика:
	// - команды (!balance, !order)
	// - фильтрация по пользователям/ролям
	// - отправка в БД/очередь
	// - аналитика и т.д.
	fmt.Printf("[%s] %s: %s\n", m.Channel, m.User, m.Text)
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("env %s is required", k)
	}
	return v
}
