package config

type Config struct {
	Addr    string `env:"ADDR" default:":8080"`
	BaseURL string `env:"BASE_URL" default:"http://localhost:8080"`
	Debug   bool   `env:"DEBUG" default:"false"`
	Database
	Ollama
	Email
}

type Database struct {
	DSN string `env:"DB_DSN" required:"true"`
}

type Ollama struct {
	BaseURL string `env:"OLLAMA_BASE_URL" default:"http://localhost:11434"`
	Model   string `env:"OLLAMA_MODEL" default:"llama3.1:8b"`
}

// Email is deliberately generic SMTP, not a specific provider's API — an
// empty SMTPHost means email is unconfigured, and internal/email.Client
// no-ops rather than erroring, so the app runs fine without it (e.g. in
// dev/CI with no mail relay available).
type Email struct {
	SMTPHost string `env:"SMTP_HOST" default:""`
	SMTPPort string `env:"SMTP_PORT" default:"587"`
	Username string `env:"SMTP_USERNAME" default:""`
	Password string `env:"SMTP_PASSWORD" default:""`
	From     string `env:"EMAIL_FROM" default:"noreply@nutmeg.local"`
}
