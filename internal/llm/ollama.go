// Package llm provides a minimal client for generating text with a local
// Ollama instance — the app's only outbound third-party network call, kept
// deliberately small (stdlib net/http, no SDK) since Ollama's REST API is
// tiny.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// NewClient returns an Ollama client. requestTimeout bounds a single
// generation call — local inference on modest hardware can be slow, so this
// should be generous (e.g. 60s+), not a typical HTTP-call timeout.
func NewClient(baseURL, model string, requestTimeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{Timeout: requestTimeout},
	}
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate sends prompt to Ollama's /api/generate and returns the model's
// full (non-streamed) response text.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(generateRequest{Model: c.model, Prompt: prompt, Stream: false})
	if err != nil {
		return "", fmt.Errorf("llm: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm: ollama returned %d: %s", resp.StatusCode, string(b))
	}

	var out generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}

	return out.Response, nil
}

// Model returns the model name this client is configured to use, for
// recording alongside generated content (player_commentary.model).
func (c *Client) Model() string {
	return c.model
}
