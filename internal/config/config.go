package config

type Config struct {
	Addr    string `env:"ADDR" default:":8080"`
	BaseURL string `env:"BASE_URL" default:"http://localhost:8080"`
	Debug   bool   `env:"DEBUG" default:"false"`
	Database
}

type Database struct {
	DSN string `env:"DB_DSN" required:"true"`
}
