package handler

import (
	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/views/pages/auth"
)

type AuthHandler struct {
	auth *ezauth.EzAuth
}

func NewAuthHandler(auth *ezauth.EzAuth) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Login(c *echo.Context) error {
	errMsg := h.auth.GetErrorMessage(c.Request().Context())
	sucMsg := h.auth.GetSuccessMessage(c.Request().Context())
	return page(c, "Sign In - Soccer Stats", false, "", auth.Login(errMsg, sucMsg))
}

func (h *AuthHandler) Register(c *echo.Context) error {
	errMsg := h.auth.GetErrorMessage(c.Request().Context())
	sucMsg := h.auth.GetSuccessMessage(c.Request().Context())
	return page(c, "Sign Up - Soccer Stats", false, "", auth.Register(errMsg, sucMsg))
}
