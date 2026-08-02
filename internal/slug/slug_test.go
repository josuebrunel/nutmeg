package slug

import (
	"testing"

	"nutmeg/internal/assert"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "Sunday Pickup", "sunday-pickup"},
		{"punctuation", "Chris's FC!!", "chris-s-fc"},
		{"leading/trailing junk", "  --Weekend League--  ", "weekend-league"},
		{"already lowercase", "leo", "leo"},
		{"unicode-ish symbols", "Team #1 (East)", "team-1-east"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Eq(t, Slugify(tc.in), tc.want)
		})
	}
}

func TestNew(t *testing.T) {
	t.Run("appends 8-char id suffix", func(t *testing.T) {
		got := New("Sunday Pickup", "285b2d35-1c2c-4e2b-b1e9-cbf069300064")
		assert.Eq(t, got, "sunday-pickup-285b2d35")
	})

	t.Run("empty name falls back to just the suffix", func(t *testing.T) {
		got := New("!!!", "285b2d35-1c2c-4e2b-b1e9-cbf069300064")
		assert.Eq(t, got, "285b2d35")
	})

	t.Run("short id used as-is", func(t *testing.T) {
		got := New("Leo", "abc123")
		assert.Eq(t, got, "leo-abc123")
	})
}
