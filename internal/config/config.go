package config

type Config struct {
	Addr    string `env:"ADDR" default:":8080"`
	BaseURL string `env:"BASE_URL" default:"http://localhost:8080"`
	Debug   bool   `env:"DEBUG" default:"false"`
	Database
	LLM
	Email
}

type Database struct {
	DSN string `env:"DB_DSN" required:"true"`
}

// LLM is one standardized set of settings shared by every LLMGenerator
// implementation (internal/llm), regardless of provider — only Provider
// needs to change to switch; BaseURL/APIKey/Model are reinterpreted per
// provider (e.g. BaseURL is Ollama's server address and unused by Google;
// APIKey is unused by Ollama, which needs no auth). Model must match
// whatever the selected provider actually expects (an Ollama tag like
// "llama3.1:8b", or a Google model id — check
// https://ai.google.dev/gemini-api/docs/models before relying on the
// default below).
type LLM struct {
	// Provider picks which LLMGenerator implementation cmd/server wires
	// up — "ollama" (default, local/self-hosted) or "google" (Google's
	// Generative Language API, e.g. to run Gemma directly).
	Provider string `env:"LLM_PROVIDER" default:"ollama"`
	BaseURL  string `env:"LLM_BASE_URL" default:"http://localhost:11434"`
	APIKey   string `env:"LLM_API_KEY" default:""`
	Model    string `env:"LLM_MODEL" default:"llama3.1:8b"`
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
