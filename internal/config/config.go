package config

import (
	"flag"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	DebugMode bool `env:"DEBUG_MODE"` //Режим дебага
}

func NewConfig() *Config {
	_ = godotenv.Load()

	cfg := &Config{}
	_ = env.Parse(cfg) //

	flag.BoolVar(&cfg.DebugMode, "debug-mode", cfg.DebugMode, "включить режим дебага для отображения до инфы")
	flag.Parse()

	return cfg
}
