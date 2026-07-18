package handler

import (
	"github.com/labstack/echo/v5"

	"nutmeg/views/pages/home"
)

type HomeHandler struct{}

func (h *HomeHandler) Index(c *echo.Context) error {
	return page(c, "Dashboard - Soccer Stats", true, "", home.Dashboard())
}
