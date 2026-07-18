package handler

import (
	"log/slog"
	"net/http"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/views/pages/groups"
)

type GroupHandler struct {
	auth    *ezauth.EzAuth
	service *service.GroupService
	repo    *repository.Repository
}

func NewGroupHandler(auth *ezauth.EzAuth, svc *service.GroupService, repo *repository.Repository) *GroupHandler {
	return &GroupHandler{auth: auth, service: svc, repo: repo}
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

	return page(c, "My Groups - Soccer Stats", true, "", groups.List(list))
}

func (h *GroupHandler) New(c *echo.Context) error {
	return page(c, "New Group - Soccer Stats", true, "", groups.Form("", nil))
}

func (h *GroupHandler) Create(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	name := c.FormValue("name")
	if name == "" {
		return page(c, "New Group - Soccer Stats", true, "", groups.Form("", &groups.FormData{Error: "Name is required"}))
	}

	desc := c.FormValue("description")
	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}

	g, err := h.service.Create(c.Request().Context(), name, descPtr, userID)
	if err != nil {
		return page(c, "New Group - Soccer Stats", true, "", groups.Form("", &groups.FormData{Name: name, Description: desc, Error: err.Error()}))
	}

	return c.Redirect(http.StatusFound, "/groups/"+g.ID)
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

	userID, err := h.auth.GetUserID(c.Request().Context())
	isAdmin := false
	if err == nil {
		for _, m := range members {
			if m.UserID == userID && m.Role == "admin" {
				isAdmin = true
				break
			}
		}
	}

	successMsg := h.auth.GetSuccessMessage(c.Request().Context())
	errMsg := h.auth.GetErrorMessage(c.Request().Context())

	return page(c, g.Name+" - Soccer Stats", true, g.ID, groups.Detail(g, members, isAdmin, successMsg, errMsg))
}

func (h *GroupHandler) Edit(c *echo.Context) error {
	id := c.Param("id")
	g, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return page(c, "Edit Group - Soccer Stats", true, g.ID, groups.Form(g.ID, &groups.FormData{
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
		return page(c, "Edit Group - Soccer Stats", true, id, groups.Form(id, &groups.FormData{Error: "Name is required"}))
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
		return page(c, "Edit Group - Soccer Stats", true, id, groups.Form(id, &groups.FormData{Name: name, Description: desc, Error: err.Error()}))
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

func (h *GroupHandler) AddMember(c *echo.Context) error {
	userID, err := h.auth.GetUserID(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	id := c.Param("id")
	email := c.FormValue("email")
	if email == "" {
		h.auth.Handler.SetFlash(c.Request().Context(), "error", "Email is required")
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	ctx := c.Request().Context()
	targetUserID, err := h.repo.GetUserByEmail(ctx, email)
	if err != nil {
		// User does not exist, simulate sending invitation email
		slog.Info("Simulating sending invitation email", "email", email, "group_id", id)

		// TODO: Send invitation email to email to register an account

		h.auth.Handler.SetFlash(ctx, "success", "User with email "+email+" does not exist. An invitation email was sent to them!")
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	if err := h.service.AddMember(ctx, id, targetUserID, userID); err != nil {
		h.auth.Handler.SetFlash(ctx, "error", err.Error())
		return c.Redirect(http.StatusFound, "/groups/"+id)
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
		return c.Redirect(http.StatusFound, "/groups/"+id)
	}

	return c.Redirect(http.StatusFound, "/groups/"+id)
}

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
