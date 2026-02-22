Коротко о клиенте Gemini TTS

- Назначение: отправляет запрос в Google Cloud Text-to-Speech v1beta1 (`text:synthesize`) и проигрывает MP3.
- Авторизация: только ADC (`GOOGLE_APPLICATION_CREDENTIALS` с JSON сервисного аккаунта) или метаданные GCP.
- Эндпоинт: по умолчанию https://texttospeech.googleapis.com/v1beta1/text:synthesize (переопределяется `GeminiTTS.Endpoint`).
- Аудио: всегда MP3; поле `audioContent` (base64) декодируется и проигрывается плеером.
- Prompt: поддерживается только здесь — `GeminiTTS.Prompt` → `input.prompt` (если непустой). У других TTS игнорируется.
- Вход:
  - `InputType=ssml` → `input.ssml`
  - `InputType=text|prompt|пусто` → `input.text`
- Параметры: `ModelName`, `Language`, `VoiceName`, `SpeakingRate`, `Pitch`, `VolumeGainDb`, `EffectsProfileID`.

ENV/конфиг

- `GEMINI_TTS_MODEL_NAME`, `GEMINI_TTS_LANGUAGE`, `GEMINI_TTS_VOICE_NAME`
- `GEMINI_TTS_SPEAKING_RATE`, `GEMINI_TTS_PITCH`, `GEMINI_TTS_VOLUME_DB`
- `GEMINI_TTS_EFFECTS_PROFILE_ID`, `GEMINI_TTS_INPUT_TYPE`, `GEMINI_TTS_ENDPOINT`
- `GEMINI_TTS_PROMPT` — опционально

Использование

- Основной сервис: `TTS_SERVICE=gemini`, задать параметры в `.env`/ENV, настроить ADC.
- Быстрая проверка: `go run ./cmd/gemeni-tts -text "Привет!"` (нужен `GOOGLE_APPLICATION_CREDENTIALS`).

Файлы

- Клиент: `internal/service/tts/gemini/client.go`
- CLI-пример: `cmd/gemeni-tts/main.go`