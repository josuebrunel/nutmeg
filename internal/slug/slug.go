// Package slug generates human-readable, collision-safe identifiers for
// public URLs (group and player links) without a second ID-generation
// path — the DB's existing gen_random_uuid() remains the sole source of
// row identity; a slug is just a display-friendly alias for it.
package slug

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lowercases s, replaces runs of non-alphanumeric characters with a
// single hyphen, and trims leading/trailing hyphens.
func Slugify(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// New builds a slug from name plus the first 8 characters of id, so two
// rows with the same name never collide without needing a retry loop.
func New(name, id string) string {
	base := Slugify(name)
	suffix := id
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	if base == "" {
		return suffix
	}
	return base + "-" + suffix
}
