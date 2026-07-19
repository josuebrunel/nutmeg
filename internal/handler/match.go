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
	return render.Component(c, matches.LogForm(id, members, nil))
}

func (h *MatchHandler) parseGoalsFromForm(c *echo.Context) string {
	var parts []string
	for key, values := range c.Request().Form {
		if strings.HasPrefix(key, "goals_") {
			pid := strings.TrimPrefix(key, "goals_")
			if len(values) > 0 {
				count := values[0]
				n, err := strconv.Atoi(count)
				if err != nil || n <= 0 {
					continue
				}
				// Determine team from team_* field
				team := c.FormValue("team_" + pid)
				if team == "" {
					continue
				}
				parts = append(parts, pid+":"+team+":"+count)
			}
		}
	}
	return strings.Join(parts, ",")
}

func (h *MatchHandler) parseTeamPlayers(c *echo.Context) ([]string, []string) {
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
	return teamAPlayers, teamBPlayers
}

func (h *MatchHandler) htmxRedirect(c *echo.Context, groupID string) error {
	c.Response().Header().Set("HX-Redirect", "/groups/"+groupID)
	return c.NoContent(http.StatusOK)
}

func (h *MatchHandler) Create(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	groupID := c.Param("id")
	scoreA, _ := strconv.Atoi(c.FormValue("score_a"))
	scoreB, _ := strconv.Atoi(c.FormValue("score_b"))

	teamAPlayers, teamBPlayers := h.parseTeamPlayers(c)
	goalsInput := h.parseGoalsFromForm(c)

	teamAName := c.FormValue("team_a_name")
	teamBName := c.FormValue("team_b_name")
	if teamAName == "" {
		teamAName = "Shirts"
	}
	if teamBName == "" {
		teamBName = "Skins"
	}

	input := service.CreateMatchInput{
		GroupID:      groupID,
		TeamAName:    teamAName,
		TeamBName:    teamBName,
		ScoreA:       scoreA,
		ScoreB:       scoreB,
		CreatedBy:    userID,
		TeamAPlayers: teamAPlayers,
		TeamBPlayers: teamBPlayers,
		GoalsInput:   goalsInput,
	}

	if err := h.service.Create(c.Request().Context(), input); err != nil {
		if isHTMX(c) {
			c.Response().Header().Set("HX-Trigger", `{"showToast":{"message":"`+err.Error()+`","type":"error"}}`)
			return c.NoContent(http.StatusOK)
		}
		h.auth.Handler.SetFlash(c.Request().Context(), "error", err.Error())
		return c.Redirect(http.StatusFound, "/groups/"+groupID)
	}

	if isHTMX(c) {
		return h.htmxRedirect(c, groupID)
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
		if isHTMX(c) {
			c.Response().Header().Set("HX-Trigger", `{"showToast":{"message":"`+err.Error()+`","type":"error"}}`)
			return c.NoContent(http.StatusOK)
		}
		return c.Redirect(http.StatusFound, "/groups/"+groupID)
	}

	if isHTMX(c) {
		return h.htmxRedirect(c, groupID)
	}
	return c.Redirect(http.StatusFound, "/groups/"+groupID)
}

func (h *MatchHandler) EditModal(c *echo.Context) error {
	groupID := c.Param("id")
	matchID := c.Param("mid")

	members, err := h.repo.ListMembers(c.Request().Context(), groupID)
	if err != nil {
		return err
	}

	editable, err := h.service.GetEditable(c.Request().Context(), matchID)
	if err != nil {
		return err
	}

	teams := make(map[string]string)
	for _, pid := range editable.TeamAPlayers {
		teams[pid] = "a"
	}
	for _, pid := range editable.TeamBPlayers {
		teams[pid] = "b"
	}

	editData := &matches.MatchEditData{
		MatchID:   matchID,
		TeamAName: editable.TeamAName,
		TeamBName: editable.TeamBName,
		ScoreA:    editable.ScoreA,
		ScoreB:    editable.ScoreB,
		Teams:     teams,
		Goals:     editable.Goals,
	}

	return render.Component(c, matches.LogForm(groupID, members, editData))
}

func (h *MatchHandler) Update(c *echo.Context) error {
	_, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	groupID := c.Param("id")
	matchID := c.Param("mid")
	scoreA, _ := strconv.Atoi(c.FormValue("score_a"))
	scoreB, _ := strconv.Atoi(c.FormValue("score_b"))

	teamAPlayers, teamBPlayers := h.parseTeamPlayers(c)
	goalsInput := h.parseGoalsFromForm(c)

	teamAName := c.FormValue("team_a_name")
	teamBName := c.FormValue("team_b_name")
	if teamAName == "" {
		teamAName = "Shirts"
	}
	if teamBName == "" {
		teamBName = "Skins"
	}

	input := service.UpdateMatchInput{
		MatchID:      matchID,
		TeamAName:    teamAName,
		TeamBName:    teamBName,
		ScoreA:       scoreA,
		ScoreB:       scoreB,
		TeamAPlayers: teamAPlayers,
		TeamBPlayers: teamBPlayers,
		GoalsInput:   goalsInput,
	}

	if err := h.service.Update(c.Request().Context(), input); err != nil {
		if isHTMX(c) {
			c.Response().Header().Set("HX-Trigger", `{"showToast":{"message":"`+err.Error()+`","type":"error"}}`)
			return c.NoContent(http.StatusOK)
		}
		h.auth.Handler.SetFlash(c.Request().Context(), "error", err.Error())
		return c.Redirect(http.StatusFound, "/groups/"+groupID)
	}

	if isHTMX(c) {
		return h.htmxRedirect(c, groupID)
	}
	h.auth.Handler.SetFlash(c.Request().Context(), "success", "Match updated!")
	return c.Redirect(http.StatusFound, "/groups/"+groupID)
}
