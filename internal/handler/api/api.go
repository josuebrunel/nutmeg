// Package api implements Nutmeg's JSON API (/api/v1), documented with
// swaggo annotations. It mirrors internal/handler's resource split (one
// handler type per resource) but returns JSON instead of rendering Templ
// views, and authenticates via ezauth's JWT Bearer middleware instead of
// the cookie session used by the HTML app.
package api

import (
	"github.com/josuebrunel/ezauth"

	"nutmeg/internal/handler"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
)

type Handler struct {
	Home    *HomeHandler
	Account *AccountHandler
	Group   *GroupHandler
	Match   *MatchHandler
}

func New(auth *ezauth.EzAuth, repo *repository.Repository, commentarySvc *service.CommentaryService, jobs handler.JobEnqueuer) *Handler {
	groupSvc := service.NewGroupService(repo, auth.Repo)
	matchSvc := service.NewMatchService(repo, repo)
	return &Handler{
		Home:    &HomeHandler{auth: auth, groupSvc: groupSvc, matchSvc: matchSvc},
		Account: &AccountHandler{auth: auth},
		Group:   NewGroupHandler(auth, groupSvc, matchSvc, repo, commentarySvc, jobs),
		Match:   NewMatchHandler(auth, matchSvc, repo, jobs),
	}
}
