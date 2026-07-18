package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/model"
	"nutmeg/internal/render"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/views/pages/groups"
	"nutmeg/views/pages/home"
)

type GroupHandler struct {
	auth     *ezauth.EzAuth
	service  *service.GroupService
	matchSvc *service.MatchService
	repo     *repository.Repository
}

func NewGroupHandler(auth *ezauth.EzAuth, svc *service.GroupService, matchSvc *service.MatchService, repo *repository.Repository) *GroupHandler {
	return &GroupHandler{auth: auth, service: svc, matchSvc: matchSvc, repo: repo}
}

func (h *GroupHandler) Index(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	list, err := h.service.List(c.Request().Context(), userID)
	if err != nil {
		return err
	}

	return page(c, "My Groups", true, "", h.userName(c), groups.List(list))
}

func (h *GroupHandler) New(c *echo.Context) error {
	return page(c, "New Group", true, "", h.userName(c), groups.Form("", nil))
}

func (h *GroupHandler) Create(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	name := c.FormValue("name")
	if name == "" {
		if isHTMX(c) {
			c.Response().Header().Set("HX-Trigger", `{"showToast":{"message":"Name is required","type":"error"}}`)
			return c.NoContent(http.StatusOK)
		}
		return page(c, "New Group", true, "", h.userName(c), groups.Form("", &groups.FormData{Error: "Name is required"}))
	}

	g, err := h.service.Create(c.Request().Context(), name, nil, userID)
	if err != nil {
		if isHTMX(c) {
			c.Response().Header().Set("HX-Trigger", `{"showToast":{"message":"`+err.Error()+`","type":"error"}}`)
			return c.NoContent(http.StatusOK)
		}
		return page(c, "New Group", true, "", h.userName(c), groups.Form("", &groups.FormData{Name: name, Error: err.Error()}))
	}

	if isHTMX(c) {
		return h.groupListFragment(c, userID)
	}

	return c.Redirect(http.StatusFound, "/groups/"+g.ID)
}

func (h *GroupHandler) groupListFragment(c *echo.Context, userID string) error {
	list, err := h.service.List(c.Request().Context(), userID)
	if err != nil {
		return err
	}

	referer := c.Request().Header.Get("HX-Current-URL")
	c.Response().Header().Set("HX-Trigger", `{"showToast":{"message":"Group created!","type":"success"}}`)
	if strings.Contains(referer, "/dashboard") {
		return render.Component(c, home.DashboardGroupList(list))
	}
	return render.Component(c, groups.GroupGrid(list))
}

func (h *GroupHandler) Detail(c *echo.Context) error {
	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}

	members, err := h.service.Members(c.Request().Context(), id)
	if err != nil {
		return err
	}

	isAdmin := h.isAdmin(c, members)

	leaderboard, _ := h.matchSvc.GetLeaderboard(c.Request().Context(), id)
	lbEntries := make([]groups.LeaderboardEntry, len(leaderboard))
	for i, e := range leaderboard {
		lbEntries[i] = groups.LeaderboardEntry{
			Name:    e.Name,
			Wins:    e.Wins,
			Losses:  e.Losses,
			Goals:   e.Goals,
			Assists: e.Assists,
		}
	}

	matches, _ := h.matchSvc.ListByGroup(c.Request().Context(), id)
	matchEntries := make([]groups.MatchEntry, len(matches))
	for i, m := range matches {
		matchEntries[i] = groups.MatchEntry{
			ID:      m.ID,
			GroupID: m.GroupID,
			TeamA:   m.TeamAName,
			TeamB:   m.TeamBName,
			ScoreA:  m.ScoreA,
			ScoreB:  m.ScoreB,
			Date:    m.PlayedAt.Format("Jan 2"),
		}
	}

	successMsg := h.auth.GetSuccessMessage(c.Request().Context())
	errMsg := h.auth.GetErrorMessage(c.Request().Context())

	return page(c, g.Name, true, g.ID, h.userName(c), groups.Detail(g, members, isAdmin, lbEntries, matchEntries, successMsg, errMsg))
}

func (h *GroupHandler) Edit(c *echo.Context) error {
	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return page(c, "Edit Group", true, g.ID, h.userName(c), groups.Form(g.ID, &groups.FormData{
		Name:        g.Name,
		Description: stringPtrValue(g.Description),
	}))
}

func (h *GroupHandler) Update(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	name := c.FormValue("name")
	if name == "" {
		return page(c, "Edit Group", true, id, h.userName(c), groups.Form(id, &groups.FormData{Error: "Name is required"}))
	}

	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}

	g.Name = name
	desc := c.FormValue("description")
	if desc == "" {
		g.Description = nil
	} else {
		g.Description = &desc
	}

	if err := h.service.Update(c.Request().Context(), g, userID); err != nil {
		return page(c, "Edit Group", true, id, h.userName(c), groups.Form(id, &groups.FormData{Name: name, Description: desc, Error: err.Error()}))
	}

	return c.Redirect(http.StatusFound, "/groups/"+id)
}

func (h *GroupHandler) Delete(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	if err := h.service.Delete(c.Request().Context(), id, userID); err != nil {
		if err == model.ErrNotAuthorized {
			return c.String(http.StatusForbidden, "Not authorized")
		}
		return err
	}

	return c.Redirect(http.StatusFound, "/groups")
}

func (h *GroupHandler) DetailContent(c *echo.Context) error {
	id := c.Param("id")

	leaderboard, _ := h.matchSvc.GetLeaderboard(c.Request().Context(), id)
	lbEntries := make([]groups.LeaderboardEntry, len(leaderboard))
	for i, e := range leaderboard {
		lbEntries[i] = groups.LeaderboardEntry{
			Name:    e.Name,
			Wins:    e.Wins,
			Losses:  e.Losses,
			Goals:   e.Goals,
			Assists: e.Assists,
		}
	}

	matches, _ := h.matchSvc.ListByGroup(c.Request().Context(), id)
	matchEntries := make([]groups.MatchEntry, len(matches))
	for i, m := range matches {
		matchEntries[i] = groups.MatchEntry{
			ID:      m.ID,
			GroupID: m.GroupID,
			TeamA:   m.TeamAName,
			TeamB:   m.TeamBName,
			ScoreA:  m.ScoreA,
			ScoreB:  m.ScoreB,
			Date:    m.PlayedAt.Format("Jan 2"),
		}
	}

	return render.Component(c, groups.DetailContent(lbEntries, matchEntries))
}

func (h *GroupHandler) RosterContent(c *echo.Context) error {
	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}

	members, err := h.service.Members(c.Request().Context(), id)
	if err != nil {
		return err
	}

	isAdmin := h.isAdmin(c, members)

	return render.Component(c, groups.RosterColumn(g, members, isAdmin))
}

func (h *GroupHandler) AddMember(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	email := c.FormValue("email")
	if email == "" {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, "Email is required", "error")
		}
		h.auth.Handler.SetFlash(c.Request().Context(), "error", "Email is required")
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	ctx := c.Request().Context()
	targetUserID, err := h.repo.GetUserByEmail(ctx, email)
	if err != nil {
		slog.Info("Simulating sending invitation email", "email", email, "group_id", id)
		if isHTMX(c) {
			return h.rosterWithToast(c, id, "User not found. Invitation sent to "+email, "success")
		}
		h.auth.Handler.SetFlash(ctx, "success", "User with email "+email+" does not exist. An invitation email was sent to them!")
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	if err := h.service.AddMember(ctx, id, targetUserID, userID); err != nil {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, err.Error(), "error")
		}
		h.auth.Handler.SetFlash(ctx, "error", err.Error())
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	if isHTMX(c) {
		return h.rosterWithToast(c, id, "Added "+email, "success")
	}

	h.auth.Handler.SetFlash(ctx, "success", "Added member "+email+" successfully!")
	return c.Redirect(http.StatusFound, "/groups/"+id)
}

func (h *GroupHandler) RemoveMember(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	memberID := c.Param("uid")

	if err := h.service.RemoveMember(c.Request().Context(), id, memberID, userID); err != nil {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, err.Error(), "error")
		}
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	if isHTMX(c) {
		return h.rosterWithToast(c, id, "Member removed", "success")
	}

	return c.Redirect(http.StatusFound, "/groups/"+id)
}

func (h *GroupHandler) rosterWithToast(c *echo.Context, groupID, message, toastType string) error {
	g, err := h.service.Get(c.Request().Context(), groupID)
	if err != nil {
		return err
	}
	members, err := h.service.Members(c.Request().Context(), groupID)
	if err != nil {
		return err
	}
	isAdmin := h.isAdmin(c, members)

	c.Response().Header().Set("HX-Trigger", `{"showToast":{"message":"`+message+`","type":"`+toastType+`"}}`)
	return render.Component(c, groups.RosterColumn(g, members, isAdmin))
}

func isHTMX(c *echo.Context) bool {
	return c.Request().Header.Get("HX-Request") == "true"
}

func (h *GroupHandler) userName(c *echo.Context) string {
	user, err := ezauth.GetUser(c.Request().Context())
	if err != nil {
		return ""
	}
	if user.FirstName != "" || user.LastName != "" {
		return user.FirstName + " " + user.LastName
	}
	return user.Email
}

func (h *GroupHandler) isAdmin(c *echo.Context, members []repository.MemberInfo) bool {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return false
	}
	for _, m := range members {
		if m.UserID == userID && m.Role == "admin" {
			return true
		}
	}
	return false
}

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
