package api

import (
	"strconv"
	"strings"
	"testing"

	"nutmeg/internal/assert"
)

// decodeTally reverses encodeTally's "playerID:team:count,..." format back
// into a plain count map, for asserting on encodeTally's output without
// depending on map iteration order or on the unexported parser in the
// service package.
func decodeTally(s string) map[string]int {
	out := make(map[string]int)
	if s == "" {
		return out
	}
	for _, part := range strings.Split(s, ",") {
		fields := strings.Split(part, ":")
		if len(fields) != 3 {
			continue
		}
		n, _ := strconv.Atoi(fields[2])
		out[fields[0]] = n
	}
	return out
}

func TestEncodeTally(t *testing.T) {
	teamA := []string{"p1", "p2"}
	teamB := []string{"p3"}

	t.Run("encodes counts per player with their team", func(t *testing.T) {
		got := encodeTally(map[string]int{"p1": 2, "p3": 1}, teamA, teamB)
		gotCounts := decodeTally(got)
		assert.Eq(t, gotCounts["p1"], 2)
		assert.Eq(t, gotCounts["p3"], 1)
	})

	t.Run("skips zero and negative counts", func(t *testing.T) {
		got := encodeTally(map[string]int{"p1": 0, "p2": -1}, teamA, teamB)
		assert.Eq(t, got, "")
	})

	t.Run("skips players not on either team", func(t *testing.T) {
		got := encodeTally(map[string]int{"unknown": 3}, teamA, teamB)
		assert.Eq(t, got, "")
	})

	t.Run("empty map yields empty string", func(t *testing.T) {
		assert.Eq(t, encodeTally(map[string]int{}, teamA, teamB), "")
	})
}
