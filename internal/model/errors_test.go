package model

import (
	"errors"
	"testing"

	"nutmeg/internal/assert"
)

func TestSentinelErrors(t *testing.T) {
	t.Run("ErrNotFound", func(t *testing.T) {
		assert.Eq(t, ErrNotFound.Error(), "not found")
		assert.True(t, errors.Is(ErrNotFound, ErrNotFound))
	})

	t.Run("ErrNotAuthorized", func(t *testing.T) {
		assert.Eq(t, ErrNotAuthorized.Error(), "not authorized")
		assert.True(t, errors.Is(ErrNotAuthorized, ErrNotAuthorized))
	})

	t.Run("ErrAlreadyExists", func(t *testing.T) {
		assert.Eq(t, ErrAlreadyExists.Error(), "already exists")
		assert.True(t, errors.Is(ErrAlreadyExists, ErrAlreadyExists))
	})

	t.Run("ErrInvalidInput", func(t *testing.T) {
		assert.Eq(t, ErrInvalidInput.Error(), "invalid input")
		assert.True(t, errors.Is(ErrInvalidInput, ErrInvalidInput))
	})

	t.Run("errorsAreDistinct", func(t *testing.T) {
		assert.False(t, errors.Is(ErrNotFound, ErrNotAuthorized))
		assert.False(t, errors.Is(ErrAlreadyExists, ErrInvalidInput))
	})
}
