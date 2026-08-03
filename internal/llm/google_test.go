package llm

import (
	"net/http"
	"testing"

	"nutmeg/internal/assert"
)

func TestParseGoogleResponse(t *testing.T) {
	t.Run("non-thinking model, single part", func(t *testing.T) {
		// Shape returned by models with no reasoning trace, e.g. Gemma 3.
		body := `{
			"candidates": [
				{
					"content": {"parts": [{"text": "Ricardo is having a moment."}], "role": "model"},
					"finishReason": "STOP"
				}
			]
		}`
		got, err := parseGoogleResponse(http.StatusOK, []byte(body))
		assert.NoErr(t, err)
		assert.Eq(t, got, "Ricardo is having a moment.")
	})

	t.Run("thinking model, thought then answer", func(t *testing.T) {
		// The real gemma-4-31b-it payload shape captured while debugging
		// the original bug: a thought:true part first, then the answer.
		body := `{
			"candidates": [
				{
					"content": {
						"parts": [
							{"text": "*   Persona: savage commentator.\n*   Draft roast...", "thought": true},
							{"text": "Enjoy the top spot while it lasts, one-hit wonder."}
						],
						"role": "model"
					},
					"finishReason": "STOP"
				}
			]
		}`
		got, err := parseGoogleResponse(http.StatusOK, []byte(body))
		assert.NoErr(t, err)
		assert.Eq(t, got, "Enjoy the top spot while it lasts, one-hit wonder.")
	})

	t.Run("thinking model, multiple non-thought parts are joined", func(t *testing.T) {
		body := `{
			"candidates": [
				{
					"content": {
						"parts": [
							{"text": "reasoning...", "thought": true},
							{"text": "Part one. "},
							{"text": "Part two."}
						],
						"role": "model"
					},
					"finishReason": "STOP"
				}
			]
		}`
		got, err := parseGoogleResponse(http.StatusOK, []byte(body))
		assert.NoErr(t, err)
		assert.Eq(t, got, "Part one. Part two.")
	})

	t.Run("all parts are thought", func(t *testing.T) {
		body := `{
			"candidates": [
				{
					"content": {"parts": [{"text": "just reasoning, no answer", "thought": true}], "role": "model"},
					"finishReason": "STOP"
				}
			]
		}`
		_, err := parseGoogleResponse(http.StatusOK, []byte(body))
		assert.NotNil(t, err)
		assert.StrContains(t, err.Error(), "only thought content")
	})

	t.Run("blocked response surfaces finishReason", func(t *testing.T) {
		body := `{
			"candidates": [
				{
					"content": {"parts": [], "role": "model"},
					"finishReason": "SAFETY"
				}
			]
		}`
		_, err := parseGoogleResponse(http.StatusOK, []byte(body))
		assert.NotNil(t, err)
		assert.StrContains(t, err.Error(), "SAFETY")
	})

	t.Run("HTTP error status with message", func(t *testing.T) {
		body := `{"error": {"message": "model not found"}}`
		_, err := parseGoogleResponse(http.StatusNotFound, []byte(body))
		assert.NotNil(t, err)
		assert.StrContains(t, err.Error(), "model not found")
	})

	t.Run("empty candidates", func(t *testing.T) {
		body := `{"candidates": []}`
		_, err := parseGoogleResponse(http.StatusOK, []byte(body))
		assert.NotNil(t, err)
		assert.StrContains(t, err.Error(), "no candidates")
	})
}
