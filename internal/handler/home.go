package handler

import (
	"log/slog"
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
		return c.Redirect(http.StatusFound, routeDashboard)
	}
	return page(c, "Nutmeg - Self-Hosted Pickup Soccer Stats Tracker", false, "", "", home.Landing())
}

func (h *HomeHandler) Help(c *echo.Context) error {
	isLoggedIn := false
	userName := ""
	if user, err := ezauth.GetUser(c.Request().Context()); err == nil {
		isLoggedIn = true
		userName = user.DisplayName()
	}
	return page(c, "How Nutmeg Works", isLoggedIn, "", userName, home.Help(isLoggedIn))
}

func (h *HomeHandler) Dashboard(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	groups, err := h.groupSvc.List(c.Request().Context(), userID)
	if err != nil {
		groups = nil
	}

	globalStats, statsErr := h.matchSvc.GlobalStats(c.Request().Context(), userID)
	if statsErr != nil {
		slog.Error("failed to get global stats", "user_id", userID, "error", statsErr)
		globalStats = &repository.GlobalStats{}
	}

	userName := h.getUserName(c)
	return page(c, "Dashboard", true, "", userName, home.Dashboard(groups, globalStats))
}

func (h *HomeHandler) getUserName(c *echo.Context) string {
	user, err := ezauth.GetUser(c.Request().Context())
	if err != nil {
		return ""
	}
	return user.DisplayName()
}
