package main

import (
	"OpenAIClient/internal/config"
	gtts "OpenAIClient/internal/service/tts/gemini"
	"OpenAIClient/internal/service/tts/player"
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
)

// Тестовый скрипт для проверки Cloud Text-to-Speech: Gemini-TTS.
// Авторизация: только через GOOGLE_APPLICATION_CREDENTIALS (ADC) / Application Default Credentials.
// Пример запуска:
//
//	go run ./cmd/gemeni-tts -text "Привет! Это тест синтеза речи Gemini TTS"
func main() {
	var (
		text string
		// Возможность быстрого переопределения модели флагом (необязательно)
		model  string
		voice  string
		lang   string
		rate   float64
		pitch  float64
		volume float64
		input  string
	)

	// Базовая конфигурация приложения (подтягивает .env и ENV)
	cfg := config.NewConfig()

	// Дефолтный текст для проверки
	flag.StringVar(&text, "text", "Это тестовый запрос к сервису Gemini TTS. Проверка связи и синтеза речи.", "Тестовый текст для синтеза речи")
	// Опциональные быстрые переопределения параметров Gemini TTS
	flag.StringVar(&model, "model", cfg.GeminiTTS.ModelName, "Имя модели Gemini TTS (напр. tts-1)")
	flag.StringVar(&voice, "voice", cfg.GeminiTTS.VoiceName, "Имя/вариант голоса модели (если поддерживается)")
	flag.StringVar(&lang, "lang", cfg.GeminiTTS.Language, "Язык синтеза, напр. ru-RU")
	flag.Float64Var(&rate, "rate", cfg.GeminiTTS.SpeakingRate, "Скорость речи (1.0 по умолчанию)")
	flag.Float64Var(&pitch, "pitch", cfg.GeminiTTS.Pitch, "Тон (полутоны)")
	flag.Float64Var(&volume, "volume", cfg.GeminiTTS.VolumeGainDb, "Усиление громкости (дБ)")
	flag.StringVar(&input, "input", cfg.GeminiTTS.InputType, "Тип входа: text|ssml|prompt (по умолчанию prompt)")
	flag.Parse()

	// Обновим поля GeminiTTS из флагов (если переданы)
	if model != "" {
		cfg.GeminiTTS.ModelName = model
	}
	if voice != "" {
		cfg.GeminiTTS.VoiceName = voice
	}
	if lang != "" {
		cfg.GeminiTTS.Language = lang
	}
	cfg.GeminiTTS.SpeakingRate = rate
	cfg.GeminiTTS.Pitch = pitch
	cfg.GeminiTTS.VolumeGainDb = volume
	if input != "" {
		cfg.GeminiTTS.InputType = input
	}

	// Проверим наличие учётных данных ADC (или метаданных GCP). Для локального запуска ожидаем GOOGLE_APPLICATION_CREDENTIALS
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		fmt.Println("Ошибка: не найдены учётные данные ADC. Установите GOOGLE_APPLICATION_CREDENTIALS (путь к service-account.json).")
		os.Exit(1)
	}

	// Логгер и плеер
	zl, _ := zap.NewDevelopment()
	logger := zl.Sugar()
	defer zl.Sync() // flush

	p := player.New() // громкость регулируется на стороне провайдера
	client := gtts.New(p, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Synthesize(ctx, text, cfg.GeminiTTS); err != nil {
		logger.Errorw("Gemini TTS synthesize failed", "error", err)
		os.Exit(1)
	}

	logger.Infow("Gemini TTS synthesize finished")
}
