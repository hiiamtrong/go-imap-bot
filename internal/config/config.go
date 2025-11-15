package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	MailConfig     *MailConfig
	DatabaseConfig *DatabaseConfig
	TelegramBot    *TelegramBotConfig
	SMTP           *SMTPConfig
	VietQR         *VietQRConfig
	AWS            *AWSConfig
	OAuth          *OAuthConfig
}

func NewConfig() *Config {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// Try to read config file, but don't panic if it doesn't exist
	// Environment variables will be used instead
	_ = viper.ReadInConfig()

	// Validate required fields
	required := []string{
		"MAIL_SERVER",
		"MAIL_USERNAME",
		"MAIL_PASSWORD",
		"MAIL_MAILBOX",
		"MAIL_HOST",
		"MAIL_PORT",
		"MAIL_FROM",

		"DB_PATH",
		"TELEGRAM_BOT_TOKEN",
		"TELEGRAM_BOT_API_ENDPOINT",

		"VIETQR_CLIENT_ID",
		"VIETQR_API_KEY",
		"VIETQR_BANK_ID",
		"VIETQR_ACCOUNT_NO",
		"VIETQR_TEMPLATE",
		"VIETQR_ACCOUNT_NAME",

		"AWS_PROFILE",
		"AWS_REGION",
		"AWS_S3_BUCKET",
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
			Token:       viper.GetString("TELEGRAM_BOT_TOKEN"),
			APIEndpoint: viper.GetString("TELEGRAM_BOT_API_ENDPOINT"),
		},
		SMTP: &SMTPConfig{
			Host: viper.GetString("MAIL_HOST"),
			Port: viper.GetInt64("MAIL_PORT"),
			From: viper.GetString("MAIL_FROM"),
			User: viper.GetString("MAIL_USERNAME"),
			Pass: viper.GetString("MAIL_PASSWORD"),
		},
		VietQR: &VietQRConfig{
			ClientID:    viper.GetString("VIETQR_CLIENT_ID"),
			APIKey:      viper.GetString("VIETQR_API_KEY"),
			BankID:      viper.GetInt64("VIETQR_BANK_ID"),
			AccountNo:   viper.GetString("VIETQR_ACCOUNT_NO"),
			AccountName: viper.GetString("VIETQR_ACCOUNT_NAME"),
			Template:    viper.GetString("VIETQR_TEMPLATE"),
		},
		AWS: &AWSConfig{
			Profile:  viper.GetString("AWS_PROFILE"),
			Region:   viper.GetString("AWS_REGION"),
			S3Bucket: viper.GetString("AWS_S3_BUCKET"),
		},
		OAuth: &OAuthConfig{
			GoogleClientID:     viper.GetString("GOOGLE_CLIENT_ID"),
			GoogleClientSecret: viper.GetString("GOOGLE_CLIENT_SECRET"),
			GoogleRedirectURL:  viper.GetString("GOOGLE_REDIRECT_URL"),
			JWTSecret:          viper.GetString("JWT_SECRET"),
		},
	}
}
