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
	auth  *ezauth.EzAuth
	repo  *repository.Repository
	Home  *HomeHandler
	Auth  *AuthHandler
	Group *GroupHandler
	Match *MatchHandler
}

func New(auth *ezauth.EzAuth, repo *repository.Repository) *Handler {
	groupSvc := service.NewGroupService(repo)
	matchSvc := service.NewMatchService(repo, repo)
	return &Handler{
		auth:  auth,
		repo:  repo,
		Home:  &HomeHandler{groupSvc: groupSvc, auth: auth, matchSvc: matchSvc},
		Auth:  NewAuthHandler(auth),
		Group: NewGroupHandler(auth, groupSvc, matchSvc, repo),
		Match: NewMatchHandler(auth, matchSvc, repo),
	}
}

func page(c *echo.Context, title string, isLoggedIn bool, currentGroupID string, userName string, cmp templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTML)
	ctx := templ.WithChildren(c.Request().Context(), cmp)
	return layout.Base(title, isLoggedIn, currentGroupID, userName).Render(ctx, c.Response())
}
