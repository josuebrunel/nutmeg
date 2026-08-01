package config

type Config struct {
	Addr    string `env:"ADDR" default:":8080"`
	BaseURL string `env:"BASE_URL" default:"http://localhost:8080"`
	Debug   bool   `env:"DEBUG" default:"false"`
	Database
	Ollama
}

type Database struct {
	DSN string `env:"DB_DSN" required:"true"`
}

type Ollama struct {
	BaseURL string `env:"OLLAMA_BASE_URL" default:"http://localhost:11434"`
	Model   string `env:"OLLAMA_MODEL" default:"llama3.1:8b"`
}
