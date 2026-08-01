package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	appmw "nutmeg/internal/middleware"
	"nutmeg/internal/render"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/internal/worker"
	"nutmeg/views/pages/matches"
)

func toastHXTrigger(message, toastType string) string {
	data, _ := json.Marshal(map[string]map[string]string{
		"showToast": {"message": message, "type": toastType},
	})
	return string(data)
}

type MatchHandler struct {
	auth    *ezauth.EzAuth
	service *service.MatchService
	repo    *repository.Repository
	jobs    JobEnqueuer
}

func NewMatchHandler(auth *ezauth.EzAuth, svc *service.MatchService, repo *repository.Repository, jobs JobEnqueuer) *MatchHandler {
	return &MatchHandler{auth: auth, service: svc, repo: repo, jobs: jobs}
}

// enqueueCommentary fires off async roast-commentary generation for every
// player in a just-logged match. Enqueueing is cheap and best-effort — a
// failure here is logged, not surfaced to the user, since it must never
// block or fail the match save that already succeeded.
func (h *MatchHandler) enqueueCommentary(ctx context.Context, matchID string, playerIDs []string) {
	for _, playerID := range playerIDs {
		_, err := h.jobs.Insert(ctx, worker.GenerateCommentaryArgs{
			GroupPlayerID: playerID,
			MatchID:       &matchID,
		}, nil)
		if err != nil {
			slog.Error("enqueue commentary generation failed", "group_player_id", playerID, "match_id", matchID, "error", err)
		}
	}
}

func (h *MatchHandler) LogMatchModal(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	groupID := c.Param("id")
	if err := h.service.AuthorizeGroupAccess(c.Request().Context(), groupID, userID); err != nil {
		return c.Redirect(http.StatusFound, "/dashboard")
	}

	members, err := h.repo.ListMembers(c.Request().Context(), groupID)
	if err != nil {
		return err
	}
	sortMembersByName(members)
	today := time.Now().In(appmw.LocationFromContext(c)).Format("2006-01-02")
	return render.Component(c, matches.LogForm(groupID, members, nil, today))
}

// sortMembersByName sorts members alphabetically by name (case-insensitive)
// in place — used only for the match-log form's player list, which is
// easier to scan alphabetically than the Roster panel's admin-first order.
func sortMembersByName(members []repository.MemberInfo) {
	sort.Slice(members, func(i, j int) bool {
		return strings.ToLower(members[i].Name) < strings.ToLower(members[j].Name)
	})
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

func (h *MatchHandler) parseAssistsFromForm(c *echo.Context) string {
	var parts []string
	for key, values := range c.Request().Form {
		if strings.HasPrefix(key, "assists_") {
			pid := strings.TrimPrefix(key, "assists_")
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

// parsePlayedAt reads the optional "played_at" date field (YYYY-MM-DD) and
// combines it with the current time-of-day in loc, so same-day matches keep
// natural ordering and an unset/invalid value simply falls back to now.
func parsePlayedAt(c *echo.Context, loc *time.Location) time.Time {
	now := time.Now().In(loc)
	val := c.FormValue("played_at")
	if val == "" {
		return now
	}
	d, err := time.Parse("2006-01-02", val)
	if err != nil {
		return now
	}
	return time.Date(d.Year(), d.Month(), d.Day(), now.Hour(), now.Minute(), now.Second(), 0, loc)
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
	if err := h.service.AuthorizeGroupAccess(c.Request().Context(), groupID, userID); err != nil {
		return c.Redirect(http.StatusFound, "/dashboard")
	}
	scoreA, _ := strconv.Atoi(c.FormValue("score_a"))
	scoreB, _ := strconv.Atoi(c.FormValue("score_b"))

	teamAPlayers, teamBPlayers := h.parseTeamPlayers(c)
	goalsInput := h.parseGoalsFromForm(c)
	assistsInput := h.parseAssistsFromForm(c)

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
		AssistsInput: assistsInput,
		PlayedAt:     parsePlayedAt(c, appmw.LocationFromContext(c)),
	}

	matchID, err := h.service.Create(c.Request().Context(), input)
	if err != nil {
		if isHTMX(c) {
			c.Response().Header().Set("HX-Trigger", toastHXTrigger(err.Error(), "error"))
			return c.NoContent(http.StatusOK)
		}
		h.auth.Handler.SetFlash(c.Request().Context(), "error", err.Error())
		return c.Redirect(http.StatusFound, "/groups/"+groupID)
	}
	h.enqueueCommentary(c.Request().Context(), matchID, append(teamAPlayers, teamBPlayers...))

	if isHTMX(c) {
		return h.htmxRedirect(c, groupID)
	}
	h.auth.Handler.SetFlash(c.Request().Context(), "success", "Match logged!")
	return c.Redirect(http.StatusFound, "/groups/"+groupID)
}

func (h *MatchHandler) Delete(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	matchID := c.Param("mid")
	groupID := c.Param("id")

	if err := h.service.Delete(c.Request().Context(), matchID, userID); err != nil {
		if isHTMX(c) {
			c.Response().Header().Set("HX-Trigger", toastHXTrigger(err.Error(), "error"))
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
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	groupID := c.Param("id")
	matchID := c.Param("mid")

	members, err := h.repo.ListMembers(c.Request().Context(), groupID)
	if err != nil {
		return err
	}
	sortMembersByName(members)

	editable, err := h.service.GetEditable(c.Request().Context(), matchID, userID)
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

	loc := appmw.LocationFromContext(c)
	editData := &matches.MatchEditData{
		MatchID:   matchID,
		TeamAName: editable.TeamAName,
		TeamBName: editable.TeamBName,
		ScoreA:    editable.ScoreA,
		ScoreB:    editable.ScoreB,
		Teams:     teams,
		Goals:     editable.Goals,
		Assists:   editable.Assists,
		PlayedAt:  editable.PlayedAt.In(loc),
	}

	today := time.Now().In(loc).Format("2006-01-02")
	return render.Component(c, matches.LogForm(groupID, members, editData, today))
}

func (h *MatchHandler) Update(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	groupID := c.Param("id")
	matchID := c.Param("mid")
	scoreA, _ := strconv.Atoi(c.FormValue("score_a"))
	scoreB, _ := strconv.Atoi(c.FormValue("score_b"))

	teamAPlayers, teamBPlayers := h.parseTeamPlayers(c)
	goalsInput := h.parseGoalsFromForm(c)
	assistsInput := h.parseAssistsFromForm(c)

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
		UserID:       userID,
		TeamAName:    teamAName,
		TeamBName:    teamBName,
		ScoreA:       scoreA,
		ScoreB:       scoreB,
		TeamAPlayers: teamAPlayers,
		TeamBPlayers: teamBPlayers,
		GoalsInput:   goalsInput,
		AssistsInput: assistsInput,
		PlayedAt:     parsePlayedAt(c, appmw.LocationFromContext(c)),
	}

	if err := h.service.Update(c.Request().Context(), input); err != nil {
		if isHTMX(c) {
			c.Response().Header().Set("HX-Trigger", toastHXTrigger(err.Error(), "error"))
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
