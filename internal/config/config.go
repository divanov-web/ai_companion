package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	DebugMode           bool     `env:"DEBUG_MODE"`                      //Режим дебага
	AssistantPrompt     string   `env:"ASSISTANT_PROMPT"`                //Текст промпта ассистента диалога
	AssistantSentences  int      `env:"ASSISTANT_SENTENCES"`             // Количество предложений в ответе ассистента
	CharacterList       []string `env:"CHARACTER_LIST" envSeparator:";"` // Список характеров/стилей персонажа, конкатенируется со стартовым промптом
	SpeechPrompt        []string `env:"SPEECH_PROMPT" envSeparator:";"`  // Список фиксированных сообщений для каждого тика; выбирается случайно
	ImagesSourceDir     string   `env:"IMAGES_SOURCE_DIR"`               // Папка с исходными изображениями
	ImagesToPick        int      `env:"IMAGES_TO_PICK"`                  // Сколько последних изображений брать
	ImagesTTLSeconds    int      `env:"IMAGES_TTL_SECONDS"`              // Время, через которое картинки считаются старыми и их надо удалить, в секундах
	HistoryHeader       string   `env:"HISTORY_HEADER"`                  // Заголовок блока с историей ответов ИИ
	MaxHistoryRecords   int      `env:"MAX_HISTORY_RECORDS"`             // Максимум хранимых ответов ИИ в локальной истории
	NotificationSendAI  string   `env:"NOTIFICATION_SEND_AI"`            // Путь к звуку уведомления ИИ (получено сообщение)
	NotificationSendTTS string   `env:"NOTIFICATION_SEND_TTS"`           // Путь к звуку перед TTS (озвучка ответа)
	// Скриншоттер
	ScreenshotIntervalSeconds int `env:"SCREENSHOT_INTERVAL_SECONDS"` // Периодичность снятия скриншотов всего экрана, в секундах
	// Общий переключатель сервиса TTS и конфиг Google/Gemini TTS
	TTSService string `env:"TTS_SERVICE"` // yandex|google|gemini, по умолчанию google
	GoogleTTS  GoogleTTSConfig
	GeminiTTS  GeminiTTSConfig
	YandexTTS  YandexTTSConfig

	// Настройки таймера (Scheduler)
	TimerIntervalSeconds   int    `env:"TIMER_INTERVAL_SECONDS"`   // Базовый интервал между тиками
	TickTimeoutSeconds     int    `env:"TICK_TIMEOUT_SECONDS"`     // Таймаут одного тика
	OverlapPolicy          string `env:"OVERLAP_POLICY"`           // Политика при наложении: skip|preempt
	MaxConsecutiveErrors   int    `env:"MAX_CONSECUTIVE_ERRORS"`   // Сколько ошибок подряд до остановки приложения
	RotateConversationEach int    `env:"ROTATE_CONVERSATION_EACH"` // Каждые N успешных запросов начинать новый диалог

	// STT (Handy) и Speech
	STTHandyWindow  time.Duration `env:"STT_HANDY_WINDOW"`  // Окно совпадения буфера и хоткея
	STTHotkeyDelay  time.Duration `env:"STT_HOTKEY_DELAY"`  // Задержка реакции на Ctrl+Enter
	SpeechHeader    string        `env:"SPEECH_HEADER"`     // Заголовок для блока сообщений из речи
	SpeechMax       int           `env:"SPEECH_MAX"`        // Максимум хранимых сообщений речи
	EnableEarlyTick bool          `env:"ENABLE_EARLY_TICK"` // Запускать тик ранее при наличии сообщений речи

	// Chat / Twitch
	ChatHistoryHeader string `env:"CHAT_HISTORY_HEADER"` // Заголовок блока сообщений из чата
	ChatMax           int    `env:"CHAT_MAX"`            // Максимум хранимых сообщений чата
	TwitchUsername    string `env:"TWITCH_USERNAME"`     // Имя пользователя Twitch (логин)
	TwitchOAuthToken  string `env:"TWITCH_OAUTH_TOKEN"`  // OAuth токен Twitch (может быть без префикса oauth:)
	TwitchChannel     string `env:"TWITCH_CHANNEL"`      // Канал Twitch (один), без #

	// StateServer — приёмник игрового состояния (например, Dota GSI)
	StateServer StateServerConfig

	// State — буфер последних сообщений из игрового состояния
	StateHeader string `env:"STATE_HEADER"` // Заголовок для блока State
	StateMax    int    `env:"STATE_MAX"`    // Максимум хранимых сообщений State

	// Screenshotter — включение/выключение фоновой съёмки скриншотов
	ScreenshotEnabled bool `env:"SCREENSHOT_ENABLED"` // По умолчанию включён
}

// YandexTTSConfig конфигурация для синтеза речи через Yandex SpeechKit.
type YandexTTSConfig struct {
	APIKey  string `env:"YC_TTS_API_KEY"` // Ключ берём из .env/ENV. Если пуст — при использовании будет ошибка
	Voice   string `env:"YC_TTS_VOICE"`   // Голос, по умолчанию filipp
	Format  string `env:"YC_TTS_FORMAT"`  // mp3|wav, по умолчанию mp3
	Speed   string `env:"YC_TTS_SPEED"`   // Скорость синтеза (1.0 по умолчанию в API); 1.3 = ~30% быстрее
	Emotion string `env:"YC_TTS_EMOTION"` // Эмоциональная окраска: neutral|good|evil. По умолчанию evil
	Volume  int    `env:"YC_TTS_VOLUME"`  // Громкость 0-100; 100 — не изменять громкость todo вероятно есть баг, что громкость уменьшается слишком быстро
}

// GoogleTTSConfig — конфигурация для синтеза речи через Google Cloud Text-to-Speech.
type GoogleTTSConfig struct {
	// Путь к файлу ключа сервисного аккаунта (ENV GOOGLE_APPLICATION_CREDENTIALS).
	CredentialsPath string  `env:"GOOGLE_APPLICATION_CREDENTIALS"`
	Language        string  `env:"GOOGLE_TTS_LANGUAGE"`
	Voice           string  `env:"GOOGLE_TTS_VOICE"`
	SpeakingRate    float64 `env:"GOOGLE_TTS_SPEAKING_RATE"`
	Pitch           float64 `env:"GOOGLE_TTS_PITCH"`
	VolumeGainDb    float64 `env:"GOOGLE_TTS_VOLUME_DB"`
	// Эффект профиля устройства воспроизведения
	EffectsProfileID string `env:"GOOGLE_TTS_EFFECTS_PROFILE_ID"`
	// Тип входа: text|ssml (auto при отсутствии явного выбора)
	InputType string `env:"GOOGLE_TTS_INPUT_TYPE"`
}

// GeminiTTSConfig — конфигурация для синтеза речи через Cloud Text-to-Speech (Gemini-TTS).
type GeminiTTSConfig struct {
	// Модель голоса Gemini‑TTS
	ModelName        string  `env:"GEMINI_TTS_MODEL_NAME"`
	Language         string  `env:"GEMINI_TTS_LANGUAGE"`
	VoiceName        string  `env:"GEMINI_TTS_VOICE_NAME"`
	SpeakingRate     float64 `env:"GEMINI_TTS_SPEAKING_RATE"`
	Pitch            float64 `env:"GEMINI_TTS_PITCH"`
	VolumeGainDb     float64 `env:"GEMINI_TTS_VOLUME_DB"`
	EffectsProfileID string  `env:"GEMINI_TTS_EFFECTS_PROFILE_ID"`
	// Тип входа: text|ssml|prompt
	InputType string `env:"GEMINI_TTS_INPUT_TYPE"`
	Endpoint  string `env:"GEMINI_TTS_ENDPOINT"`
	Prompt    string `env:"GEMINI_TTS_PROMPT"`
}

// StateServerConfig — конфигурация сервиса приёма игрового состояния.
type StateServerConfig struct {
	Enabled   bool   `env:"STATE_SERVER_ENABLED"`    // Главный флаг включения/выключения
	BindAddr  string `env:"STATE_SERVER_BIND_ADDR"`  // Адрес слушателя, напр. 127.0.0.1:3000
	Path      string `env:"STATE_SERVER_PATH"`       // HTTP‑путь, напр. "/"
	AuthToken string `env:"STATE_SERVER_AUTH_TOKEN"` // Токен авторизации (опционально)
}

// Defaults возвращает конфигурацию со значениями по умолчанию.
// Значения могут быть переопределены из .env и переменных окружения.
func Defaults() *Config {
	return &Config{
		DebugMode:                 false,
		AssistantPrompt:           "Ты помощник капитана и озвучиваешь то, что видишь на картинках",
		AssistantSentences:        3,
		CharacterList:             []string{""},
		SpeechPrompt:              []string{"доложи статус"},
		ImagesSourceDir:           "images\\sharex",
		ImagesToPick:              3,
		ImagesTTLSeconds:          60,
		HistoryHeader:             "история предыдущих ответов AI:",
		SpeechHeader:              "Моя реплика",
		MaxHistoryRecords:         5,
		ScreenshotIntervalSeconds: 2,
		ScreenshotEnabled:         true,
		// Таймер по умолчанию
		RotateConversationEach: 3,
		TimerIntervalSeconds:   5, //Задержка перед началом тика
		TickTimeoutSeconds:     60,
		OverlapPolicy:          "skip", //`skip`|`preempt`
		MaxConsecutiveErrors:   3,
		NotificationSendAI:     "sound/notification2.mp3",
		NotificationSendTTS:    "sound/notification3.mp3",
		// STT/Speech
		STTHandyWindow:  time.Second,
		STTHotkeyDelay:  100 * time.Millisecond,
		SpeechMax:       10,
		EnableEarlyTick: true,
		// Chat/Twitch
		ChatHistoryHeader: "Сообщения из чата",
		ChatMax:           30,
		// По умолчанию используем Google TTS
		TTSService: "gemini",
		YandexTTS: YandexTTSConfig{
			APIKey:  "",
			Voice:   "omazh",
			Format:  "mp3",
			Speed:   "1.3",
			Emotion: "evil",
			Volume:  100,
		},
		StateServer: StateServerConfig{
			Enabled:  false,
			BindAddr: "127.0.0.1:3000",
			Path:     "/",
		},
		GoogleTTS: GoogleTTSConfig{
			CredentialsPath:  "service-account.json",
			Language:         "ru-RU",
			Voice:            "ru-RU-Standard-A",
			SpeakingRate:     1.0,
			Pitch:            0.0,
			VolumeGainDb:     0.0,
			EffectsProfileID: "large-home-entertainment-class-device",
			InputType:        "",
		},
		GeminiTTS: GeminiTTSConfig{
			ModelName:        "gemini-2.5-pro-tts",
			Language:         "ru-RU",
			VoiceName:        "Achernar", //кавайные: Achernar, Kore, Leda
			Prompt:           "Read kawaii",
			SpeakingRate:     1.2,
			Pitch:            0.0,
			VolumeGainDb:     0.0,
			EffectsProfileID: "",
			InputType:        "prompt",
			Endpoint:         "https://texttospeech.googleapis.com/v1beta1/text:synthesize",
		},
		StateHeader: "Состояние игры",
		StateMax:    3,
	}
}

// NewConfig загружает конфигурацию приложения.
func NewConfig() *Config {
	_ = godotenv.Load()

	cfg := Defaults()
	_ = env.Parse(cfg)

	// Валидация для Google TTS: проверка наличия и доступности файла ключа
	if strings.EqualFold(cfg.TTSService, "google") {
		cred := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
		if cred == "" {
			panic(fmt.Errorf("google tts: переменная окружения GOOGLE_APPLICATION_CREDENTIALS не задана; укажите её или задайте путь к ключу сервисного аккаунта"))
		}
		if _, err := os.Stat(cred); err != nil {
			panic(fmt.Errorf("google tts: файл ключа не найден: %s", cred))
		}
	}

	return cfg
}
