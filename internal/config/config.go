package config

import (
	"flag"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	DebugMode          bool   `env:"DEBUG_MODE"`           //Режим дебага
	StartPrompt        string `env:"START_PROMPT"`         //Текст стартового промпта диалога
	ImagesSourceDir    string `env:"IMAGES_SOURCE_DIR"`    // Папка с исходными изображениями
	ImagesProcessedDir string `env:"IMAGES_PROCESSED_DIR"` // Папка для сохранения обработанных изображений
	ImagesToPick       int    `env:"IMAGES_TO_PICK"`       // Сколько последних изображений брать
	ImagesTTLSeconds   int    `env:"IMAGES_TTL_SECONDS"`   // Время, через которое картинки считаются старыми и их надо удалить, в секундах
}

// Defaults возвращает конфигурацию с предустановленными значениями по умолчанию.
// Эти значения перекрываются .env, переменными окружения и флагами CLI.
func Defaults() *Config {
	return &Config{
		DebugMode:          false,
		StartPrompt:        "какой результат боя?",
		ImagesSourceDir:    "images\\sharex",
		ImagesProcessedDir: "images\\processed",
		ImagesToPick:       3,
		ImagesTTLSeconds:   60,
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
	flag.StringVar(&cfg.ImagesSourceDir, "images-source-dir", cfg.ImagesSourceDir, "путь к папке с исходными изображениями")
	flag.StringVar(&cfg.ImagesProcessedDir, "images-processed-dir", cfg.ImagesProcessedDir, "путь к папке для сохранения обработанных изображений")
	flag.IntVar(&cfg.ImagesToPick, "images-to-pick", cfg.ImagesToPick, "количество последних изображений для отправки")
	flag.IntVar(&cfg.ImagesTTLSeconds, "images-ttl-seconds", cfg.ImagesTTLSeconds, "время, через которое картинки считаются старыми и их надо удалить, в секундах")
	flag.Parse()

	return cfg
}
