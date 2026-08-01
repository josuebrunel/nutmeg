package middleware

import (
	"time"

	"github.com/labstack/echo/v5"
)

const contextKeyLocation = "location"

// Location reads the "tz" cookie (set client-side from
// Intl.DateTimeFormat().resolvedOptions().timeZone) and stores the
// resolved *time.Location on the request context, defaulting to UTC when
// the cookie is missing or names an unrecognized zone.
func Location(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		loc := time.UTC
		if cookie, err := c.Request().Cookie("tz"); err == nil && cookie.Value != "" {
			if l, err := time.LoadLocation(cookie.Value); err == nil {
				loc = l
			}
		}
		c.Set(contextKeyLocation, loc)
		return next(c)
	}
}

// LocationFromContext returns the *time.Location resolved by Location,
// defaulting to UTC if the middleware didn't run.
func LocationFromContext(c *echo.Context) *time.Location {
	if loc, ok := c.Get(contextKeyLocation).(*time.Location); ok {
		return loc
	}
	return time.UTC
}
