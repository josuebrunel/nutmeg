package api

import (
	"context"
	"net/http"
	"time"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/handler"
	"nutmeg/internal/model"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/internal/worker"
)

type MatchHandler struct {
	auth    *ezauth.EzAuth
	service *service.MatchService
	repo    *repository.Repository
	jobs    handler.JobEnqueuer
}

func NewMatchHandler(auth *ezauth.EzAuth, svc *service.MatchService, repo *repository.Repository, jobs handler.JobEnqueuer) *MatchHandler {
	return &MatchHandler{auth: auth, service: svc, repo: repo, jobs: jobs}
}

// enqueueCommentary fires off async roast-commentary generation for every
// player in a just-logged match — best-effort, failures are swallowed since
// this must never block or fail the match save that already succeeded.
func (h *MatchHandler) enqueueCommentary(ctx context.Context, matchID string, playerIDs []string) {
	for _, playerID := range playerIDs {
		_, _ = h.jobs.Insert(ctx, worker.GenerateCommentaryArgs{GroupPlayerID: playerID, MatchID: &matchID}, nil)
	}
}

// List returns a group's match history.
// @Summary List matches
// @Tags matches
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Success 200 {array} MatchResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/matches [get]
func (h *MatchHandler) List(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")

	if err := h.service.AuthorizeGroupAccess(ctx, groupID, userID); err != nil {
		return writeError(c, err)
	}
	matches, err := h.service.ListByGroup(ctx, groupID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toMatchResponses(matches))
}

// Create logs a new match.
// @Summary Log a match
// @Tags matches
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param request body MatchWriteRequest true "Match"
// @Success 201 {object} MatchDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /groups/{id}/matches [post]
func (h *MatchHandler) Create(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")

	if err := h.service.AuthorizeGroupAccess(ctx, groupID, userID); err != nil {
		return writeError(c, err)
	}

	var req MatchWriteRequest
	if err := c.Bind(&req); err != nil {
		return writeError(c, model.ErrInvalidInput)
	}
	playedAt := time.Now()
	if req.PlayedAt != nil {
		playedAt = *req.PlayedAt
	}

	input := service.CreateMatchInput{
		GroupID:      groupID,
		TeamAName:    req.TeamAName,
		TeamBName:    req.TeamBName,
		ScoreA:       req.ScoreA,
		ScoreB:       req.ScoreB,
		CreatedBy:    userID,
		TeamAPlayers: req.TeamAPlayers,
		TeamBPlayers: req.TeamBPlayers,
		GoalsInput:   encodeTally(req.Goals, req.TeamAPlayers, req.TeamBPlayers),
		AssistsInput: encodeTally(req.Assists, req.TeamAPlayers, req.TeamBPlayers),
		PlayedAt:     playedAt,
	}

	matchID, err := h.service.Create(ctx, input)
	if err != nil {
		return badRequest(c, err)
	}
	h.enqueueCommentary(ctx, matchID, append(append([]string{}, req.TeamAPlayers...), req.TeamBPlayers...))
	handler.RecordNews(ctx, h.repo, h.jobs, groupID, "match_logged", matchID, "Full match report coming soon.")

	editable, err := h.service.GetEditable(ctx, matchID, userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusCreated, toMatchDetailResponse(editable))
}

// Get returns a single match's full per-player detail.
// @Summary Get a match
// @Tags matches
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param mid path string true "Match ID"
// @Success 200 {object} MatchDetailResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/matches/{mid} [get]
func (h *MatchHandler) Get(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	editable, err := h.service.GetEditable(ctx, c.Param("mid"), userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toMatchDetailResponse(editable))
}

// Update edits an existing match.
// @Summary Update a match
// @Tags matches
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param mid path string true "Match ID"
// @Param request body MatchWriteRequest true "Match"
// @Success 200 {object} MatchDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/matches/{mid} [patch]
func (h *MatchHandler) Update(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	matchID := c.Param("mid")

	var req MatchWriteRequest
	if err := c.Bind(&req); err != nil {
		return writeError(c, model.ErrInvalidInput)
	}
	playedAt := time.Now()
	if req.PlayedAt != nil {
		playedAt = *req.PlayedAt
	}

	input := service.UpdateMatchInput{
		MatchID:      matchID,
		UserID:       userID,
		TeamAName:    req.TeamAName,
		TeamBName:    req.TeamBName,
		ScoreA:       req.ScoreA,
		ScoreB:       req.ScoreB,
		TeamAPlayers: req.TeamAPlayers,
		TeamBPlayers: req.TeamBPlayers,
		GoalsInput:   encodeTally(req.Goals, req.TeamAPlayers, req.TeamBPlayers),
		AssistsInput: encodeTally(req.Assists, req.TeamAPlayers, req.TeamBPlayers),
		PlayedAt:     playedAt,
	}
	if err := h.service.Update(ctx, input); err != nil {
		if err == model.ErrNotFound || err == model.ErrNotAuthorized {
			return writeError(c, err)
		}
		return badRequest(c, err)
	}

	editable, err := h.service.GetEditable(ctx, matchID, userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toMatchDetailResponse(editable))
}

// Delete removes a match.
// @Summary Delete a match
// @Tags matches
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param mid path string true "Match ID"
// @Success 204 "no content"
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/matches/{mid} [delete]
func (h *MatchHandler) Delete(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	if err := h.service.Delete(ctx, c.Param("mid"), userID); err != nil {
		return writeError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
