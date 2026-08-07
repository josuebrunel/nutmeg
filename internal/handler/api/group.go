package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/handler"
	"nutmeg/internal/model"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/internal/worker"
)

type GroupHandler struct {
	auth          *ezauth.EzAuth
	service       *service.GroupService
	matchSvc      *service.MatchService
	repo          *repository.Repository
	commentarySvc *service.CommentaryService
	jobs          handler.JobEnqueuer
}

func NewGroupHandler(auth *ezauth.EzAuth, svc *service.GroupService, matchSvc *service.MatchService, repo *repository.Repository, commentarySvc *service.CommentaryService, jobs handler.JobEnqueuer) *GroupHandler {
	return &GroupHandler{auth: auth, service: svc, matchSvc: matchSvc, repo: repo, commentarySvc: commentarySvc, jobs: jobs}
}

// List returns every group the caller owns or administers.
// @Summary List groups
// @Tags groups
// @Produce json
// @Security BearerAuth
// @Success 200 {array} GroupResponse
// @Failure 401 {object} ErrorResponse
// @Router /groups [get]
func (h *GroupHandler) List(c *echo.Context) error {
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	list, err := h.service.List(c.Request().Context(), userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toGroupResponses(list))
}

// Create creates a new group owned by the authenticated user.
// @Summary Create a group
// @Tags groups
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body GroupCreateRequest true "Group"
// @Success 201 {object} GroupResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /groups [post]
func (h *GroupHandler) Create(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	var req GroupCreateRequest
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return writeError(c, model.ErrInvalidInput)
	}

	user, err := h.auth.Repo.UserGetByID(ctx, userID)
	if err != nil {
		return writeError(c, err)
	}

	g, err := h.service.Create(ctx, req.Name, nil, userID, user.DisplayName(), user.Email)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusCreated, toGroupResponse(g))
}

// Get returns a single group by id. Owner or promoted admin only — use the
// public leaderboard/player-profile endpoints for unauthenticated access.
// @Summary Get a group
// @Tags groups
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Success 200 {object} GroupResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id} [get]
func (h *GroupHandler) Get(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	g, err := h.service.Get(ctx, c.Param("id"))
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	if !h.service.CanEdit(ctx, g, userID) {
		return writeError(c, model.ErrNotAuthorized)
	}
	return c.JSON(http.StatusOK, toGroupResponse(g))
}

// Update edits a group's name/description. Owner or promoted admin only.
// @Summary Update a group
// @Tags groups
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param request body GroupUpdateRequest true "Group fields"
// @Success 200 {object} GroupResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id} [patch]
func (h *GroupHandler) Update(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	var req GroupUpdateRequest
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return writeError(c, model.ErrInvalidInput)
	}

	g, err := h.service.Get(ctx, c.Param("id"))
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	g.Name = req.Name
	g.Description = req.Description

	if err := h.service.Update(ctx, g, userID); err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toGroupResponse(g))
}

// Delete removes a group. Owner only.
// @Summary Delete a group
// @Tags groups
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Success 204 "no content"
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id} [delete]
func (h *GroupHandler) Delete(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	if err := h.service.Delete(ctx, c.Param("id"), userID); err != nil {
		return writeError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListMembers returns a group's roster, including each member's phone/email.
// Owner or promoted admin only.
// @Summary List group members
// @Tags groups
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Success 200 {array} MemberResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/members [get]
func (h *GroupHandler) ListMembers(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	g, err := h.service.Get(ctx, c.Param("id"))
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	if !h.service.CanEdit(ctx, g, userID) {
		return writeError(c, model.ErrNotAuthorized)
	}

	members, err := h.service.Members(ctx, g.ID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toMemberResponses(members))
}

// AddMember adds one or more roster members to a group. Phone/email only
// apply when adding a single member (see MemberAddRequest).
// @Summary Add group member(s)
// @Tags groups
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param request body MemberAddRequest true "Member(s) to add"
// @Success 201 {array} MemberResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/members [post]
func (h *GroupHandler) AddMember(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")

	var req MemberAddRequest
	if err := c.Bind(&req); err != nil || len(req.Names) == 0 {
		return writeError(c, model.ErrInvalidInput)
	}

	var singleID string
	if len(req.Names) == 1 {
		memberID, err := h.service.AddMember(ctx, groupID, req.Names[0], req.Phone, req.Email, req.Position, userID)
		if err != nil {
			return writeError(c, err)
		}
		handler.RecordNews(ctx, h.repo, h.jobs, groupID, "player_added", memberID, req.Names[0]+" joined the group")
		singleID = memberID
	} else if _, err := h.service.AddMembers(ctx, groupID, req.Names, userID); err != nil {
		return writeError(c, err)
	}

	members, err := h.service.Members(ctx, groupID)
	if err != nil {
		return writeError(c, err)
	}

	if singleID != "" {
		for _, m := range members {
			if m.ID == singleID {
				return c.JSON(http.StatusCreated, []MemberResponse{toMemberResponse(m)})
			}
		}
	}

	nameSet := make(map[string]bool, len(req.Names))
	for _, n := range req.Names {
		nameSet[n] = true
	}
	var out []MemberResponse
	for _, m := range members {
		if nameSet[m.Name] {
			out = append(out, toMemberResponse(m))
		}
	}
	return c.JSON(http.StatusCreated, out)
}

// ImportMembers bulk-adds or updates roster members — the JSON equivalent
// of the HTML roster's CSV upload.
// @Summary Import members
// @Tags groups
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param request body MemberImportRequest true "Rows to import"
// @Success 200 {object} MemberImportResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /groups/{id}/members/import [post]
func (h *GroupHandler) ImportMembers(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")

	var req MemberImportRequest
	if err := c.Bind(&req); err != nil || len(req.Rows) == 0 {
		return writeError(c, model.ErrInvalidInput)
	}

	rows := make([]service.ImportRow, len(req.Rows))
	for i, r := range req.Rows {
		rows[i] = service.ImportRow{Name: r.Name, Phone: r.Phone, Email: r.Email, Position: r.Position}
	}

	imported, updated, skipped, err := h.service.ImportMembers(ctx, groupID, rows, userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, MemberImportResponse{Imported: imported, Updated: updated, Skipped: skipped})
}

// UpdateMember edits a roster member's name/phone/email.
// @Summary Update a group member
// @Tags groups
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param memberId path string true "Member ID"
// @Param request body MemberUpdateRequest true "Member fields"
// @Success 200 {object} MemberResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /groups/{id}/members/{memberId} [patch]
func (h *GroupHandler) UpdateMember(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")
	memberID := c.Param("memberId")

	var req MemberUpdateRequest
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return writeError(c, model.ErrInvalidInput)
	}

	if err := h.service.UpdateMember(ctx, groupID, memberID, req.Name, req.Phone, req.Email, req.Position, userID); err != nil {
		return writeError(c, err)
	}

	member, err := h.repo.GetMember(ctx, groupID, memberID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toMemberResponse(repository.MemberInfo{
		ID: member.ID, Name: member.Name, Phone: member.Phone, Email: member.Email, Role: member.Role, Position: member.Position, JoinedAt: member.JoinedAt,
	}))
}

// RemoveMember removes a roster member from a group.
// @Summary Remove a group member
// @Tags groups
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param memberId path string true "Member ID"
// @Success 204 "no content"
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/members/{memberId} [delete]
func (h *GroupHandler) RemoveMember(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	if err := h.service.RemoveMember(ctx, c.Param("id"), c.Param("memberId"), userID); err != nil {
		return writeError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// PromoteMember grants a roster member admin rights on the group. Owner only.
// @Summary Promote a member to admin
// @Tags groups
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param memberId path string true "Member ID"
// @Success 204 "no content"
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/members/{memberId}/promote [post]
func (h *GroupHandler) PromoteMember(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	if err := h.service.PromoteMember(ctx, c.Param("id"), c.Param("memberId"), userID); err != nil {
		return writeError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// DemoteMember reverses PromoteMember. Owner only.
// @Summary Demote an admin to member
// @Tags groups
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param memberId path string true "Member ID"
// @Success 204 "no content"
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/members/{memberId}/demote [post]
func (h *GroupHandler) DemoteMember(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	if err := h.service.DemoteMember(ctx, c.Param("id"), c.Param("memberId"), userID); err != nil {
		return writeError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListJoinRequests lists a group's pending join requests. Admin only.
// @Summary List join requests
// @Tags groups
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Success 200 {array} JoinRequestResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/join-requests [get]
func (h *GroupHandler) ListJoinRequests(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")

	g, err := h.service.Get(ctx, groupID)
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	if !h.service.CanEdit(ctx, g, userID) {
		return writeError(c, model.ErrNotAuthorized)
	}

	requests, err := h.service.JoinRequests(ctx, groupID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toJoinRequestResponses(requests))
}

// RequestJoin submits a request for the authenticated user to join a group.
// @Summary Request to join a group
// @Tags groups
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Success 204 "no content"
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /groups/{id}/join-requests [post]
func (h *GroupHandler) RequestJoin(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")

	g, err := h.service.Get(ctx, groupID)
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	if err := h.service.RequestToJoin(ctx, g, userID); err != nil {
		return writeError(c, err)
	}
	handler.EnqueueEmail(ctx, h.jobs, h.adminEmails(ctx, groupID), "New join request for "+g.Name,
		"Someone requested to join "+g.Name+". Review the request from the group's roster page.")
	return c.NoContent(http.StatusNoContent)
}

// adminEmails returns the email addresses of a group's admin members
// (includes the creator, who is added as an "admin"-role member at group
// creation — see GroupService.Create).
func (h *GroupHandler) adminEmails(ctx context.Context, groupID string) []string {
	members, err := h.repo.ListMembers(ctx, groupID)
	if err != nil {
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

// ApproveJoinRequest approves a pending join request, adding the requester
// to the roster.
// @Summary Approve a join request
// @Tags groups
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param reqId path string true "Join request ID"
// @Success 204 "no content"
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/join-requests/{reqId}/approve [post]
func (h *GroupHandler) ApproveJoinRequest(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")

	g, err := h.service.Get(ctx, groupID)
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	req, err := h.service.ApproveJoinRequest(ctx, g, c.Param("reqId"), userID)
	if err != nil {
		return writeError(c, err)
	}
	handler.EnqueueEmail(ctx, h.jobs, []string{req.Email}, "You're in! Welcome to "+g.Name,
		"Your request to join "+g.Name+" was approved. Head to the group page to see the roster and leaderboard.")
	return c.NoContent(http.StatusNoContent)
}

// RejectJoinRequest rejects a pending join request.
// @Summary Reject a join request
// @Tags groups
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param reqId path string true "Join request ID"
// @Success 204 "no content"
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/join-requests/{reqId}/reject [post]
func (h *GroupHandler) RejectJoinRequest(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")

	g, err := h.service.Get(ctx, groupID)
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	req, err := h.service.RejectJoinRequest(ctx, g, c.Param("reqId"), userID)
	if err != nil {
		return writeError(c, err)
	}
	handler.EnqueueEmail(ctx, h.jobs, []string{req.Email}, "Update on your request to join "+g.Name,
		"Your request to join "+g.Name+" was not approved this time.")
	return c.NoContent(http.StatusNoContent)
}

// Leaderboard returns a group's leaderboard, optionally sorted by a field.
// Registered under both the authenticated route table and the public one
// (see internal/router/api.go), since the data itself isn't scoped to the
// viewer — same as the HTML app's PublicLeaderboard page.
// @Summary Get a group's leaderboard
// @Tags groups
// @Produce json
// @Param id path string true "Group ID"
// @Param sort query string false "Sort field"
// @Success 200 {array} LeaderboardEntryResponse
// @Failure 404 {object} ErrorResponse
// @Router /groups/{id}/leaderboard [get]
func (h *GroupHandler) Leaderboard(c *echo.Context) error {
	ctx := c.Request().Context()
	groupID := c.Param("id")

	if _, err := h.service.Get(ctx, groupID); err != nil {
		return writeError(c, model.ErrNotFound)
	}
	entries, err := h.matchSvc.GetLeaderboard(ctx, groupID, c.QueryParam("sort"))
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toLeaderboardResponses(entries))
}

// RegenerateCommentary lets a group admin manually re-run roast generation
// for a single player, gated by CanEdit and a per-player cooldown.
// @Summary Regenerate player commentary
// @Tags groups
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param memberId path string true "Member ID"
// @Success 202 "accepted"
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Router /groups/{id}/players/{memberId}/regenerate-commentary [post]
func (h *GroupHandler) RegenerateCommentary(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	groupID := c.Param("id")
	memberID := c.Param("memberId")

	g, err := h.service.Get(ctx, groupID)
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	if !h.service.CanEdit(ctx, g, userID) {
		return writeError(c, model.ErrNotAuthorized)
	}

	ok, wait, err := h.commentarySvc.CanRegenerate(ctx, memberID)
	if err != nil {
		return writeError(c, err)
	}
	if !ok {
		return c.JSON(http.StatusTooManyRequests, ErrorResponse{
			Error: fmt.Sprintf("too soon — try again in %d minute(s)", int(wait.Minutes())+1),
		})
	}

	if _, err := h.jobs.Insert(ctx, worker.GenerateCommentaryArgs{GroupPlayerID: memberID}, nil); err != nil {
		slog.Error("enqueue commentary regeneration failed", "group_player_id", memberID, "error", err)
		return writeError(c, err)
	}
	return c.NoContent(http.StatusAccepted)
}

// PlayerProfile returns a single player's public stats — no authentication
// required, mirroring the HTML app's public player-profile page.
// @Summary Get a player's public profile
// @Tags public
// @Produce json
// @Param id path string true "Group ID"
// @Param memberId path string true "Member ID"
// @Success 200 {object} PlayerProfileResponse
// @Failure 404 {object} ErrorResponse
// @Router /public/groups/{id}/players/{memberId} [get]
func (h *GroupHandler) PlayerProfile(c *echo.Context) error {
	ctx := c.Request().Context()
	groupID := c.Param("id")
	memberID := c.Param("memberId")

	g, err := h.service.Get(ctx, groupID)
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	player, err := h.repo.GetMember(ctx, g.ID, memberID)
	if err != nil {
		return writeError(c, model.ErrNotFound)
	}
	stats, err := h.matchSvc.GetPlayerStats(ctx, player.ID)
	if err != nil {
		return writeError(c, err)
	}

	var commentary *string
	if pc, err := h.repo.GetActivePlayerCommentary(ctx, player.ID); err == nil && pc != nil {
		commentary = &pc.Content
	}

	return c.JSON(http.StatusOK, PlayerProfileResponse{
		ID:            player.ID,
		Name:          player.Name,
		GroupID:       g.ID,
		MatchesPlayed: stats.MatchesPlayed,
		Wins:          stats.Wins,
		Draws:         stats.Draws,
		Losses:        stats.Losses,
		Goals:         stats.Goals,
		Assists:       stats.Assists,
		Commentary:    commentary,
	})
}
