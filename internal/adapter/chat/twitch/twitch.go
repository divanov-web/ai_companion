package twitch

import (
	svcchat "OpenAIClient/internal/service/chat"
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	twitchirc "github.com/gempir/go-twitch-irc/v4"
	"go.uber.org/zap"
)

// Config хранит параметры подключения к Twitch IRC.
type Config struct {
	Username string
	OAuth    string // может быть с/без префикса oauth:
	Channel  string // без #, регистр не важен
}

// Run запускает клиент Twitch IRC и пересылает отфильтрованные сообщения в chatSvc.
// Базовые реконнекты обеспечиваются клиентом; функция завершается по отмене ctx.
func Run(ctx context.Context, logger *zap.SugaredLogger, cfg Config, chatSvc *svcchat.Chat) error {
	if chatSvc == nil {
		return nil
	}
	username := strings.ToLower(strings.TrimSpace(cfg.Username))
	token := strings.TrimSpace(cfg.OAuth)
	channel := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(cfg.Channel), "#"))
	if username == "" || token == "" || channel == "" {
		logger.Warnw("Twitch chat not configured: missing env", "username", username != "", "token", token != "", "channel", channel != "")
		return nil
	}
	if !strings.HasPrefix(token, "oauth:") {
		token = "oauth:" + token
	}

	client := twitchirc.NewClient(username, token)

	// Префильтр URL и минимальный антиспам: дедуп одинаковых сообщений пользователя в течение окна.
	urlRe := regexp.MustCompile(`https?://[^\s]+`)
	const spamWindow = 5 * time.Second
	type lastMsg struct {
		text string
		at   time.Time
	}
	var mu sync.Mutex
	lastByUser := map[string]lastMsg{}

	client.OnConnect(func() {
		logger.Infow("Twitch connected", "as", username, "join", channel)
		client.Join(channel)
	})

	client.OnPrivateMessage(func(msg twitchirc.PrivateMessage) {
		user := strings.TrimSpace(msg.User.Name)
		text := strings.TrimSpace(msg.Message)
		if text == "" || user == "" {
			return
		}
		// Вырезаем URL
		text = urlRe.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
		if text == "" { // всё было URL — пропускаем
			return
		}

		// Антиспам: одинаковый текст от того же пользователя в течение окна — дропаем
		now := time.Now()
		drop := false
		mu.Lock()
		if lm, ok := lastByUser[user]; ok {
			if lm.text == text && now.Sub(lm.at) <= spamWindow {
				drop = true
			}
		}
		if !drop {
			lastByUser[user] = lastMsg{text: text, at: now}
		}
		mu.Unlock()
		if drop {
			return
		}

		// Формат: HH:MM:SS User: Text
		ts := now.Format("15:04:05")
		line := ts + " " + user + ": " + text
		chatSvc.Add(line)
	})

	errCh := make(chan error, 1)
	go func() { errCh <- client.Connect() }()

	select {
	case <-ctx.Done():
		_ = client.Disconnect()
		// Подождём чуть-чуть корректного завершения
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
		}
		return context.Canceled
	case err := <-errCh:
		if err != nil {
			logger.Errorw("twitch connect error", "error", err)
		}
		return err
	}
}
