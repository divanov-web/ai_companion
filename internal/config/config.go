package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	DebugMode             bool     `env:"DEBUG_MODE"`                      //Режим дебага
	AssistantPrompt       string   `env:"ASSISTANT_PROMPT"`                //Текст промпта ассистента диалога
	CharacterList         []string `env:"CHARACTER_LIST" envSeparator:";"` // Список характеров/стилей персонажа, конкатенируется со стартовым промптом
	SpeechPrompt          []string `env:"SPEECH_PROMPT" envSeparator:";"`  // Список фиксированных сообщений для каждого тика; выбирается случайно
	ImagesSourceDir       string   `env:"IMAGES_SOURCE_DIR"`               // Папка с исходными изображениями
	ImagesToPick          int      `env:"IMAGES_TO_PICK"`                  // Сколько последних изображений брать
	ImagesTTLSeconds      int      `env:"IMAGES_TTL_SECONDS"`              // Время, через которое картинки считаются старыми и их надо удалить, в секундах
	HistoryHeader         string   `env:"HISTORY_HEADER"`                  // Заголовок блока с историей ответов ИИ
	MaxHistoryRecords     int      `env:"MAX_HISTORY_RECORDS"`             // Максимум хранимых ответов ИИ в локальной истории
	NotificationSoundPath string   `env:"NOTIFICATION_SOUND_PATH"`         // Путь к звуковому файлу уведомления
	// Скриншоттер
	ScreenshotIntervalSeconds int             `env:"SCREENSHOT_INTERVAL_SECONDS"` // Периодичность снятия скриншотов всего экрана, в секундах
	YandexTTS                 YandexTTSConfig // Конфигурация TTS (Yandex SpeechKit)
	// Общий переключатель сервиса TTS и конфиг Google TTS
	TTSService string `env:"TTS_SERVICE"` // yandex|google, по умолчанию google
	GoogleTTS  GoogleTTSConfig

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

// GoogleTTSConfig конфигурация для синтеза речи через Google Cloud Text-to-Speech.
type GoogleTTSConfig struct {
	// Путь к файлу ключа сервисного аккаунта. Фактически читается из ENV GOOGLE_APPLICATION_CREDENTIALS.
	// Здесь храним дефолт (service-account.json в корне проекта) для удобства.
	CredentialsPath string  `env:"GOOGLE_APPLICATION_CREDENTIALS"`
	Language        string  `env:"GOOGLE_TTS_LANGUAGE"`
	Voice           string  `env:"GOOGLE_TTS_VOICE"`
	SpeakingRate    float64 `env:"GOOGLE_TTS_SPEAKING_RATE"`
	Pitch           float64 `env:"GOOGLE_TTS_PITCH"`
	VolumeGainDb    float64 `env:"GOOGLE_TTS_VOLUME_DB"`
	// Эффект профиля устройства воспроизведения (оптимизация эквализации), напр. large-home-entertainment-class-device
	EffectsProfileID string `env:"GOOGLE_TTS_EFFECTS_PROFILE_ID"`
	// Тип входа: text|ssml. Пусто — auto (по наличию тега <speak> в тексте).
	InputType string `env:"GOOGLE_TTS_INPUT_TYPE"`
}

// StateServerConfig конфигурация сервиса приёма игрового состояния.
type StateServerConfig struct {
	Enabled   bool   `env:"STATE_SERVER_ENABLED"`    // Главный флаг включения/выключения
	BindAddr  string `env:"STATE_SERVER_BIND_ADDR"`  // Адрес слушателя, напр. 127.0.0.1:3000
	Path      string `env:"STATE_SERVER_PATH"`       // HTTP‑путь, напр. "/"
	AuthToken string `env:"STATE_SERVER_AUTH_TOKEN"` // Токен авторизации (опционально)
}

// Defaults возвращает конфигурацию с предустановленными значениями по умолчанию.
// Эти значения перекрываются .env, переменными окружения и флагами CLI.
func Defaults() *Config {
	return &Config{
		DebugMode:                 false,
		AssistantPrompt:           "Ты помощник капитана и озвучиваешь то, что видишь на картинках",
		CharacterList:             []string{""}, // по умолчанию один пустой характер
		SpeechPrompt:              []string{"доложи статус"},
		ImagesSourceDir:           "images\\sharex",
		ImagesToPick:              3,
		ImagesTTLSeconds:          60,
		HistoryHeader:             "история предыдущих ответов AI:",
		SpeechHeader:              "Моя реплика",
		MaxHistoryRecords:         10,
		ScreenshotIntervalSeconds: 2,
		ScreenshotEnabled:         true,
		// Таймер по умолчанию
		RotateConversationEach: 3,
		TimerIntervalSeconds:   10,
		TickTimeoutSeconds:     60,
		OverlapPolicy:          "skip", //`skip`|`preempt`
		MaxConsecutiveErrors:   3,
		NotificationSoundPath:  "sound/notification2.mp3",
		// STT/Speech
		STTHandyWindow:  time.Second,
		STTHotkeyDelay:  100 * time.Millisecond,
		SpeechMax:       10,
		EnableEarlyTick: true,
		// Chat/Twitch
		ChatHistoryHeader: "Сообщения из чата",
		ChatMax:           30,
		// По умолчанию используем Google TTS
		TTSService: "google",
		YandexTTS: YandexTTSConfig{
			APIKey:  "", // ключ берём из .env/ENV, если пусто — будет ошибка при использовании
			Voice:   "omazh",
			Format:  "mp3",  // поддерживаемые форматы: mp3|wav|oggopus (проигрывание mp3/wav)
			Speed:   "1.3",  // ускорение речи ~30% относительно дефолта 1.0
			Emotion: "evil", // эмоциональная окраска по умолчанию
			Volume:  100,    // 0-100, 100 — громкость не меняем
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
			InputType:        "", // auto
		},
		// State
		StateHeader: "Состояние игры",
		StateMax:    3,
	}
}

// NewConfig загружает конфигурацию приложения.
func NewConfig() *Config {
	_ = godotenv.Load()

	// Стартуем с дефолтов, затем перекрываем .env/окружением и флагами
	cfg := Defaults()
	_ = env.Parse(cfg)

	flag.BoolVar(&cfg.DebugMode, "debug-mode", cfg.DebugMode, "включить режим дебага для отображения до инфы")
	flag.StringVar(&cfg.AssistantPrompt, "assistant-prompt", cfg.AssistantPrompt, "текст промпта ассистента диалога")
	flag.StringVar(&cfg.HistoryHeader, "history-header", cfg.HistoryHeader, "заголовок блока с историей предыдущих ответов AI")
	flag.IntVar(&cfg.MaxHistoryRecords, "max-history-records", cfg.MaxHistoryRecords, "максимум хранимых ответов ИИ в локальной истории")
	// Принимаем список характеров одной строкой, разделённой ';'
	var characterListFlag string
	characterListFlag = strings.Join(cfg.CharacterList, ";")
	flag.StringVar(&characterListFlag, "character-list", characterListFlag, "список характеров персонажа, разделённых ';'")
	// Принимаем список реплик-подсказок одной строкой, разделённой ';'
	var speechPromptFlag string
	speechPromptFlag = strings.Join(cfg.SpeechPrompt, ";")
	flag.StringVar(&speechPromptFlag, "speech-prompt", speechPromptFlag, "реплики-подсказки, разделённые ';' (одна будет выбрана случайно)")
	flag.StringVar(&cfg.ImagesSourceDir, "images-source-dir", cfg.ImagesSourceDir, "путь к папке с исходными изображениями")
	flag.IntVar(&cfg.ImagesToPick, "images-to-pick", cfg.ImagesToPick, "количество последних изображений для отправки")
	flag.IntVar(&cfg.ImagesTTLSeconds, "images-ttl-seconds", cfg.ImagesTTLSeconds, "время, через которое картинки считаются старыми и их надо удалить, в секундах")
	// Звук уведомления
	flag.StringVar(&cfg.NotificationSoundPath, "notification-sound-path", cfg.NotificationSoundPath, "путь к звуковому файлу уведомления (mp3 или wav)")
	// Скриншоттер
	flag.IntVar(&cfg.ScreenshotIntervalSeconds, "screenshot-interval-seconds", cfg.ScreenshotIntervalSeconds, "периодичность снятия скриншотов всего экрана, в секундах")
	flag.BoolVar(&cfg.ScreenshotEnabled, "screenshot-enabled", cfg.ScreenshotEnabled, "включить фоновую съёмку скриншотов (Screenshotter)")
	// Таймер
	flag.IntVar(&cfg.TimerIntervalSeconds, "timer-interval-seconds", cfg.TimerIntervalSeconds, "базовый интервал таймера в секундах")
	flag.IntVar(&cfg.TickTimeoutSeconds, "tick-timeout-seconds", cfg.TickTimeoutSeconds, "таймаут одного тика в секундах")
	flag.StringVar(&cfg.OverlapPolicy, "overlap-policy", cfg.OverlapPolicy, "политика наложения тиков: skip|preempt")
	flag.IntVar(&cfg.MaxConsecutiveErrors, "max-consecutive-errors", cfg.MaxConsecutiveErrors, "количество последовательных ошибок до остановки приложения")
	flag.IntVar(&cfg.RotateConversationEach, "rotate-conversation-each", cfg.RotateConversationEach, "каждые N успешных запросов начинать новый диалог")
	// Chat/Twitch
	flag.StringVar(&cfg.ChatHistoryHeader, "chat-history-header", cfg.ChatHistoryHeader, "заголовок блока с сообщениями чата")
	flag.IntVar(&cfg.ChatMax, "chat-max", cfg.ChatMax, "максимум хранимых сообщений чата")
	// Общие/переключатель TTS
	flag.StringVar(&cfg.TTSService, "tts-service", cfg.TTSService, "выбор сервиса TTS: yandex|google")
	flag.StringVar(&cfg.TwitchUsername, "twitch-username", cfg.TwitchUsername, "логин Twitch для подключения к чату")
	flag.StringVar(&cfg.TwitchOAuthToken, "twitch-oauth-token", cfg.TwitchOAuthToken, "OAuth токен Twitch (может быть без префикса oauth:)")
	flag.StringVar(&cfg.TwitchChannel, "twitch-channel", cfg.TwitchChannel, "канал Twitch (без #)")
	// Параметры Yandex TTS
	flag.StringVar(&cfg.YandexTTS.APIKey, "yc-tts-api-key", cfg.YandexTTS.APIKey, "API ключ Yandex SpeechKit TTS (перекрывает ENV)")
	flag.StringVar(&cfg.YandexTTS.Voice, "yc-tts-voice", cfg.YandexTTS.Voice, "голос для синтеза (напр. filipp, jane, oksana, zahar, ermil)")
	flag.StringVar(&cfg.YandexTTS.Format, "yc-tts-format", cfg.YandexTTS.Format, "формат аудио (mp3|wav|oggopus), проигрывание поддерживается для mp3 и wav")
	flag.StringVar(&cfg.YandexTTS.Speed, "yc-tts-speed", cfg.YandexTTS.Speed, "скорость речи (например, 1.0 по умолчанию; 1.3 = на 30% быстрее)")
	flag.StringVar(&cfg.YandexTTS.Emotion, "yc-tts-emotion", cfg.YandexTTS.Emotion, "эмоциональная окраска (neutral|good|evil). По умолчанию evil")
	flag.IntVar(&cfg.YandexTTS.Volume, "yc-tts-volume", cfg.YandexTTS.Volume, "громкость 0-100 (100 — без изменений)")
	// Параметры Google TTS
	flag.StringVar(&cfg.GoogleTTS.CredentialsPath, "google-tts-credentials", cfg.GoogleTTS.CredentialsPath, "путь к service-account.json (также читается из ENV GOOGLE_APPLICATION_CREDENTIALS)")
	flag.StringVar(&cfg.GoogleTTS.Language, "google-tts-language", cfg.GoogleTTS.Language, "язык синтеза, напр. ru-RU")
	flag.StringVar(&cfg.GoogleTTS.Voice, "google-tts-voice", cfg.GoogleTTS.Voice, "имя голоса, напр. ru-RU-Standard-A или ru-RU-Wavenet-A")
	flag.Float64Var(&cfg.GoogleTTS.SpeakingRate, "google-tts-speaking-rate", cfg.GoogleTTS.SpeakingRate, "скорость речи (1.0 по умолчанию)")
	flag.Float64Var(&cfg.GoogleTTS.Pitch, "google-tts-pitch", cfg.GoogleTTS.Pitch, "тон (полутоны), может быть отрицательным")
	flag.Float64Var(&cfg.GoogleTTS.VolumeGainDb, "google-tts-volume-db", cfg.GoogleTTS.VolumeGainDb, "усиление громкости (дБ), допустимо от -96.0 до +16.0")
	flag.StringVar(&cfg.GoogleTTS.EffectsProfileID, "google-tts-effects-profile-id", cfg.GoogleTTS.EffectsProfileID, "EffectsProfileId, напр. large-home-entertainment-class-device")
	flag.StringVar(&cfg.GoogleTTS.InputType, "google-tts-input-type", cfg.GoogleTTS.InputType, "тип входа: text|ssml; пусто = авто по наличию <speak>")
	// STT/Speech
	flag.DurationVar(&cfg.STTHandyWindow, "stt-handy-window", cfg.STTHandyWindow, "окно времени (Handy) для совпадения буфера и хоткея, напр. 1s")
	flag.DurationVar(&cfg.STTHotkeyDelay, "stt-hotkey-delay", cfg.STTHotkeyDelay, "задержка реакции на Ctrl+Enter перед фиксацией текста, напр. 100ms")
	flag.StringVar(&cfg.SpeechHeader, "speech-header", cfg.SpeechHeader, "заголовок блока сообщений из речи (Speech)")
	flag.IntVar(&cfg.SpeechMax, "speech-max", cfg.SpeechMax, "максимум хранимых сообщений в Speech")
	flag.BoolVar(&cfg.EnableEarlyTick, "enable-early-tick", cfg.EnableEarlyTick, "запускать ранний тик при наличии сообщений из Speech")

	// StateServer
	flag.BoolVar(&cfg.StateServer.Enabled, "state-server-enabled", cfg.StateServer.Enabled, "включить приёмник игрового состояния (StateServer)")
	flag.StringVar(&cfg.StateServer.BindAddr, "state-server-bind-addr", cfg.StateServer.BindAddr, "адрес для прослушивания StateServer (напр. 127.0.0.1:3000)")
	flag.StringVar(&cfg.StateServer.Path, "state-server-path", cfg.StateServer.Path, "HTTP путь StateServer (напр. /)")
	flag.StringVar(&cfg.StateServer.AuthToken, "state-server-auth-token", cfg.StateServer.AuthToken, "токен авторизации StateServer (опционально)")

	// State buffer
	flag.StringVar(&cfg.StateHeader, "state-header", cfg.StateHeader, "заголовок блока сообщений из игрового состояния (State)")
	flag.IntVar(&cfg.StateMax, "state-max", cfg.StateMax, "максимум хранимых сообщений в State")
	flag.Parse()

	// Разбор списков по общему правилу (trim + убрать пустые), дефолты различаются
	cfg.CharacterList = parseListFlag(characterListFlag, []string{""})
	cfg.SpeechPrompt = parseListFlag(speechPromptFlag, []string{"доложи статус"})

	// Валидация и подготовка окружения для Google TTS.
	// Если выбран сервис google, убеждаемся, что задан путь к cred-файлу
	// и он существует. Если ENV пуст, но в конфиге указан путь — устанавливаем ENV.
	if strings.EqualFold(cfg.TTSService, "google") {
		cred := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
		if cred == "" {
			if cp := strings.TrimSpace(cfg.GoogleTTS.CredentialsPath); cp != "" {
				_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", cp)
				cred = cp
			}
		}
		if cred == "" {
			panic(fmt.Errorf("google tts: переменная окружения GOOGLE_APPLICATION_CREDENTIALS не задана; укажите ENV или флаг -google-tts-credentials"))
		}
		if _, err := os.Stat(cred); err != nil {
			panic(fmt.Errorf("google tts: файл ключа не найден: %s", cred))
		}
	}

	return cfg
}

// parseListFlag разбирает значение флага со списком, разделённым ';'
func parseListFlag(v string, def []string) []string {
	// Пустая строка → дефолт
	if v == "" {
		return def
	}
	parts := strings.Split(v, ";")
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 0 {
		return def
	}
	return cleaned
}
