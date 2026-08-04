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

	"github.com/a-h/templ"
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
	articleSvc    *service.MatchArticleService
	jobs          JobEnqueuer
}

func NewGroupHandler(auth *ezauth.EzAuth, svc *service.GroupService, matchSvc *service.MatchService, repo *repository.Repository, commentarySvc *service.CommentaryService, articleSvc *service.MatchArticleService, jobs JobEnqueuer) *GroupHandler {
	return &GroupHandler{auth: auth, service: svc, matchSvc: matchSvc, repo: repo, commentarySvc: commentarySvc, articleSvc: articleSvc, jobs: jobs}
}

func (h *GroupHandler) Index(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
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
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	name := c.FormValue("name")
	if name == "" {
		if isHTMX(c) {
			c.Response().Header().Set(hxTrigger, `{"showToast":{"message":"Name is required","type":"error"}}`)
			return c.NoContent(http.StatusOK)
		}
		return page(c, "New Group", true, "", h.userName(c), groups.Form("", &groups.FormData{Error: "Name is required"}))
	}

	user, err := ezauth.GetUser(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, routeLogin)
	}
	creatorName := h.userName(c)

	g, err := h.service.Create(c.Request().Context(), name, nil, userID, creatorName, user.Email)
	if err != nil {
		if isHTMX(c) {
			c.Response().Header().Set(hxTrigger, toastHXTrigger(err.Error(), "error"))
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

	referer := c.Request().Header.Get(hxCurrentURL)
	c.Response().Header().Set(hxTrigger, toastHXTrigger("Group created!", "success"))
	if strings.Contains(referer, routeDashboard) {
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

	canEdit, isOwner, _ := h.rosterViewData(c.Request().Context(), g, userID)
	if !canEdit {
		return c.Redirect(http.StatusFound, routeDashboard)
	}

	tabContent, err := h.groupTabComponent(c, g, userID, c.QueryParam("tab"))
	if err != nil {
		return err
	}

	successMsg := h.auth.GetSuccessMessage(c.Request().Context())
	errMsg := h.auth.GetErrorMessage(c.Request().Context())

	return page(c, g.Name, true, g.ID, h.userName(c), groups.Detail(g, canEdit, isOwner, tabContent, successMsg, errMsg))
}

// groupTabComponent builds whichever tab's content the caller asked for
// ("roster", "matches", or the default "leaderboard") — shared by Detail
// (the default group-page load, which honors ?tab= so redirects like
// "match logged" can land on the right tab) and the LeaderboardFull /
// RosterFull / MatchesFull handlers (a tab-bar click).
func (h *GroupHandler) groupTabComponent(c *echo.Context, g *model.Group, userID string, tab string) (templ.Component, error) {
	ctx := c.Request().Context()
	switch tab {
	case "roster":
		canEdit, isOwner, ownerEmail := h.rosterViewData(ctx, g, userID)
		members, err := h.service.Members(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		joinRequests := h.joinRequestEntries(ctx, g.ID, canEdit)
		activity, actErr := h.repo.ListGroupActivity(ctx, g.ID, 20)
		if actErr != nil {
			slog.Error("failed to list group activity", "group_id", g.ID, "error", actErr)
		}
		return groups.RosterTab(g, members, canEdit, isOwner, ownerEmail, joinRequests, mapActivityEntries(activity, appmw.LocationFromContext(c))), nil
	case "matches":
		matches, matchErr := h.matchSvc.ListByGroup(ctx, g.ID)
		if matchErr != nil {
			slog.Error("failed to list matches", "group_id", g.ID, "error", matchErr)
		}
		activity, actErr := h.repo.ListGroupActivity(ctx, g.ID, 20)
		if actErr != nil {
			slog.Error("failed to list group activity", "group_id", g.ID, "error", actErr)
		}
		return groups.MatchesTab(g.ID, mapMatchEntries(matches, appmw.LocationFromContext(c)), mapActivityEntries(activity, appmw.LocationFromContext(c))), nil
	default:
		sortBy := c.QueryParam("sort")
		leaderboard, lbErr := h.matchSvc.GetLeaderboard(ctx, g.ID, sortBy)
		if lbErr != nil {
			slog.Error("failed to get leaderboard", "group_id", g.ID, "error", lbErr)
		}
		activity, actErr := h.repo.ListGroupActivity(ctx, g.ID, 20)
		if actErr != nil {
			slog.Error("failed to list group activity", "group_id", g.ID, "error", actErr)
		}
		return groups.LeaderboardTab(g.ID, sortBy, mapLeaderboardEntries(leaderboard), mapActivityEntries(activity, appmw.LocationFromContext(c))), nil
	}
}

// mapLeaderboardEntries converts repository leaderboard rows into the
// groups view struct — shared by every handler that renders a leaderboard
// (Detail, PublicLeaderboard, LeaderboardFull).
func mapLeaderboardEntries(entries []repository.LeaderboardEntry) []groups.LeaderboardEntry {
	out := make([]groups.LeaderboardEntry, len(entries))
	for i, e := range entries {
		out[i] = groups.LeaderboardEntry{
			ID:        e.MemberID,
			Name:      e.Name,
			Matches:   e.Matches,
			Wins:      e.Wins,
			Draws:     e.Draws,
			Losses:    e.Losses,
			Goals:     e.Goals,
			Assists:   e.Assists,
			Score:     e.Score,
			Qualified: e.Qualified,
		}
	}
	return out
}

// mapMatchEntries converts repository matches into the groups view struct —
// shared by every handler that renders the Recent Matches list (Detail,
// MatchesFull).
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
// feed (Detail, LeaderboardFull, RosterFull, MatchesFull, rosterTabWithToast).
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
// not — unlike Detail it doesn't require a session or CanEdit, since the
// leaderboard data itself isn't scoped to the viewer.
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
	matches, matchErr := h.matchSvc.ListByGroup(ctx, g.ID)
	if matchErr != nil {
		slog.Error("failed to list matches", "group_id", g.ID, "error", matchErr)
	}
	matchEntries := mapMatchEntries(matches, appmw.LocationFromContext(c))

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
	return pageWithMeta(c, g.Name+" Leaderboard", description, isLoggedIn, "", userName, groups.PublicLeaderboard(g, lbEntries, matchEntries, joinStatus, sortBy, successMsg, errMsg))
}

// PublicMatchArticle renders the AI-generated news article for a single
// match — public/unauthenticated, like PublicLeaderboard. An HTMX request
// (a match card click on the public or private page) gets just the
// overlay fragment; a direct/non-HTMX request (a shared link or a social
// crawler) gets a full standalone page with proper link-preview meta tags.
func (h *GroupHandler) PublicMatchArticle(c *echo.Context) error {
	ctx := c.Request().Context()
	groupID := c.Param("id")
	matchID := c.Param("mid")

	match, err := h.repo.GetMatchDetail(ctx, matchID)
	if err != nil || match.GroupID != groupID {
		return c.NoContent(http.StatusNotFound)
	}

	article, err := h.repo.GetMatchArticle(ctx, matchID)
	if err != nil {
		slog.Error("failed to get match article", "match_id", matchID, "error", err)
	}

	if isHTMX(c) {
		return render.Component(c, groups.MatchArticlePanel(match, article))
	}

	g, err := h.service.Get(ctx, groupID)
	if err != nil {
		return err
	}

	title := fmt.Sprintf("%s %d - %d %s", match.TeamAName, match.ScoreA, match.ScoreB, match.TeamBName)
	description := "The match report is being written up — check back in a few seconds."
	if article != nil {
		description = truncateMeta(article.Content, 200)
	}

	isLoggedIn := false
	userName := ""
	if user, err := ezauth.GetUser(ctx); err == nil {
		isLoggedIn = true
		userName = user.DisplayName()
	}

	return pageWithMeta(c, title+" — "+g.Name, description, isLoggedIn, "", userName, groups.MatchArticlePage(g, match, article))
}

// RegenerateMatchArticle lets a group admin manually re-run article
// generation for a single match — the same generation+validation flow as
// automatic post-match generation, gated by CanEdit and a per-match
// cooldown, mirroring RegenerateCommentary above.
func (h *GroupHandler) RegenerateMatchArticle(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	matchID := c.Param("mid")
	redirectURL := "/groups/" + id + "?tab=matches"

	_, err, done := h.requireGroupEdit(c, userID, routeDashboard)
	if done {
		return err
	}

	ok, wait, err := h.articleSvc.CanRegenerate(ctx, matchID)
	if err != nil {
		return err
	}
	if !ok {
		h.auth.Handler.SetFlash(ctx, "error", fmt.Sprintf("Too soon — try again in %d minute(s).", int(wait.Minutes())+1))
		return c.Redirect(http.StatusFound, redirectURL)
	}

	article, err := h.repo.GetMatchArticle(ctx, matchID)
	if err != nil || article == nil {
		h.auth.Handler.SetFlash(ctx, "error", "No article to regenerate for this match yet.")
		return c.Redirect(http.StatusFound, redirectURL)
	}

	if _, err := h.jobs.Insert(ctx, worker.GenerateMatchArticleArgs{ArticleID: article.ID, MatchID: matchID}, nil); err != nil {
		h.auth.Handler.SetFlash(ctx, "error", "Could not start regeneration: "+err.Error())
		return c.Redirect(http.StatusFound, redirectURL)
	}

	h.auth.Handler.SetFlash(ctx, "success", "Regenerating match article — refresh in a few seconds.")
	return c.Redirect(http.StatusFound, redirectURL)
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
	var score float64
	var qualified bool
	if entries, lbErr := h.matchSvc.GetLeaderboard(ctx, g.ID, ""); lbErr != nil {
		slog.Error("failed to get leaderboard for profile badges", "group_id", g.ID, "error", lbErr)
	} else {
		isTopScorer = repository.TopScorerID(entries) == player.ID
		isTopPasser = repository.TopPasserID(entries) == player.ID
		if entry, ok := repository.PlayerLeaderboardEntry(entries, player.ID); ok {
			score = entry.Score
			qualified = entry.Qualified
		}
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

	return pageWithMeta(c, player.Name+" — "+g.Name, description, isLoggedIn, "", userName, players.Profile(g, player, stats, score, qualified, commentary, chartData, isTopScorer, isTopPasser, canEdit, successMsg, errMsg))
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
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	memberID := c.Param("memberId")
	profileURL := "/groups/" + id + "/players/" + memberID

	_, err, done := h.requireGroupEdit(c, userID, routeDashboard)
	if done {
		return err
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
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
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
	EnqueueEmail(ctx, h.jobs, h.adminEmails(ctx, id), "New join request for "+g.Name,
		"Someone requested to join "+g.Name+". Review the request from the group's roster page.")

	h.auth.Handler.SetFlash(ctx, "success", "Request sent! The group admin will review it.")
	return c.Redirect(http.StatusFound, "/groups/"+id+"/leaderboard")
}

func (h *GroupHandler) ApproveJoinRequest(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	reqID := c.Param("reqId")
	g, err := h.service.Get(ctx, id)
	if err != nil {
		return err
	}

	req, err := h.service.ApproveJoinRequest(ctx, g, reqID, userID)
	if err != nil {
		return h.respondRosterMutation(c, id, err.Error(), "error")
	}
	EnqueueEmail(ctx, h.jobs, []string{req.Email}, "You're in! Welcome to "+g.Name,
		"Your request to join "+g.Name+" was approved. Head to the group page to see the roster and leaderboard.")

	return h.respondRosterMutation(c, id, "Member approved", "success")
}

func (h *GroupHandler) RejectJoinRequest(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	reqID := c.Param("reqId")
	g, err := h.service.Get(ctx, id)
	if err != nil {
		return err
	}

	req, err := h.service.RejectJoinRequest(ctx, g, reqID, userID)
	if err != nil {
		return h.respondRosterMutation(c, id, err.Error(), "error")
	}
	EnqueueEmail(ctx, h.jobs, []string{req.Email}, "Update on your request to join "+g.Name,
		"Your request to join "+g.Name+" was not approved this time.")

	return h.respondRosterMutation(c, id, "Request rejected", "success")
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
		if m.Role == model.RoleAdmin && m.Email != nil && *m.Email != "" {
			emails = append(emails, *m.Email)
		}
	}
	return emails
}

func (h *GroupHandler) Edit(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	g, err, done := h.requireGroupEdit(c, userID, routeDashboard)
	if done {
		return err
	}

	return page(c, "Edit Group", true, g.ID, h.userName(c), groups.Form(g.ID, &groups.FormData{
		Name:        g.Name,
		Description: stringPtrValue(g.Description),
	}))
}

func (h *GroupHandler) Update(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
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
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	if err := h.service.Delete(c.Request().Context(), id, userID); err != nil {
		if err == model.ErrNotAuthorized {
			return c.String(http.StatusForbidden, "Not authorized")
		}
		return err
	}

	if isHTMX(c) {
		c.Response().Header().Set(hxRedirect, routeDashboard)
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusFound, routeDashboard)
}

// LeaderboardFull renders the Leaderboard tab (tab bar + complete
// leaderboard + activity feed) — hit when the Leaderboard tab button is
// clicked.
func (h *GroupHandler) LeaderboardFull(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	g, err, done := h.requireGroupEdit(c, userID, routeDashboard)
	if done {
		return err
	}

	tabContent, err := h.groupTabComponent(c, g, userID, "leaderboard")
	if err != nil {
		return err
	}
	return render.Component(c, tabContent)
}

// requireGroupEdit loads the group identified by the request's "id" route
// param and verifies userID can edit it. On failure it returns done=true
// having already written the response itself — err is either the lookup
// failure (propagate it) or the CanEdit-redirect's own result (usually
// nil) — so the caller should just `return err`. Shared by every handler
// that needs "load this group, then require CanEdit" (Edit, LeaderboardFull,
// MatchesFull, RegenerateMatchArticle, RegenerateCommentary,
// EditMemberForm); RosterFull below uses rosterViewData instead since it
// also needs isOwner/ownerEmail.
func (h *GroupHandler) requireGroupEdit(c *echo.Context, userID, redirectTo string) (g *model.Group, err error, done bool) {
	g, err = h.service.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err, true
	}
	if !h.service.CanEdit(c.Request().Context(), g, userID) {
		return nil, c.Redirect(http.StatusFound, redirectTo), true
	}
	return g, nil, false
}

// RosterFull renders the Roster tab (tab bar + complete roster + activity
// feed) — hit when the Roster tab button is clicked.
func (h *GroupHandler) RosterFull(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	canEdit, _, _ := h.rosterViewData(c.Request().Context(), g, userID)
	if !canEdit {
		return c.Redirect(http.StatusFound, routeDashboard)
	}

	tabContent, err := h.groupTabComponent(c, g, userID, "roster")
	if err != nil {
		return err
	}
	return render.Component(c, tabContent)
}

// MatchesFull renders the Recent Matches tab (tab bar + complete match list
// + activity feed) — hit when the Recent Matches tab button is clicked.
func (h *GroupHandler) MatchesFull(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	g, err, done := h.requireGroupEdit(c, userID, routeDashboard)
	if done {
		return err
	}

	tabContent, err := h.groupTabComponent(c, g, userID, "matches")
	if err != nil {
		return err
	}
	return render.Component(c, tabContent)
}

func (h *GroupHandler) AddMember(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	raw := c.FormValue("name")
	names := splitNames(raw)
	if len(names) == 0 {
		return h.respondRosterMutation(c, id, "Name is required", "error")
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
			return h.respondRosterMutation(c, id, err.Error(), "error")
		}
		RecordActivity(ctx, h.repo, h.jobs, id, "player_added", memberID, name+" joined the group")

		return h.respondRosterMutation(c, id, "Added "+name, "success")
	}

	added, err := h.service.AddMembers(ctx, id, names, userID)
	if err != nil {
		return h.respondRosterMutation(c, id, err.Error(), "error")
	}

	return h.respondRosterMutation(c, id, fmt.Sprintf("Added %d players", len(added)), "success")
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
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	ctx := c.Request().Context()

	failWith := func(msg string) error {
		return h.respondRosterMutation(c, id, msg, "error")
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
	return h.respondRosterMutation(c, id, msg, "success")
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
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	memberID := c.Param("memberId")

	if err := h.service.RemoveMember(c.Request().Context(), id, memberID, userID); err != nil {
		return h.respondRosterMutation(c, id, err.Error(), "error")
	}

	return h.respondRosterMutation(c, id, "Member removed", "success")
}

func (h *GroupHandler) PromoteMember(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	memberID := c.Param("memberId")

	if err := h.service.PromoteMember(c.Request().Context(), id, memberID, userID); err != nil {
		return h.respondRosterMutation(c, id, err.Error(), "error")
	}

	return h.respondRosterMutation(c, id, "Member promoted to admin", "success")
}

func (h *GroupHandler) DemoteMember(c *echo.Context) error {
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	memberID := c.Param("memberId")

	if err := h.service.DemoteMember(c.Request().Context(), id, memberID, userID); err != nil {
		return h.respondRosterMutation(c, id, err.Error(), "error")
	}

	return h.respondRosterMutation(c, id, "Admin demoted to member", "success")
}

func (h *GroupHandler) EditMemberForm(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	id := c.Param("id")
	memberID := c.Param("memberId")

	_, err, done := h.requireGroupEdit(c, userID, "/groups/"+id)
	if done {
		return err
	}

	member, err := h.repo.GetMember(ctx, id, memberID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	return render.Component(c, groups.MemberEditForm(id, member, ""))
}

func (h *GroupHandler) UpdateMember(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
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
		return h.rosterTabWithToast(c, id, "Member updated", "success")
	}
	h.auth.Handler.SetFlash(ctx, "success", "Member updated")
	return c.Redirect(http.StatusFound, "/groups/"+id)
}

// rosterTabWithToast re-renders #detail-content-area with the Roster tab and
// a toast, restoring it after a mutation (like editing a player's details)
// that took over the whole area rather than just #roster-column — so saving
// a player edit drops the admin back into the roster listing instead of a
// full page reload.
func (h *GroupHandler) rosterTabWithToast(c *echo.Context, groupID, message, toastType string) error {
	ctx := c.Request().Context()
	userID, err := h.auth.GetUserID(ctx)
	if err != nil {
		return err
	}
	g, err := h.service.Get(ctx, groupID)
	if err != nil {
		return err
	}

	tabContent, err := h.groupTabComponent(c, g, userID, "roster")
	if err != nil {
		return err
	}

	c.Response().Header().Set(hxTrigger, toastHXTrigger(message, toastType))
	return render.Component(c, tabContent)
}

// respondRosterMutation replies to a roster mutation with an HTMX toast
// (re-rendering #roster-column) or a flash message + redirect back to the
// group page for the non-JS fallback — the "if isHTMX { toast } else {
// flash + redirect }" branch repeated after nearly every roster mutation
// handler (add/import/update/remove member, promote/demote,
// approve/reject join request).
func (h *GroupHandler) respondRosterMutation(c *echo.Context, groupID, message, toastType string) error {
	if isHTMX(c) {
		return h.rosterWithToast(c, groupID, message, toastType)
	}
	h.auth.Handler.SetFlash(c.Request().Context(), toastType, message)
	return c.Redirect(http.StatusFound, "/groups/"+groupID)
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

	c.Response().Header().Set(hxTrigger, toastHXTrigger(message, toastType))
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
	return c.Request().Header.Get(hxRequest) == "true"
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
