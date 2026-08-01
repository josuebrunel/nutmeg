package handler

import (
	"context"

	"github.com/a-h/templ"
	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/views/layout"
)

// JobEnqueuer is satisfied by *river.Client[*sql.Tx] — declared narrowly
// here (just the one method handlers need) rather than threading the
// concrete generic River client type through handler constructors.
type JobEnqueuer interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

type Handler struct {
	auth    *ezauth.EzAuth
	repo    *repository.Repository
	Home    *HomeHandler
	Auth    *AuthHandler
	Account *AccountHandler
	Group   *GroupHandler
	Match   *MatchHandler
}

func New(auth *ezauth.EzAuth, repo *repository.Repository, commentarySvc *service.CommentaryService, jobs JobEnqueuer) *Handler {
	groupSvc := service.NewGroupService(repo, auth.Repo)
	matchSvc := service.NewMatchService(repo, repo)
	return &Handler{
		auth:    auth,
		repo:    repo,
		Home:    &HomeHandler{groupSvc: groupSvc, auth: auth, matchSvc: matchSvc},
		Auth:    NewAuthHandler(auth),
		Account: NewAccountHandler(auth),
		Group:   NewGroupHandler(auth, groupSvc, matchSvc, repo, commentarySvc, jobs),
		Match:   NewMatchHandler(auth, matchSvc, repo, jobs),
	}
}

func page(c *echo.Context, title string, isLoggedIn bool, currentGroupID string, userName string, cmp templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTML)
	ctx := templ.WithChildren(c.Request().Context(), cmp)
	return layout.Base(title, isLoggedIn, currentGroupID, userName).Render(ctx, c.Response())
}
