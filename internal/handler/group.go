package handler

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	appmw "nutmeg/internal/middleware"
	"nutmeg/internal/model"
	"nutmeg/internal/render"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/internal/worker"
	"nutmeg/views/pages/groups"
	"nutmeg/views/pages/home"
	"nutmeg/views/pages/players"
)

type GroupHandler struct {
	auth          *ezauth.EzAuth
	service       *service.GroupService
	matchSvc      *service.MatchService
	repo          *repository.Repository
	commentarySvc *service.CommentaryService
	jobs          JobEnqueuer
}

func NewGroupHandler(auth *ezauth.EzAuth, svc *service.GroupService, matchSvc *service.MatchService, repo *repository.Repository, commentarySvc *service.CommentaryService, jobs JobEnqueuer) *GroupHandler {
	return &GroupHandler{auth: auth, service: svc, matchSvc: matchSvc, repo: repo, commentarySvc: commentarySvc, jobs: jobs}
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

	user, err := ezauth.GetUser(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}
	creatorName := h.userName(c)

	g, err := h.service.Create(c.Request().Context(), name, nil, userID, creatorName, user.Email)
	if err != nil {
		if isHTMX(c) {
			c.Response().Header().Set("HX-Trigger", toastHXTrigger(err.Error(), "error"))
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
	c.Response().Header().Set("HX-Trigger", toastHXTrigger("Group created!", "success"))
	if strings.Contains(referer, "/dashboard") {
		return render.Component(c, home.DashboardGroupList(list))
	}
	return render.Component(c, groups.GroupGrid(list))
}

func (h *GroupHandler) Detail(c *echo.Context) error {
	id := c.Param("id")
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/groups/"+id+"/leaderboard")
	}

	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}

	canEdit, isOwner, ownerEmail := h.rosterViewData(c.Request().Context(), g, userID)
	if !canEdit {
		return c.Redirect(http.StatusFound, "/dashboard")
	}

	members, err := h.service.Members(c.Request().Context(), id)
	if err != nil {
		return err
	}

	sortBy := c.QueryParam("sort")
	leaderboard, lbErr := h.matchSvc.GetLeaderboard(c.Request().Context(), id, sortBy)
	if lbErr != nil {
		slog.Error("failed to get leaderboard", "group_id", id, "error", lbErr)
	}

	matches, matchErr := h.matchSvc.ListByGroup(c.Request().Context(), id)
	if matchErr != nil {
		slog.Error("failed to list matches", "group_id", id, "error", matchErr)
	}

	joinRequests := h.joinRequestEntries(c.Request().Context(), id, canEdit)

	activity, actErr := h.repo.ListGroupActivity(c.Request().Context(), id, 20)
	if actErr != nil {
		slog.Error("failed to list group activity", "group_id", id, "error", actErr)
	}

	successMsg := h.auth.GetSuccessMessage(c.Request().Context())
	errMsg := h.auth.GetErrorMessage(c.Request().Context())

	return page(c, g.Name, true, g.ID, h.userName(c), groups.Detail(g, members, canEdit, isOwner, ownerEmail, joinRequests, mapLeaderboardEntries(leaderboard), mapMatchEntries(matches, appmw.LocationFromContext(c)), mapActivityEntries(activity, appmw.LocationFromContext(c)), sortBy, successMsg, errMsg))
}

// mapLeaderboardEntries converts repository leaderboard rows into the
// groups view struct — shared by every handler that renders a leaderboard
// (Detail, DetailContent, PublicLeaderboard, LeaderboardFull).
func mapLeaderboardEntries(entries []repository.LeaderboardEntry) []groups.LeaderboardEntry {
	out := make([]groups.LeaderboardEntry, len(entries))
	for i, e := range entries {
		out[i] = groups.LeaderboardEntry{
			ID:      e.MemberID,
			Name:    e.Name,
			Matches: e.Matches,
			Wins:    e.Wins,
			Draws:   e.Draws,
			Losses:  e.Losses,
			Goals:   e.Goals,
			Assists: e.Assists,
		}
	}
	return out
}

// mapMatchEntries converts repository matches into the groups view struct —
// shared by every handler that renders the Recent Matches list (Detail,
// DetailContent, MatchesFull).
func mapMatchEntries(matches []repository.MatchWithTeams, loc *time.Location) []groups.MatchEntry {
	out := make([]groups.MatchEntry, len(matches))
	for i, m := range matches {
		out[i] = groups.MatchEntry{
			ID:      m.ID,
			GroupID: m.GroupID,
			TeamA:   m.TeamAName,
			TeamB:   m.TeamBName,
			ScoreA:  m.ScoreA,
			ScoreB:  m.ScoreB,
			Date:    m.PlayedAt.In(loc).Format("Jan 2"),
		}
	}
	return out
}

// mapActivityEntries converts repository group_activity rows into the
// groups view struct — shared by every handler that renders the activity
// feed (Detail, DetailContent, detailContentWithToast).
func mapActivityEntries(entries []model.GroupActivity, loc *time.Location) []groups.ActivityEntry {
	out := make([]groups.ActivityEntry, len(entries))
	for i, e := range entries {
		out[i] = groups.ActivityEntry{
			Kind: e.Kind,
			Text: e.Content,
			When: e.CreatedAt.In(loc).Format("Jan 2, 3:04 PM"),
		}
	}
	return out
}

// PublicLeaderboard renders a group's leaderboard for anyone, logged in or
// not — unlike Detail/DetailContent it doesn't require a session or CanEdit,
// since the leaderboard data itself isn't scoped to the viewer.
func (h *GroupHandler) PublicLeaderboard(c *echo.Context) error {
	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}

	sortBy := c.QueryParam("sort")
	leaderboard, lbErr := h.matchSvc.GetLeaderboard(c.Request().Context(), g.ID, sortBy)
	if lbErr != nil {
		slog.Error("failed to get leaderboard", "group_id", g.ID, "error", lbErr)
	}
	lbEntries := mapLeaderboardEntries(leaderboard)

	ctx := c.Request().Context()
	isLoggedIn := false
	userName := ""
	userID := ""
	if user, err := ezauth.GetUser(ctx); err == nil {
		isLoggedIn = true
		userName = user.DisplayName()
		if uid, err := h.auth.GetUserID(ctx); err == nil {
			userID = uid
		}
	}
	joinStatus := h.service.ViewerJoinStatus(ctx, g, userID)

	successMsg := h.auth.GetSuccessMessage(ctx)
	errMsg := h.auth.GetErrorMessage(ctx)

	description := fmt.Sprintf("%s's leaderboard on Nutmeg — %d players tracked, updated after every match.", g.Name, len(lbEntries))
	return pageWithMeta(c, g.Name+" Leaderboard", description, isLoggedIn, "", userName, groups.PublicLeaderboard(g, lbEntries, joinStatus, sortBy, successMsg, errMsg))
}

// PlayerProfile renders a single player's stats. Like PublicLeaderboard,
// it's public/unauthenticated — anyone with the link can view a player's
// stats within a group's shareable leaderboard.
func (h *GroupHandler) PlayerProfile(c *echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")
	memberID := c.Param("memberId")

	g, err := h.service.Get(ctx, id)
	if err != nil {
		return err
	}
	player, err := h.repo.GetMember(ctx, g.ID, memberID)
	if err != nil {
		return err
	}
	stats, err := h.matchSvc.GetPlayerStats(ctx, player.ID)
	if err != nil {
		return err
	}

	var commentary *string
	if pc, err := h.repo.GetActivePlayerCommentary(ctx, player.ID); err != nil {
		slog.Error("failed to get player commentary", "member_id", player.ID, "error", err)
	} else if pc != nil {
		commentary = &pc.Content
	}

	history, histErr := h.repo.GetPlayerMatchHistory(ctx, player.ID, 10)
	if histErr != nil {
		slog.Error("failed to get player match history", "member_id", player.ID, "error", histErr)
	}
	chartData := buildChartData(stats, history, appmw.LocationFromContext(c))

	isTopScorer := false
	isTopPasser := false
	if entries, lbErr := h.matchSvc.GetLeaderboard(ctx, g.ID, ""); lbErr != nil {
		slog.Error("failed to get leaderboard for profile badges", "group_id", g.ID, "error", lbErr)
	} else {
		isTopScorer = repository.TopScorerID(entries) == player.ID
		isTopPasser = repository.TopPasserID(entries) == player.ID
	}

	isLoggedIn := false
	userName := ""
	canEdit := false
	if user, err := ezauth.GetUser(ctx); err == nil {
		isLoggedIn = true
		userName = user.DisplayName()
		if userID, err := h.auth.GetUserID(ctx); err == nil {
			canEdit = h.service.CanEdit(ctx, g, userID)
		}
	}

	successMsg := h.auth.GetSuccessMessage(ctx)
	errMsg := h.auth.GetErrorMessage(ctx)

	description := fmt.Sprintf("%s — %d matches, %d wins, %d goals for %s on Nutmeg.", player.Name, stats.MatchesPlayed, stats.Wins, stats.Goals, g.Name)
	if commentary != nil && *commentary != "" {
		description = *commentary
	}
	description = truncateMeta(description, 200)

	return pageWithMeta(c, player.Name+" — "+g.Name, description, isLoggedIn, "", userName, players.Profile(g, player, stats, commentary, chartData, isTopScorer, isTopPasser, canEdit, successMsg, errMsg))
}

// buildChartData maps a player's aggregate stats and recent match history
// (newest-first) into the profile page's chart payload, oldest-first so
// the goals-per-match chart reads left-to-right chronologically.
func buildChartData(stats *repository.PlayerStats, history []repository.PlayerMatchResult, loc *time.Location) players.ChartData {
	data := players.ChartData{Wins: stats.Wins, Draws: stats.Draws, Losses: stats.Losses}
	for i := len(history) - 1; i >= 0; i-- {
		m := history[i]
		data.Labels = append(data.Labels, m.PlayedAt.In(loc).Format("Jan 2"))
		data.Goals = append(data.Goals, m.GoalsScored)
	}
	return data
}

// truncateMeta hard-caps a link-preview description to max characters,
// breaking on a word boundary when possible. Distinct from the service
// package's sentence-boundary roast trimming (truncateAtSentence) — this
// solves a different problem (a crawler-snippet length cap, not display
// readability), so it isn't shared with that logic.
func truncateMeta(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "…"
}

// RegenerateCommentary lets a group admin manually re-run roast generation
// for a single player — the same generation+validation flow as automatic
// post-match generation (GenerateCommentaryArgs with no MatchID), gated by
// CanEdit and a per-player cooldown so a repeatedly-clicked button can't
// hammer the LLM on a memory-constrained box.
func (h *GroupHandler) RegenerateCommentary(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := h.auth.GetUserID(ctx)
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	memberID := c.Param("memberId")
	profileURL := "/groups/" + id + "/players/" + memberID

	g, err := h.service.Get(ctx, id)
	if err != nil {
		return err
	}
	if !h.service.CanEdit(ctx, g, userID) {
		return c.Redirect(http.StatusFound, "/dashboard")
	}

	ok, wait, err := h.commentarySvc.CanRegenerate(ctx, memberID)
	if err != nil {
		return err
	}
	if !ok {
		h.auth.Handler.SetFlash(ctx, "error", fmt.Sprintf("Too soon — try again in %d minute(s).", int(wait.Minutes())+1))
		return c.Redirect(http.StatusFound, profileURL)
	}

	if _, err := h.jobs.Insert(ctx, worker.GenerateCommentaryArgs{GroupPlayerID: memberID}, nil); err != nil {
		h.auth.Handler.SetFlash(ctx, "error", "Could not start regeneration: "+err.Error())
		return c.Redirect(http.StatusFound, profileURL)
	}

	h.auth.Handler.SetFlash(ctx, "success", "Regenerating commentary — refresh in a few seconds.")
	return c.Redirect(http.StatusFound, profileURL)
}

func (h *GroupHandler) RequestJoin(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := h.auth.GetUserID(ctx)
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	g, err := h.service.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := h.service.RequestToJoin(ctx, g, userID); err != nil {
		h.auth.Handler.SetFlash(ctx, "error", err.Error())
		return c.Redirect(http.StatusFound, "/groups/"+id+"/leaderboard")
	}
	enqueueEmail(ctx, h.jobs, h.adminEmails(ctx, id), "New join request for "+g.Name,
		"Someone requested to join "+g.Name+". Review the request from the group's roster page.")

	h.auth.Handler.SetFlash(ctx, "success", "Request sent! The group admin will review it.")
	return c.Redirect(http.StatusFound, "/groups/"+id+"/leaderboard")
}

func (h *GroupHandler) ApproveJoinRequest(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := h.auth.GetUserID(ctx)
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	reqID := c.Param("reqId")
	g, err := h.service.Get(ctx, id)
	if err != nil {
		return err
	}

	req, err := h.service.ApproveJoinRequest(ctx, g, reqID, userID)
	if err != nil {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, err.Error(), "error")
		}
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}
	enqueueEmail(ctx, h.jobs, []string{req.Email}, "You're in! Welcome to "+g.Name,
		"Your request to join "+g.Name+" was approved. Head to the group page to see the roster and leaderboard.")

	if isHTMX(c) {
		return h.rosterWithToast(c, id, "Member approved", "success")
	}
	return c.Redirect(http.StatusFound, "/groups/"+id)
}

func (h *GroupHandler) RejectJoinRequest(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := h.auth.GetUserID(ctx)
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	reqID := c.Param("reqId")
	g, err := h.service.Get(ctx, id)
	if err != nil {
		return err
	}

	req, err := h.service.RejectJoinRequest(ctx, g, reqID, userID)
	if err != nil {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, err.Error(), "error")
		}
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}
	enqueueEmail(ctx, h.jobs, []string{req.Email}, "Update on your request to join "+g.Name,
		"Your request to join "+g.Name+" was not approved this time.")

	if isHTMX(c) {
		return h.rosterWithToast(c, id, "Request rejected", "success")
	}
	return c.Redirect(http.StatusFound, "/groups/"+id)
}

// adminEmails returns the email addresses of a group's admin members
// (includes the creator, who is added as an "admin"-role member at group
// creation — see GroupService.Create).
func (h *GroupHandler) adminEmails(ctx context.Context, groupID string) []string {
	members, err := h.repo.ListMembers(ctx, groupID)
	if err != nil {
		slog.Error("failed to list members for admin email lookup", "group_id", groupID, "error", err)
		return nil
	}
	var emails []string
	for _, m := range members {
		if m.Role == "admin" && m.Email != nil && *m.Email != "" {
			emails = append(emails, *m.Email)
		}
	}
	return emails
}

func (h *GroupHandler) Edit(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}

	if !h.service.CanEdit(c.Request().Context(), g, userID) {
		return c.Redirect(http.StatusFound, "/dashboard")
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

	if isHTMX(c) {
		c.Response().Header().Set("HX-Redirect", "/dashboard")
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusFound, "/dashboard")
}

// DetailContent re-renders the full swappable group body (leaderboard,
// roster, recent matches) — used to restore it after the match-log form's
// "Cancel" link, since logging/editing a match now swaps the whole area,
// not just the leaderboard.
func (h *GroupHandler) DetailContent(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}

	canEdit, isOwner, ownerEmail := h.rosterViewData(c.Request().Context(), g, userID)
	if !canEdit {
		return c.Redirect(http.StatusFound, "/dashboard")
	}

	members, err := h.service.Members(c.Request().Context(), id)
	if err != nil {
		return err
	}

	sortBy := c.QueryParam("sort")
	leaderboard, lbErr := h.matchSvc.GetLeaderboard(c.Request().Context(), id, sortBy)
	if lbErr != nil {
		slog.Error("failed to get leaderboard", "group_id", id, "error", lbErr)
	}

	matches, matchErr := h.matchSvc.ListByGroup(c.Request().Context(), id)
	if matchErr != nil {
		slog.Error("failed to list matches", "group_id", id, "error", matchErr)
	}

	joinRequests := h.joinRequestEntries(c.Request().Context(), id, canEdit)

	activity, actErr := h.repo.ListGroupActivity(c.Request().Context(), id, 20)
	if actErr != nil {
		slog.Error("failed to list group activity", "group_id", id, "error", actErr)
	}

	return render.Component(c, groups.GroupContent(g, members, canEdit, isOwner, ownerEmail, joinRequests, mapLeaderboardEntries(leaderboard), mapMatchEntries(matches, appmw.LocationFromContext(c)), mapActivityEntries(activity, appmw.LocationFromContext(c)), sortBy))
}

// LeaderboardFull renders the complete (untruncated) leaderboard, taking
// over #detail-content-area — the "Show all" counterpart to GroupContent's
// truncated leaderboard. The rendered "← Back" link restores the normal
// view via DetailContent.
func (h *GroupHandler) LeaderboardFull(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if !h.service.CanEdit(c.Request().Context(), g, userID) {
		return c.Redirect(http.StatusFound, "/dashboard")
	}

	sortBy := c.QueryParam("sort")
	leaderboard, lbErr := h.matchSvc.GetLeaderboard(c.Request().Context(), id, sortBy)
	if lbErr != nil {
		slog.Error("failed to get leaderboard", "group_id", id, "error", lbErr)
	}

	return render.Component(c, groups.LeaderboardSection(id, sortBy, mapLeaderboardEntries(leaderboard), true))
}

// RosterFull renders the complete (untruncated) roster, taking over
// #detail-content-area.
func (h *GroupHandler) RosterFull(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	canEdit, isOwner, ownerEmail := h.rosterViewData(c.Request().Context(), g, userID)
	if !canEdit {
		return c.Redirect(http.StatusFound, "/dashboard")
	}

	members, err := h.service.Members(c.Request().Context(), id)
	if err != nil {
		return err
	}

	joinRequests := h.joinRequestEntries(c.Request().Context(), id, canEdit)

	return render.Component(c, groups.RosterColumn(g, members, canEdit, isOwner, ownerEmail, joinRequests, true))
}

// MatchesFull renders the complete (untruncated) recent-matches list, taking
// over #detail-content-area.
func (h *GroupHandler) MatchesFull(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if !h.service.CanEdit(c.Request().Context(), g, userID) {
		return c.Redirect(http.StatusFound, "/dashboard")
	}

	matches, matchErr := h.matchSvc.ListByGroup(c.Request().Context(), id)
	if matchErr != nil {
		slog.Error("failed to list matches", "group_id", id, "error", matchErr)
	}

	return render.Component(c, groups.RecentMatchesColumn(id, mapMatchEntries(matches, appmw.LocationFromContext(c)), true))
}

func (h *GroupHandler) AddMember(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	raw := c.FormValue("name")
	names := splitNames(raw)
	if len(names) == 0 {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, "Name is required", "error")
		}
		h.auth.Handler.SetFlash(c.Request().Context(), "error", "Name is required")
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	ctx := c.Request().Context()

	if len(names) == 1 {
		name := names[0]
		var phonePtr, emailPtr *string
		if phone := c.FormValue("phone"); phone != "" {
			phonePtr = &phone
		}
		if email := c.FormValue("email"); email != "" {
			emailPtr = &email
		}

		memberID, err := h.service.AddMember(ctx, id, name, phonePtr, emailPtr, userID)
		if err != nil {
			if isHTMX(c) {
				return h.rosterWithToast(c, id, err.Error(), "error")
			}
			h.auth.Handler.SetFlash(ctx, "error", err.Error())
			return c.Redirect(http.StatusFound, "/groups/"+id)
		}
		recordActivity(ctx, h.repo, h.jobs, id, "player_added", memberID, name+" joined the group")

		if isHTMX(c) {
			return h.rosterWithToast(c, id, "Added "+name, "success")
		}
		h.auth.Handler.SetFlash(ctx, "success", "Added member "+name+" successfully!")
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	added, err := h.service.AddMembers(ctx, id, names, userID)
	if err != nil {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, err.Error(), "error")
		}
		h.auth.Handler.SetFlash(ctx, "error", err.Error())
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	msg := fmt.Sprintf("Added %d players", len(added))
	if isHTMX(c) {
		return h.rosterWithToast(c, id, msg, "success")
	}
	h.auth.Handler.SetFlash(ctx, "success", msg)
	return c.Redirect(http.StatusFound, "/groups/"+id)
}

// splitNames turns a comma-separated "name" field into a deduplicated-free
// list of trimmed, non-empty names.
func splitNames(raw string) []string {
	parts := strings.Split(raw, ",")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		if n := strings.TrimSpace(p); n != "" {
			names = append(names, n)
		}
	}
	return names
}

const maxImportFileSize = 1 << 20 // 1MB

func (h *GroupHandler) ImportMembers(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	ctx := c.Request().Context()

	failWith := func(msg string) error {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, msg, "error")
		}
		h.auth.Handler.SetFlash(ctx, "error", msg)
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	fileHeader, err := c.FormFile("csv")
	if err != nil {
		return failWith("Choose a CSV file to import")
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".csv") {
		return failWith("File must be a .csv")
	}
	if fileHeader.Size > maxImportFileSize {
		return failWith("CSV file is too large (max 1MB)")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	rows, err := parseMemberImportCSV(file)
	if err != nil {
		return failWith(err.Error())
	}

	imported, updated, skipped, err := h.service.ImportMembers(ctx, id, rows, userID)
	if err != nil {
		return failWith(err.Error())
	}

	msg := fmt.Sprintf("Imported %d, updated %d", imported, updated)
	if skipped > 0 {
		msg += fmt.Sprintf(", skipped %d", skipped)
	}
	if isHTMX(c) {
		return h.rosterWithToast(c, id, msg, "success")
	}
	h.auth.Handler.SetFlash(ctx, "success", msg)
	return c.Redirect(http.StatusFound, "/groups/"+id)
}

// parseMemberImportCSV reads a roster CSV with a header row (case-insensitive
// column names, any order) into service.ImportRow values. Only a "name"
// column is required; "phone" and "email" are optional.
func parseMemberImportCSV(r io.Reader) ([]service.ImportRow, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // tolerate rows missing trailing optional columns

	header, err := reader.Read()
	if err != nil {
		return nil, errors.New("could not read CSV header row")
	}

	col := make(map[string]int, len(header))
	for i, name := range header {
		col[strings.ToLower(strings.TrimSpace(name))] = i
	}
	nameIdx, ok := col["name"]
	if !ok {
		return nil, errors.New(`CSV must have a "name" column`)
	}
	phoneIdx, hasPhone := col["phone"]
	emailIdx, hasEmail := col["email"]

	field := func(record []string, idx int, has bool) string {
		if !has || idx >= len(record) {
			return ""
		}
		return record[idx]
	}

	var rows []service.ImportRow
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid CSV row: %w", err)
		}
		if nameIdx >= len(record) {
			continue
		}
		rows = append(rows, service.ImportRow{
			Name:  record[nameIdx],
			Phone: field(record, phoneIdx, hasPhone),
			Email: field(record, emailIdx, hasEmail),
		})
	}
	return rows, nil
}

func (h *GroupHandler) RemoveMember(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	memberID := c.Param("memberId")

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

func (h *GroupHandler) PromoteMember(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	memberID := c.Param("memberId")

	if err := h.service.PromoteMember(c.Request().Context(), id, memberID, userID); err != nil {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, err.Error(), "error")
		}
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	if isHTMX(c) {
		return h.rosterWithToast(c, id, "Member promoted to admin", "success")
	}

	return c.Redirect(http.StatusFound, "/groups/"+id)
}

func (h *GroupHandler) DemoteMember(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	memberID := c.Param("memberId")

	if err := h.service.DemoteMember(c.Request().Context(), id, memberID, userID); err != nil {
		if isHTMX(c) {
			return h.rosterWithToast(c, id, err.Error(), "error")
		}
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	if isHTMX(c) {
		return h.rosterWithToast(c, id, "Admin demoted to member", "success")
	}

	return c.Redirect(http.StatusFound, "/groups/"+id)
}

func (h *GroupHandler) EditMemberForm(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := h.auth.GetUserID(ctx)
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	memberID := c.Param("memberId")

	g, err := h.service.Get(ctx, id)
	if err != nil {
		return err
	}
	if !h.service.CanEdit(ctx, g, userID) {
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	member, err := h.repo.GetMember(ctx, id, memberID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	return render.Component(c, groups.MemberEditForm(id, member, ""))
}

func (h *GroupHandler) UpdateMember(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := h.auth.GetUserID(ctx)
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	memberID := c.Param("memberId")
	name := c.FormValue("name")

	var phonePtr, emailPtr *string
	if phone := c.FormValue("phone"); phone != "" {
		phonePtr = &phone
	}
	if email := c.FormValue("email"); email != "" {
		emailPtr = &email
	}

	if err := h.service.UpdateMember(ctx, id, memberID, name, phonePtr, emailPtr, userID); err != nil {
		if isHTMX(c) {
			member := &model.GroupPlayer{ID: memberID, GroupID: id, Name: name, Phone: phonePtr, Email: emailPtr}
			return render.Component(c, groups.MemberEditForm(id, member, err.Error()))
		}
		h.auth.Handler.SetFlash(ctx, "error", err.Error())
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	if isHTMX(c) {
		return h.detailContentWithToast(c, id, "Member updated", "success")
	}
	h.auth.Handler.SetFlash(ctx, "success", "Member updated")
	return c.Redirect(http.StatusFound, "/groups/"+id)
}

// detailContentWithToast re-renders #detail-content-area with a toast,
// restoring the normal Leaderboard/Roster/Matches view after a mutation
// (like editing a player) that took over the whole area rather than just
// #roster-column — so saving a player edit drops the admin back into the
// roster listing instead of a full page reload.
func (h *GroupHandler) detailContentWithToast(c *echo.Context, groupID, message, toastType string) error {
	ctx := c.Request().Context()
	userID, err := h.auth.GetUserID(ctx)
	if err != nil {
		return err
	}
	g, err := h.service.Get(ctx, groupID)
	if err != nil {
		return err
	}
	canEdit, isOwner, ownerEmail := h.rosterViewData(ctx, g, userID)
	members, err := h.service.Members(ctx, groupID)
	if err != nil {
		return err
	}
	leaderboard, lbErr := h.matchSvc.GetLeaderboard(ctx, groupID, "")
	if lbErr != nil {
		slog.Error("failed to get leaderboard", "group_id", groupID, "error", lbErr)
	}
	matches, matchErr := h.matchSvc.ListByGroup(ctx, groupID)
	if matchErr != nil {
		slog.Error("failed to list matches", "group_id", groupID, "error", matchErr)
	}
	joinRequests := h.joinRequestEntries(ctx, groupID, canEdit)
	activity, actErr := h.repo.ListGroupActivity(ctx, groupID, 20)
	if actErr != nil {
		slog.Error("failed to list group activity", "group_id", groupID, "error", actErr)
	}

	c.Response().Header().Set("HX-Trigger", toastHXTrigger(message, toastType))
	return render.Component(c, groups.GroupContent(g, members, canEdit, isOwner, ownerEmail, joinRequests, mapLeaderboardEntries(leaderboard), mapMatchEntries(matches, appmw.LocationFromContext(c)), mapActivityEntries(activity, appmw.LocationFromContext(c)), ""))
}

// rosterWithToast re-renders #roster-column with a toast after a roster
// mutation. It preserves whichever view the mutation was triggered from —
// the normal truncated column, or the full-width "Show all" view (signaled
// by ?view=full, appended to every roster-mutation URL when RosterColumn is
// rendered with fullWidth=true) — so acting on a member deep in a large
// roster doesn't snap the admin back to a truncated view mid-task.
func (h *GroupHandler) rosterWithToast(c *echo.Context, groupID, message, toastType string) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return err
	}
	g, err := h.service.Get(c.Request().Context(), groupID)
	if err != nil {
		return err
	}
	members, err := h.service.Members(c.Request().Context(), groupID)
	if err != nil {
		return err
	}
	canEdit, isOwner, ownerEmail := h.rosterViewData(c.Request().Context(), g, userID)
	joinRequests := h.joinRequestEntries(c.Request().Context(), groupID, canEdit)
	fullWidth := c.QueryParam("view") == "full"

	c.Response().Header().Set("HX-Trigger", toastHXTrigger(message, toastType))
	return render.Component(c, groups.RosterColumn(g, members, canEdit, isOwner, ownerEmail, joinRequests, fullWidth))
}

// joinRequestEntries returns the group's pending join requests as view
// entries, or nil if the viewer can't manage the roster (canEdit is false)
// or the fetch fails.
func (h *GroupHandler) joinRequestEntries(ctx context.Context, groupID string, canEdit bool) []groups.JoinRequestEntry {
	if !canEdit {
		return nil
	}
	requests, err := h.service.JoinRequests(ctx, groupID)
	if err != nil {
		slog.Error("failed to list join requests", "group_id", groupID, "error", err)
		return nil
	}
	entries := make([]groups.JoinRequestEntry, len(requests))
	for i, r := range requests {
		entries[i] = groups.JoinRequestEntry{ID: r.ID, Name: r.Name, Email: r.Email}
	}
	return entries
}

// rosterViewData computes what the roster UI needs to know about the current
// viewer: whether they can edit (Owner or promoted admin), whether they are
// the Owner specifically, and the Owner's email (used to tell the Owner's
// roster row apart from a promoted admin's row, since both carry
// group_players.role == "admin").
func (h *GroupHandler) rosterViewData(ctx context.Context, g *model.Group, userID string) (canEdit, isOwner bool, ownerEmail string) {
	isOwner = g.CreatedBy == userID
	canEdit = h.service.CanEdit(ctx, g, userID)
	if owner, err := h.auth.Repo.UserGetByID(ctx, g.CreatedBy); err == nil {
		ownerEmail = owner.Email
	}
	return
}

func isHTMX(c *echo.Context) bool {
	return c.Request().Header.Get("HX-Request") == "true"
}

func (h *GroupHandler) userName(c *echo.Context) string {
	user, err := ezauth.GetUser(c.Request().Context())
	if err != nil {
		return ""
	}
	return user.DisplayName()
}

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
