package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/render"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/views/pages/matches"
)

type MatchHandler struct {
	auth    *ezauth.EzAuth
	service *service.MatchService
	repo    *repository.Repository
}

func NewMatchHandler(auth *ezauth.EzAuth, svc *service.MatchService, repo *repository.Repository) *MatchHandler {
	return &MatchHandler{auth: auth, service: svc, repo: repo}
}

func (h *MatchHandler) LogMatchModal(c *echo.Context) error {
	id := c.Param("id")
	members, err := h.repo.ListMembers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return render.Component(c, matches.LogForm(id, members))
}

func (h *MatchHandler) Create(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	groupID := c.Param("id")
	scoreA, _ := strconv.Atoi(c.FormValue("score_a"))
	scoreB, _ := strconv.Atoi(c.FormValue("score_b"))

	var teamAPlayers, teamBPlayers []string
	for key, values := range c.Request().Form {
		if strings.HasPrefix(key, "team_") {
			pid := strings.TrimPrefix(key, "team_")
			for _, v := range values {
				if v == "a" {
					teamAPlayers = append(teamAPlayers, pid)
				} else if v == "b" {
					teamBPlayers = append(teamBPlayers, pid)
				}
			}
		}
	}

	input := service.CreateMatchInput{
		GroupID:      groupID,
		TeamAName:    "Shirts",
		TeamBName:    "Skins",
		ScoreA:       scoreA,
		ScoreB:       scoreB,
		CreatedBy:    userID,
		TeamAPlayers: teamAPlayers,
		TeamBPlayers: teamBPlayers,
	}

	if err := h.service.Create(c.Request().Context(), input); err != nil {
		h.auth.Handler.SetFlash(c.Request().Context(), "error", err.Error())
		return c.Redirect(http.StatusFound, "/groups/"+groupID)
	}

	h.auth.Handler.SetFlash(c.Request().Context(), "success", "Match logged!")
	return c.Redirect(http.StatusFound, "/groups/"+groupID)
}

func (h *MatchHandler) Delete(c *echo.Context) error {
	_, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	matchID := c.Param("mid")
	groupID := c.Param("id")

	if err := h.service.Delete(c.Request().Context(), matchID, ""); err != nil {
		return c.Redirect(http.StatusFound, "/groups/"+groupID)
	}

	return c.Redirect(http.StatusFound, "/groups/"+groupID)
}
