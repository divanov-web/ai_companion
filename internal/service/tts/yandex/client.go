package yandex

import (
	"OpenAIClient/internal/config"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const endpoint = "https://tts.api.cloud.yandex.net/speech/v1/tts:synthesize"

// Client реализует синтез речи через Yandex SpeechKit.
type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{http: http.DefaultClient}
}

// Synthesize выполняет запрос к Yandex TTS и возвращает аудио. cfg должен быть config.YandexTTSConfig.
func (c *Client) Synthesize(ctx context.Context, text string, _ string, cfg any) (string, io.ReadCloser, error) {
	yc, ok := cfg.(config.YandexTTSConfig)
	if !ok {
		return "", nil, errors.New("yandex tts: unexpected config type")
	}
	if strings.TrimSpace(yc.APIKey) == "" {
		return "", nil, errors.New("yandex tts: empty API key (set YC_TTS_API_KEY in .env/ENV or pass via flag)")
	}
	// Значения по умолчанию задаются исключительно в config.Defaults().
	// Здесь используем переданные из конфигурации параметры как есть.
	voice := yc.Voice
	format := strings.ToLower(yc.Format)
	speed := yc.Speed
	emotion := strings.ToLower(yc.Emotion)

	form := url.Values{}
	form.Set("text", text)
	form.Set("voice", voice)
	form.Set("format", format)
	form.Set("speed", speed)
	form.Set("emotion", emotion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Api-Key "+yc.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	// Не закрываем здесь resp.Body — отдадим вызывающему звук как ReadCloser

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(b) == 0 {
			b = []byte(resp.Status)
		}
		// Закрыть тело при ошибке
		resp.Body.Close()
		return "", nil, fmt.Errorf("yandex tts error: status=%d, body=%s", resp.StatusCode, bytes.TrimSpace(b))
	}

	return format, resp.Body, nil
}

//
