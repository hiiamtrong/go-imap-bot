package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	MailConfig     *MailConfig
	DatabaseConfig *DatabaseConfig
	TelegramBot    *TelegramBotConfig
}

func NewConfig() *Config {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			panic(fmt.Errorf("fatal error reading config file: %w", err))
		}
		// Config file not found; ignore error if desired
	}

	// Validate required fields
	required := []string{
		"MAIL_SERVER",
		"MAIL_USERNAME",
		"MAIL_PASSWORD",
		"MAIL_MAILBOX",

		"DB_PATH",
		"TELEGRAM_BOT_TOKEN",
		"TELEGRAM_BOT_CHAT_ID",
	}

	for _, r := range required {
		if !viper.IsSet(r) || viper.GetString(r) == "" {
			panic(fmt.Sprintf("required config field %s is not set", r))
		}
	}

	return &Config{
		MailConfig: &MailConfig{
			Server:   viper.GetString("MAIL_SERVER"),
			Username: viper.GetString("MAIL_USERNAME"),
			Password: viper.GetString("MAIL_PASSWORD"),
			Mailbox:  viper.GetString("MAIL_MAILBOX"),
		},
		DatabaseConfig: &DatabaseConfig{
			DatabasePath: viper.GetString("DB_PATH"),
		},
		TelegramBot: &TelegramBotConfig{
			Token:  viper.GetString("TELEGRAM_BOT_TOKEN"),
			ChatID: int64(viper.GetInt("TELEGRAM_BOT_CHAT_ID")),
		},
	}
}
