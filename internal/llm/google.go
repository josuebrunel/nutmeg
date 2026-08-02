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

// googleAPIBaseURL is Google's Generative Language API — hosts both
// Gemini and open Gemma models behind the same generateContent shape.
const googleAPIBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// GoogleClient is a minimal client for Google's Generative Language API
// (used to run e.g. Gemma directly rather than through a third-party
// router) — kept just as small as the Ollama client (stdlib net/http, no
// SDK), satisfying the same Generate/Model shape so any of these clients
// can be used interchangeably behind service.LLMGenerator.
type GoogleClient struct {
	apiKey string
	model  string
	http   *http.Client
}

// NewGoogleClient returns a Google Generative Language API client.
// requestTimeout bounds a single generation call.
func NewGoogleClient(apiKey, model string, requestTimeout time.Duration) *GoogleClient {
	return &GoogleClient{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: requestTimeout},
	}
}

type googlePart struct {
	Text string `json:"text"`
}

type googleContent struct {
	Parts []googlePart `json:"parts"`
}

type googleRequest struct {
	Contents []googleContent `json:"contents"`
}

type googleResponse struct {
	Candidates []struct {
		Content googleContent `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Generate sends prompt as a single content part to the model's
// generateContent endpoint and returns the model's response text.
func (c *GoogleClient) Generate(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(googleRequest{
		Contents: []googleContent{{Parts: []googlePart{{Text: prompt}}}},
	})
	if err != nil {
		return "", fmt.Errorf("llm: marshal request: %w", err)
	}

	url := googleAPIBaseURL + "/" + c.model + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llm: read response: %w", err)
	}

	var out googleResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if out.Error != nil {
			return "", fmt.Errorf("llm: google returned %d: %s", resp.StatusCode, out.Error.Message)
		}
		return "", fmt.Errorf("llm: google returned %d: %s", resp.StatusCode, string(b))
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("llm: google returned no candidates")
	}

	return out.Candidates[0].Content.Parts[0].Text, nil
}

// Model returns the model name this client is configured to use, for
// recording alongside generated content (player_commentary.model).
func (c *GoogleClient) Model() string {
	return c.model
}
