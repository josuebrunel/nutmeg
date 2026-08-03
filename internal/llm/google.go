package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	// Thought marks this part as the model's internal reasoning trace
	// rather than its actual answer — "thinking" models like Gemma 4
	// return the thought part(s) first, then the real response last, all
	// inside the same Parts slice. Must be skipped when extracting output.
	Thought bool `json:"thought,omitempty"`
}

type googleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []googlePart `json:"parts"`
}

type googleRequest struct {
	Contents []googleContent `json:"contents"`
}

type googleResponse struct {
	Candidates []struct {
		Content      googleContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Generate sends prompt as a single content part to the model's
// generateContent endpoint and returns the model's response text.
func (c *GoogleClient) Generate(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(googleRequest{
		Contents: []googleContent{{Role: "user", Parts: []googlePart{{Text: prompt}}}},
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

	return parseGoogleResponse(resp.StatusCode, b)
}

// parseGoogleResponse extracts the model's actual answer from a
// generateContent response body, regardless of which model produced it.
// Deliberately doesn't assume anything about *whether* a given model
// "thinks" — some do (returning thought:true parts ahead of the real
// answer, e.g. gemma-4-31b-it) and some don't (a single plain part, e.g.
// Gemma 3) — so every non-thought part with text is treated as real output
// and joined, which is correct either way instead of hardcoding per-model
// behavior.
func parseGoogleResponse(statusCode int, body []byte) (string, error) {
	var out googleResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}

	if statusCode != http.StatusOK {
		if out.Error != nil {
			return "", fmt.Errorf("llm: google returned %d: %s", statusCode, out.Error.Message)
		}
		return "", fmt.Errorf("llm: google returned %d: %s", statusCode, string(body))
	}
	if len(out.Candidates) == 0 {
		return "", fmt.Errorf("llm: google returned no candidates")
	}
	candidate := out.Candidates[0]

	var answer strings.Builder
	for _, part := range candidate.Content.Parts {
		if !part.Thought && part.Text != "" {
			answer.WriteString(part.Text)
		}
	}
	if answer.Len() > 0 {
		return answer.String(), nil
	}

	if candidate.FinishReason != "" && candidate.FinishReason != "STOP" {
		return "", fmt.Errorf("llm: google returned no usable text, finishReason: %s", candidate.FinishReason)
	}
	return "", fmt.Errorf("llm: google returned only thought content, no final answer")
}

// Model returns the model name this client is configured to use, for
// recording alongside generated content (player_commentary.model).
func (c *GoogleClient) Model() string {
	return c.model
}
