package handler

import (
	"net/http"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/views/pages/home"
)

type HomeHandler struct {
	groupSvc *service.GroupService
	matchSvc *service.MatchService
	auth     *ezauth.EzAuth
}

func (h *HomeHandler) Landing(c *echo.Context) error {
	_, err := ezauth.GetUser(c.Request().Context())
	if err == nil {
		return c.Redirect(http.StatusFound, "/dashboard")
	}
	return page(c, "Nutmeg - Self-Hosted Pickup Soccer Stats Tracker", false, "", "", home.Landing())
}

func (h *HomeHandler) Dashboard(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	groups, err := h.groupSvc.List(c.Request().Context(), userID)
	if err != nil {
		groups = nil
	}

	userName := h.getUserName(c)
	return page(c, "Dashboard", true, "", userName, home.Dashboard(groups))
}

func (h *HomeHandler) Stats(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	stats, err := h.matchSvc.GetPlayerStats(c.Request().Context(), userID)
	if err != nil {
		stats = &repository.PlayerStats{}
	}

	userName := h.getUserName(c)
	return page(c, "My Stats", true, "", userName, home.Stats(stats))
}

func (h *HomeHandler) getUserName(c *echo.Context) string {
	user, err := ezauth.GetUser(c.Request().Context())
	if err != nil {
		return ""
	}
	return user.DisplayName()
}
