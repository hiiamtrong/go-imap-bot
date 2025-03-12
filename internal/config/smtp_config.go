package config

type SMTPConfig struct {
	Host string
	Port int64
	User string
	Pass string
	From string
}
