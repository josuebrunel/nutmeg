package api

import (
	"net/http"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
)

type HomeHandler struct {
	auth     *ezauth.EzAuth
	groupSvc *service.GroupService
	matchSvc *service.MatchService
}

// Dashboard returns the caller's groups and global stats.
// @Summary Get dashboard data
// @Tags home
// @Produce json
// @Security BearerAuth
// @Success 200 {object} DashboardResponse
// @Failure 401 {object} ErrorResponse
// @Router /dashboard [get]
func (h *HomeHandler) Dashboard(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	groups, err := h.groupSvc.List(ctx, userID)
	if err != nil {
		groups = nil
	}
	stats, err := h.matchSvc.GlobalStats(ctx, userID)
	if err != nil {
		stats = &repository.GlobalStats{}
	}

	return c.JSON(http.StatusOK, DashboardResponse{Groups: toGroupResponses(groups), Stats: toGlobalStatsResponse(stats)})
}

// Stats returns the caller's global stats.
// @Summary Get global stats
// @Tags home
// @Produce json
// @Security BearerAuth
// @Success 200 {object} GlobalStatsResponse
// @Failure 401 {object} ErrorResponse
// @Router /stats [get]
func (h *HomeHandler) Stats(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	stats, err := h.matchSvc.GlobalStats(ctx, userID)
	if err != nil {
		stats = &repository.GlobalStats{}
	}
	return c.JSON(http.StatusOK, toGlobalStatsResponse(stats))
}
