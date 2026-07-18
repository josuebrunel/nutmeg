package render

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
)

func Component(c *echo.Context, cmp templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTML)
	return cmp.Render(c.Request().Context(), c.Response())
}

func ComponentStatus(c *echo.Context, status int, cmp templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTML)
	c.Response().WriteHeader(status)
	return cmp.Render(c.Request().Context(), c.Response())
}
