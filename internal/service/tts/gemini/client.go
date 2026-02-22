package gemini

import (
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/tts/player"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
)

// По умолчанию используем Cloud TTS v1beta1 text:synthesize, совместимый с Generative AI TTS.
const defaultEndpoint = "https://texttospeech.googleapis.com/v1beta1/text:synthesize"

// Client реализует синтез речи через Cloud Text-to-Speech: Gemini‑TTS и воспроизводит результат.
type Client struct {
	http   *http.Client
	player player.Player
	logger *zap.SugaredLogger
}

func New(p player.Player, logger *zap.SugaredLogger) *Client {
	return &Client{http: http.DefaultClient, player: p, logger: logger}
}

// requestPayload — максимально нейтральная структура, покрывающая input.prompt и voice.model_name.
type requestPayload struct {
	Input struct {
		Prompt string `json:"prompt,omitempty"`
		Text   string `json:"text,omitempty"`
		Ssml   string `json:"ssml,omitempty"`
	} `json:"input"`
	Voice struct {
		ModelName    string `json:"modelName,omitempty"`
		LanguageCode string `json:"languageCode,omitempty"`
		VoiceName    string `json:"name,omitempty"`
	} `json:"voice"`
	AudioConfig struct {
		AudioEncoding  string   `json:"audioEncoding,omitempty"`
		SpeakingRate   float64  `json:"speakingRate,omitempty"`
		Pitch          float64  `json:"pitch,omitempty"`
		VolumeGainDb   float64  `json:"volumeGainDb,omitempty"`
		EffectsProfile []string `json:"effectsProfileId,omitempty"`
	} `json:"audioConfig"`
}

// В некоторых реализациях ответ приходит как бинарный аудио‑стрим, в других — JSON с base64.
// Поддержим оба варианта для гибкости.
type jsonAudioResponse struct {
	AudioContent string `json:"audioContent"`
}

// Synthesize выполняет запрос к Gemini‑TTS и воспроизводит аудио. cfg должен быть config.GeminiTTSConfig.
func (c *Client) Synthesize(ctx context.Context, text string, prompt string, cfg any) error {
	gc, ok := cfg.(config.GeminiTTSConfig)
	if !ok {
		return errors.New("gemini tts: unexpected config type")
	}
	// Валидация входа: Cloud TTS ожидает text или ssml. Пустой ввод приведёт к 400.
	if strings.TrimSpace(text) == "" {
		return errors.New("gemini tts: empty input text — provide -text or non-empty SSML")
	}

	// Формирование запроса
	var rp requestPayload
	inType := strings.ToLower(strings.TrimSpace(gc.InputType))
	switch inType {
	case "ssml":
		rp.Input.Ssml = text
	case "text", "prompt", "":
		rp.Input.Text = text
	default:
		// Неизвестный тип — отправим как text, чтобы избежать 400 INVALID_ARGUMENT.
		rp.Input.Text = text
	}
	// Промпт из конфигурации — используется только Gemini. Пустым не отправляем.
	if p := strings.TrimSpace(prompt); p != "" {
		rp.Input.Prompt = p
	}
	rp.Voice.ModelName = strings.TrimSpace(gc.ModelName)
	rp.Voice.LanguageCode = strings.TrimSpace(gc.Language)
	rp.Voice.VoiceName = strings.TrimSpace(gc.VoiceName)
	// Требование пользователя: использовать MP3 и зафиксировать соответствующий AudioEncoding в коде
	rp.AudioConfig.AudioEncoding = "MP3"
	rp.AudioConfig.SpeakingRate = gc.SpeakingRate
	rp.AudioConfig.Pitch = gc.Pitch
	rp.AudioConfig.VolumeGainDb = gc.VolumeGainDb
	if ep := strings.TrimSpace(gc.EffectsProfileID); ep != "" {
		rp.AudioConfig.EffectsProfile = []string{ep}
	}

	body, err := json.Marshal(&rp)
	if err != nil {
		return err
	}

	endpoint := strings.TrimSpace(gc.Endpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	// Создаём OAuth2 HTTP‑клиент только через ADC/metadata. API Key не используется.
	httpClient, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return errors.New("gemini tts: ADC credentials not found. Set GOOGLE_APPLICATION_CREDENTIALS to a service account JSON or run in GCE/GKE with default credentials")
	}

	url := endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	started := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if c.logger != nil {
		c.logger.Infow("Gemini TTS request completed", "status", resp.StatusCode, "took", time.Since(started).String())
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(b) == 0 {
			b = []byte(resp.Status)
		}
		return fmt.Errorf("gemini tts error: status=%d, body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	// JSON с base64 полем audioContent
	var jr jsonAudioResponse
	dec := json.NewDecoder(io.LimitReader(resp.Body, 5<<20)) // до 5 МБ JSON
	if err := dec.Decode(&jr); err != nil {
		return fmt.Errorf("gemini tts: decode json response: %w", err)
	}
	if strings.TrimSpace(jr.AudioContent) == "" {
		return errors.New("gemini tts: empty audioContent in response")
	}
	data, err := base64.StdEncoding.DecodeString(jr.AudioContent)
	if err != nil {
		return fmt.Errorf("gemini tts: base64 decode: %w", err)
	}
	rc := io.NopCloser(bytes.NewReader(data))
	// audioEncoding=MP3 — играем как MP3
	return c.player.Play("mp3", rc)
}
