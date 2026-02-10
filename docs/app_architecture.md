# Архитектура текущего приложения (заглушка)

Приоритет: слоёная архитектура. Детализация появится по мере развития.

## Слои (предложение)
- cmd/*: входные точки (CLI/сервисы).
- app/usecase: прикладная логика/оркестрация сценариев.
- domain: модели и интерфейсы (LLM, TTS, Storage).
- adapters: реализации интерфейсов (OpenAI, файловое/памятное хранилище, TTS).
- infra: конфигурация, логирование, метрики.
- internal/pkg: общие утилиты.

## Контракты (эскиз)
- type LLMClient interface { Generate(ctx, prompt, images) (text string, err error) }
- type ScreenShotStore interface { Put(img []byte) error; GetLast(n int) ([][]byte, error) }
- type TTS interface { Speak(ctx context.Context, text string) error }

## Следующие шаги
- Выделить интерфейсы из `cmd/vision` и `main.go`.
- Ввести пакет домена и адаптер OpenAI.
- Добавить конфигурацию через env и логирование.
