package service

import (
	"errors"
	"strings"
)

var (
	errEmptyGeneration   = errors.New("empty generation")
	errBlockedGeneration = errors.New("generation matched the safety blocklist")
)

// generationBlocklist is a coarse, illustrative safety net, not a real
// moderation solution — both the player profile and group activity feed
// are public and unauthenticated, and a local model has no built-in
// moderation of its own, so this is a last-resort backstop against the
// worst outputs rather than a substitute for actually reviewing model
// behavior.
var generationBlocklist = []string{
	"kill yourself", "kys", "slur",
}

// validateGeneratedText is the gate between a raw model response and
// anything ever stored or shown: reject empty output, reject anything
// hitting the (coarse) blocklist, and trim overly long output at a
// sentence boundary rather than mid-word. Shared by player commentary and
// group activity news so the safety blocklist can't silently drift
// between the two generation paths.
func validateGeneratedText(text string, max int) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errEmptyGeneration
	}

	lower := strings.ToLower(text)
	for _, word := range generationBlocklist {
		if strings.Contains(lower, word) {
			return "", errBlockedGeneration
		}
	}

	if len(text) > max {
		text = truncateAtSentence(text, max)
	}

	return text, nil
}

// truncateAtSentence cuts text to at most max bytes, preferring to end on
// a sentence boundary (., !, ?) so trimmed output still reads cleanly.
func truncateAtSentence(text string, max int) string {
	if len(text) <= max {
		return text
	}
	cut := text[:max]
	if i := strings.LastIndexAny(cut, ".!?"); i > 0 {
		return strings.TrimSpace(cut[:i+1])
	}
	return strings.TrimSpace(cut) + "…"
}
