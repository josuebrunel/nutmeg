package router

import (
	"database/sql"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"
)

func Register(e *echo.Echo, auth *ezauth.EzAuth, db *sql.DB) {
	e.GET("/", func(c *echo.Context) error {
		return c.Redirect(302, "/groups")
	})
}
