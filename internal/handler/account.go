package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/model"
	"nutmeg/internal/service"
	"nutmeg/views/pages/account"
)

type AccountHandler struct {
	auth       *ezauth.EzAuth
	accountSvc *service.AccountService
}

func NewAccountHandler(auth *ezauth.EzAuth, accountSvc *service.AccountService) *AccountHandler {
	return &AccountHandler{auth: auth, accountSvc: accountSvc}
}

func (h *AccountHandler) Edit(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	user, err := h.auth.Repo.UserGetByID(ctx, userID)
	if err != nil {
		return err
	}

	successMsg := h.auth.GetSuccessMessage(ctx)
	errMsg := h.auth.GetErrorMessage(ctx)

	return page(c, "My Account", true, "", user.DisplayName(), account.Form(user, successMsg, errMsg))
}

func (h *AccountHandler) Update(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	user, err := h.auth.Repo.UserGetByID(ctx, userID)
	if err != nil {
		return err
	}

	firstName := c.FormValue("first_name")
	lastName := c.FormValue("last_name")
	username := strings.TrimSpace(c.FormValue("username"))
	email := strings.TrimSpace(c.FormValue("email"))

	if email == "" {
		h.auth.Handler.SetFlash(ctx, "error", "Email is required")
		return c.Redirect(http.StatusFound, "/account")
	}

	if email != user.Email {
		if existing, err := h.auth.Repo.UserGetByEmail(ctx, email); err == nil && existing.ID != user.ID {
			h.auth.Handler.SetFlash(ctx, "error", "That email is already in use")
			return c.Redirect(http.StatusFound, "/account")
		}
	}
	if username != "" && username != user.Username {
		if existing, err := h.auth.Repo.UserGetByUsername(ctx, username); err == nil && existing.ID != user.ID {
			h.auth.Handler.SetFlash(ctx, "error", "That username is already taken")
			return c.Redirect(http.StatusFound, "/account")
		}
	}

	user.FirstName = firstName
	user.LastName = lastName
	user.Username = username
	if email != user.Email {
		user.Email = email
		user.EmailVerified = false
		user.EmailVerifiedAt = nil
	}

	if _, err := h.auth.Repo.UserUpdate(ctx, user); err != nil {
		slog.Error("failed to update account", "user_id", userID, "error", err)
		h.auth.Handler.SetFlash(ctx, "error", "Could not update your info. Please try again.")
		return c.Redirect(http.StatusFound, "/account")
	}

	h.auth.Handler.SetFlash(ctx, "success", "Your info was updated")
	return c.Redirect(http.StatusFound, "/account")
}

func (h *AccountHandler) UpdatePassword(c *echo.Context) error {
	ctx := c.Request().Context()
	userID, done := requireUserID(c, h.auth)
	if done {
		return nil
	}

	err := h.accountSvc.ChangePassword(ctx, userID,
		c.FormValue("current_password"), c.FormValue("new_password"), c.FormValue("new_password_confirm"))
	switch err {
	case nil:
		h.auth.Handler.SetFlash(ctx, "success", "Password updated")
	case model.ErrOAuthPasswordChange, model.ErrPasswordMismatch:
		h.auth.Handler.SetFlash(ctx, "error", err.Error())
	case model.ErrCurrentPasswordIncorrect:
		slog.Warn("password change: current password incorrect", "user_id", userID)
		h.auth.Handler.SetFlash(ctx, "error", err.Error())
	default:
		slog.Error("failed to update password", "user_id", userID, "error", err)
		h.auth.Handler.SetFlash(ctx, "error", "Could not update your password. Please try again.")
	}
	return c.Redirect(http.StatusFound, "/account")
}
