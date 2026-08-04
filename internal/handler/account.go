package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/josuebrunel/ezauth"
	ezauthservice "github.com/josuebrunel/ezauth/pkg/service"
	"github.com/labstack/echo/v5"

	"nutmeg/views/pages/account"
)

type AccountHandler struct {
	auth *ezauth.EzAuth
}

func NewAccountHandler(auth *ezauth.EzAuth) *AccountHandler {
	return &AccountHandler{auth: auth}
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

	user, err := h.auth.Repo.UserGetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.IsOAuth() {
		h.auth.Handler.SetFlash(ctx, "error", "Password can't be changed for an account linked to an OAuth provider")
		return c.Redirect(http.StatusFound, "/account")
	}

	currentPassword := c.FormValue("current_password")
	newPassword := c.FormValue("new_password")
	confirmPassword := c.FormValue("new_password_confirm")

	if newPassword != confirmPassword {
		h.auth.Handler.SetFlash(ctx, "error", "New passwords do not match")
		return c.Redirect(http.StatusFound, "/account")
	}

	if _, err := h.auth.Service.UserAuthenticate(ctx, ezauthservice.RequestBasicAuth{Email: user.Email, Password: currentPassword}); err != nil {
		h.auth.Handler.SetFlash(ctx, "error", "Current password is incorrect")
		return c.Redirect(http.StatusFound, "/account")
	}

	if _, err := h.auth.Service.UserUpdatePassword(ctx, user, newPassword); err != nil {
		h.auth.Handler.SetFlash(ctx, "error", err.Error())
		return c.Redirect(http.StatusFound, "/account")
	}

	h.auth.Handler.SetFlash(ctx, "success", "Password updated")
	return c.Redirect(http.StatusFound, "/account")
}
