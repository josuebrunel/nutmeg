package handler

import (
	"github.com/a-h/templ"
	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/views/layout"
)

type Handler struct {
	auth *ezauth.EzAuth
	repo *repository.Repository
	Home *HomeHandler
	Auth *AuthHandler
	Group *GroupHandler
}

func New(auth *ezauth.EzAuth, repo *repository.Repository) *Handler {
	groupSvc := service.NewGroupService(repo)
	return &Handler{
		auth:  auth,
		repo:  repo,
		Home:  &HomeHandler{},
		Auth:  NewAuthHandler(auth),
		Group: NewGroupHandler(auth, groupSvc),
	}
}

func page(c *echo.Context, title string, isLoggedIn bool, currentGroupID string, cmp templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTML)
	ctx := templ.WithChildren(c.Request().Context(), cmp)
	return layout.Base(title, isLoggedIn, currentGroupID).Render(ctx, c.Response())
}
