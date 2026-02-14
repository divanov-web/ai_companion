package config

import (
	"flag"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	DebugMode          bool            `env:"DEBUG_MODE"`           //Режим дебага
	StartPrompt        string          `env:"START_PROMPT"`         //Текст стартового промпта диалога
	FixedMessage       string          `env:"FIXED_MESSAGE"`        // Фиксированный текст сообщения на каждом тике таймера
	ImagesSourceDir    string          `env:"IMAGES_SOURCE_DIR"`    // Папка с исходными изображениями
	ImagesProcessedDir string          `env:"IMAGES_PROCESSED_DIR"` // Папка для сохранения обработанных изображений
	ImagesToPick       int             `env:"IMAGES_TO_PICK"`       // Сколько последних изображений брать
	ImagesTTLSeconds   int             `env:"IMAGES_TTL_SECONDS"`   // Время, через которое картинки считаются старыми и их надо удалить, в секундах
	YandexTTS          YandexTTSConfig // Конфигурация TTS (Yandex SpeechKit)

	// Настройки таймера (Scheduler)
	TimerIntervalSeconds   int    `env:"TIMER_INTERVAL_SECONDS"`   // Базовый интервал между тиками
	TimerJitterPercent     int    `env:"TIMER_JITTER_PERCENT"`     // Джиттер в процентах от интервала (+/-), 0-100
	TickTimeoutSeconds     int    `env:"TICK_TIMEOUT_SECONDS"`     // Таймаут одного тика
	OverlapPolicy          string `env:"OVERLAP_POLICY"`           // Политика при наложении: skip|preempt
	MaxConsecutiveErrors   int    `env:"MAX_CONSECUTIVE_ERRORS"`   // Сколько ошибок подряд до остановки приложения
	RotateConversationEach int    `env:"ROTATE_CONVERSATION_EACH"` // Каждые N успешных запросов начинать новый диалог
}

// Defaults возвращает конфигурацию с предустановленными значениями по умолчанию.
// Эти значения перекрываются .env, переменными окружения и флагами CLI.
func Defaults() *Config {
	return &Config{
		DebugMode:          false,
		StartPrompt:        "Ты помощник капитана и озвучиваешь то, что видишь на картинках",
		FixedMessage:       "доложи статус",
		ImagesSourceDir:    "images\\sharex",
		ImagesProcessedDir: "images\\processed",
		ImagesToPick:       3,
		ImagesTTLSeconds:   60,
		// Таймер по умолчанию
		TimerIntervalSeconds:   10,
		TimerJitterPercent:     10,
		TickTimeoutSeconds:     60,
		OverlapPolicy:          "skip", //`skip`|`preempt`
		MaxConsecutiveErrors:   3,
		RotateConversationEach: 1,
		YandexTTS: YandexTTSConfig{
			APIKey:  "", // ключ берём из .env/ENV, если пусто — будет ошибка при использовании
			Voice:   "omazh",
			Format:  "mp3",  // поддерживаемые форматы: mp3|wav|oggopus (проигрывание mp3/wav)
			Speed:   "1.3",  // ускорение речи ~30% относительно дефолта 1.0
			Emotion: "evil", // эмоциональная окраска по умолчанию
		},
	}
}

// NewConfig загружает конфигурацию приложения.
func NewConfig() *Config {
	_ = godotenv.Load()

	// Стартуем с дефолтов, затем перекрываем .env/окружением и флагами
	cfg := Defaults()
	_ = env.Parse(cfg)

	flag.BoolVar(&cfg.DebugMode, "debug-mode", cfg.DebugMode, "включить режим дебага для отображения до инфы")
	flag.StringVar(&cfg.StartPrompt, "start-prompt", cfg.StartPrompt, "текст стартового промпта диалога")
	flag.StringVar(&cfg.FixedMessage, "fixed-message", cfg.FixedMessage, "фиксированный текст сообщения на каждом тике")
	flag.StringVar(&cfg.ImagesSourceDir, "images-source-dir", cfg.ImagesSourceDir, "путь к папке с исходными изображениями")
	flag.StringVar(&cfg.ImagesProcessedDir, "images-processed-dir", cfg.ImagesProcessedDir, "путь к папке для сохранения обработанных изображений")
	flag.IntVar(&cfg.ImagesToPick, "images-to-pick", cfg.ImagesToPick, "количество последних изображений для отправки")
	flag.IntVar(&cfg.ImagesTTLSeconds, "images-ttl-seconds", cfg.ImagesTTLSeconds, "время, через которое картинки считаются старыми и их надо удалить, в секундах")
	// Таймер
	flag.IntVar(&cfg.TimerIntervalSeconds, "timer-interval-seconds", cfg.TimerIntervalSeconds, "базовый интервал таймера в секундах")
	flag.IntVar(&cfg.TimerJitterPercent, "timer-jitter-percent", cfg.TimerJitterPercent, "джиттер таймера в процентах (0-100)")
	flag.IntVar(&cfg.TickTimeoutSeconds, "tick-timeout-seconds", cfg.TickTimeoutSeconds, "таймаут одного тика в секундах")
	flag.StringVar(&cfg.OverlapPolicy, "overlap-policy", cfg.OverlapPolicy, "политика наложения тиков: skip|preempt")
	flag.IntVar(&cfg.MaxConsecutiveErrors, "max-consecutive-errors", cfg.MaxConsecutiveErrors, "количество последовательных ошибок до остановки приложения")
	flag.IntVar(&cfg.RotateConversationEach, "rotate-conversation-each", cfg.RotateConversationEach, "каждые N успешных запросов начинать новый диалог")
	// Параметры Yandex TTS
	flag.StringVar(&cfg.YandexTTS.APIKey, "yc-tts-api-key", cfg.YandexTTS.APIKey, "API ключ Yandex SpeechKit TTS (перекрывает ENV)")
	flag.StringVar(&cfg.YandexTTS.Voice, "yc-tts-voice", cfg.YandexTTS.Voice, "голос для синтеза (напр. filipp, jane, oksana, zahar, ermil)")
	flag.StringVar(&cfg.YandexTTS.Format, "yc-tts-format", cfg.YandexTTS.Format, "формат аудио (mp3|wav|oggopus), проигрывание поддерживается для mp3 и wav")
	flag.StringVar(&cfg.YandexTTS.Speed, "yc-tts-speed", cfg.YandexTTS.Speed, "скорость речи (например, 1.0 по умолчанию; 1.3 = на 30% быстрее)")
	flag.StringVar(&cfg.YandexTTS.Emotion, "yc-tts-emotion", cfg.YandexTTS.Emotion, "эмоциональная окраска (neutral|good|evil). По умолчанию evil")
	flag.Parse()

	return cfg
}

// YandexTTSConfig конфигурация для синтеза речи через Yandex SpeechKit.
type YandexTTSConfig struct {
	APIKey  string `env:"YC_TTS_API_KEY"` // Ключ берём из .env/ENV. Если пуст — при использовании будет ошибка
	Voice   string `env:"YC_TTS_VOICE"`   // Голос, по умолчанию filipp
	Format  string `env:"YC_TTS_FORMAT"`  // mp3|wav|oggopus, по умолчанию mp3
	Speed   string `env:"YC_TTS_SPEED"`   // Скорость синтеза (1.0 по умолчанию в API); 1.3 = ~30% быстрее
	Emotion string `env:"YC_TTS_EMOTION"` // Эмоциональная окраска: neutral|good|evil. По умолчанию evil
}
