package api

import (
	"net/http"
	"strings"

	"github.com/josuebrunel/ezauth"
	ezauthservice "github.com/josuebrunel/ezauth/pkg/service"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/model"
)

type AccountHandler struct {
	auth *ezauth.EzAuth
}

// Get returns the authenticated user's account info.
// @Summary Get account
// @Tags account
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AccountResponse
// @Failure 401 {object} ErrorResponse
// @Router /account [get]
func (h *AccountHandler) Get(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	user, err := h.auth.Repo.UserGetByID(ctx, userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toAccountResponse(user))
}

// Update edits the authenticated user's account info.
// @Summary Update account
// @Tags account
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body AccountUpdateRequest true "Account fields"
// @Success 200 {object} AccountResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /account [patch]
func (h *AccountHandler) Update(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	user, err := h.auth.Repo.UserGetByID(ctx, userID)
	if err != nil {
		return writeError(c, err)
	}

	var req AccountUpdateRequest
	if err := c.Bind(&req); err != nil {
		return writeError(c, model.ErrInvalidInput)
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return writeError(c, model.ErrInvalidInput)
	}

	if email != user.Email {
		if existing, err := h.auth.Repo.UserGetByEmail(ctx, email); err == nil && existing.ID != user.ID {
			return writeError(c, model.ErrAlreadyExists)
		}
	}
	username := strings.TrimSpace(req.Username)
	if username != "" && username != user.Username {
		if existing, err := h.auth.Repo.UserGetByUsername(ctx, username); err == nil && existing.ID != user.ID {
			return writeError(c, model.ErrAlreadyExists)
		}
	}

	user.FirstName = req.FirstName
	user.LastName = req.LastName
	user.Username = username
	if email != user.Email {
		user.Email = email
		user.EmailVerified = false
		user.EmailVerifiedAt = nil
	}

	updated, err := h.auth.Repo.UserUpdate(ctx, user)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(http.StatusOK, toAccountResponse(updated))
}

// UpdatePassword changes the authenticated user's password.
// @Summary Change password
// @Tags account
// @Accept json
// @Security BearerAuth
// @Param request body PasswordChangeRequest true "Password change"
// @Success 204 "no content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /account/password [post]
func (h *AccountHandler) UpdatePassword(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, err := currentUserID(c, h.auth)
	if err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}

	user, err := h.auth.Repo.UserGetByID(ctx, userID)
	if err != nil {
		return writeError(c, err)
	}
	if user.IsOAuth() {
		return writeError(c, model.ErrInvalidInput)
	}

	var req PasswordChangeRequest
	if err := c.Bind(&req); err != nil {
		return writeError(c, model.ErrInvalidInput)
	}

	if _, err := h.auth.Service.UserAuthenticate(ctx, ezauthservice.RequestBasicAuth{Email: user.Email, Password: req.CurrentPassword}); err != nil {
		return writeError(c, model.ErrNotAuthorized)
	}
	if _, err := h.auth.Service.UserUpdatePassword(ctx, user, req.NewPassword); err != nil {
		return writeError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
