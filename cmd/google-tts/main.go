package main

import (
	"OpenAIClient/internal/config"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2/google"
)

// Небольшая утилита: делает GET к Google TTS Voices и печатает список голосов для ru-RU.
// Конфигурация берётся из internal/config (в частности путь к cred-файлу сервисного аккаунта).
func main() {
	cfg := config.NewConfig()

	// Установим GOOGLE_APPLICATION_CREDENTIALS из конфига, если не задано в окружении.
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" && cfg.GoogleTTS.CredentialsPath != "" {
		_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", cfg.GoogleTTS.CredentialsPath)
	}

	// Подготовим контекст с таймаутом.
	ctx, cancel := context.WithTimeoutCause(context.Background(), 15*time.Second, errors.New("google tts voices request timeout"))
	defer cancel()

	// Получим учётные данные по ADC и токен для вызова REST API.
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		fmt.Println("не удалось найти учётные данные Google (ADC):", err)
		os.Exit(1)
	}
	tok, err := creds.TokenSource.Token()
	if err != nil {
		fmt.Println("не удалось получить токен доступа Google:", err)
		os.Exit(1)
	}

	// Язык берём из конфига, по умолчанию ru-RU.
	lang := cfg.GoogleTTS.Language
	if lang == "" {
		lang = "ru-RU"
	}

	url := fmt.Sprintf("https://texttospeech.googleapis.com/v1/voices?languageCode=%s", lang)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Println("не удалось создать запрос:", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)

	// Выполняем запрос
	hc := &http.Client{Timeout: 20 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		fmt.Println("ошибка при выполнении запроса:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Печатаем как есть
		var raw any
		_ = json.NewDecoder(resp.Body).Decode(&raw)
		b, _ := json.MarshalIndent(raw, "", "  ")
		if len(b) == 0 {
			fmt.Printf("Google TTS Voices: status=%d\n", resp.StatusCode)
		} else {
			fmt.Printf("Google TTS Voices: status=%d, body=%s\n", resp.StatusCode, string(b))
		}
		os.Exit(1)
	}

	// Успех: выводим JSON с голосами
	var payload struct {
		Voices []any `json:"voices"`
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&payload); err != nil {
		// Если не удалось распарсить — напечатаем сырое тело
		// Повторный запрос делать не будем; просто сообщим об ошибке парсинга
		fmt.Println("не удалось распарсить ответ Google TTS Voices:", err)
		os.Exit(1)
	}

	// Печатаем результат в человекочитаемом виде
	out, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(out))
}
