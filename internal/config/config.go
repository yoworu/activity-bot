package config

import (
	"github.com/caarlos0/env/v11"
)

type Config struct {
	BotTokens         []string `env:"BOT_TOKENS"`
	DBDSN             string   `env:"DB_DSN"`
	Debug             bool     `env:"DEBUG" envDefault:"false"`
	DeveloperID       int64    `env:"DEVELOPER_ID"`
	DeveloperUsername string   `env:"DEVELOPER_USERNAME"`
	CommandsURL       string   `env:"COMMANDS_URL"`
	AppID             int      `env:"APP_ID"`
	AppHash           string   `env:"APP_HASH"`
	StoragePath       string   `env:"STORAGE_PATH" envDefault:""`
	DeepseekAPIKey    string   `env:"DEEPSEEK_API_KEY"`
	RedisADDR         string   `env:"REDIS_ADDR" envDefault:"redis:6379"`

	ApplicationBotToken string `env:"APPLICATION_BOT_TOKEN"`
	ApplicationChatID   int64  `env:"APPLICATION_CHAT_ID"`
	TargetChatID        int64  `env:"TARGET_CHAT_ID"`
	TargetChatLink      string `env:"TARGET_CHAT_LINK"`
	RolesPostLink       string `env:"ROLES_POST_LINK"`

	Application2BotToken string `env:"APPLICATION2_BOT_TOKEN"`
	Application2ChatID   int64  `env:"APPLICATION2_CHAT_ID"`
	Target2ChatID        int64  `env:"TARGET2_CHAT_ID"`
	Target2ChatLink      string `env:"TARGET2_CHAT_LINK"`
	Target2RolesPostLink string `env:"TARGET2_ROLES_POST_LINK" envDefault:"https://t.me/gummyfl00D/5"`

	WeatherAPIKey string `env:"WEATHER_API_KEY"`
}

func Load() (Config, error) {
	cfg := Config{}

	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
