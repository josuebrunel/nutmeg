package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"nutmeg/internal/repository"
	"nutmeg/internal/service"
	"nutmeg/internal/worker"
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

func New(auth *ezauth.EzAuth, repo *repository.Repository, commentarySvc *service.CommentaryService, newsSvc *service.NewsService, jobs JobEnqueuer) *Handler {
	groupSvc := service.NewGroupService(repo, auth.Repo)
	matchSvc := service.NewMatchService(repo, repo)
	accountSvc := service.NewAccountService(auth.Repo, auth.Service)
	return &Handler{
		auth:    auth,
		repo:    repo,
		Home:    &HomeHandler{groupSvc: groupSvc, auth: auth, matchSvc: matchSvc},
		Auth:    NewAuthHandler(auth),
		Account: NewAccountHandler(auth, accountSvc),
		Group:   NewGroupHandler(auth, groupSvc, matchSvc, repo, commentarySvc, newsSvc, jobs),
		Match:   NewMatchHandler(auth, matchSvc, repo, jobs),
	}
}

// defaultDescription is used by every page except those that opt into a
// more specific one via pageWithMeta (currently the public leaderboard and
// player profile pages, whose link-preview description should reflect the
// group/player being shared rather than this generic site blurb).
const defaultDescription = "Nutmeg is a free, self-hosted stats tracker for pickup soccer groups — split into teams, log matches, and keep a standing leaderboard of wins, goals, and assists."

// siteBaseURL is set once at startup via SetBaseURL — every page shares the
// same og:image, so this avoids threading a base-URL parameter through
// every page(...) call site in the package.
var siteBaseURL string

// SetBaseURL configures the absolute base URL used to build the og:image
// meta tag. Must be called once during startup, before the server accepts
// requests.
func SetBaseURL(url string) {
	siteBaseURL = url
}

// requireUserID resolves the current session's user id, redirecting to
// /login and reporting done=true if there's no valid session — the
// "get user id or redirect" guard repeated at the top of every
// authenticated handler.
func requireUserID(c *echo.Context, auth *ezauth.EzAuth) (userID string, done bool) {
	userID, err := auth.GetUserID(c.Request().Context())
	if err != nil {
		c.Redirect(http.StatusFound, routeLogin)
		return "", true
	}
	return userID, false
}

// logUnexpected logs err at Error if it's an operational failure — i.e. it
// wraps a lower-level cause via fmt.Errorf("...: %w", err), per the
// discipline internal/service/{group,match}.go follow for real repo/DB
// failures. Flat errors (model's sentinel errors, hand-written validation
// errors) are normal application flow already shown to the user via a
// flash message or toast, and are deliberately not logged here.
func logUnexpected(msg string, err error, args ...any) {
	if errors.Unwrap(err) == nil {
		return
	}
	slog.Error(msg, append(args, "error", err)...)
}

func page(c *echo.Context, title string, isLoggedIn bool, currentGroupID string, userName string, cmp templ.Component) error {
	return pageWithMeta(c, title, defaultDescription, isLoggedIn, currentGroupID, userName, cmp)
}

func pageWithMeta(c *echo.Context, title, description string, isLoggedIn bool, currentGroupID string, userName string, cmp templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTML)
	ctx := templ.WithChildren(c.Request().Context(), cmp)
	ogImage := siteBaseURL + "/static/img/logo.png"
	return layout.Base(title, description, ogImage, isLoggedIn, currentGroupID, userName).Render(ctx, c.Response())
}

// RecordNews inserts a new group_news row with fallback content already in
// place, then enqueues a background job to upgrade it with AI-generated
// copy. Shared by GroupHandler (player_added) and MatchHandler
// (match_logged) — both hold a *repository.Repository and a JobEnqueuer
// already. Failures are logged and swallowed, same as enqueueCommentary: a
// group's news feed is never allowed to block the request that triggered
// it.
func RecordNews(ctx context.Context, repo *repository.Repository, jobs JobEnqueuer, groupID, kind, subjectID, fallback string) {
	newsID, err := repo.CreateGroupNews(ctx, groupID, kind, subjectID, fallback)
	if err != nil {
		slog.Error("record group news failed", "group_id", groupID, "kind", kind, "error", err)
		return
	}
	res, err := jobs.Insert(ctx, worker.GenerateGroupNewsArgs{NewsID: newsID, EventKind: kind, SubjectID: subjectID}, nil)
	if err != nil {
		slog.Error("enqueue group news generation failed", "news_id", newsID, "error", err)
		return
	}
	slog.Info("enqueued group news generation", "news_id", newsID, "kind", kind, "job_id", res.Job.ID)
}

// EnqueueEmail enqueues a background job to send an email to one or more
// recipients — a no-op (with no error) if to is empty, e.g. a group with
// no admin email on file. Same log-and-continue discipline as
// RecordNews: never blocks or fails the request that triggered it.
func EnqueueEmail(ctx context.Context, jobs JobEnqueuer, to []string, subject, body string) {
	if len(to) == 0 {
		return
	}
	res, err := jobs.Insert(ctx, worker.SendEmailArgs{To: to, Subject: subject, Body: body}, nil)
	if err != nil {
		slog.Error("enqueue email failed", "to", to, "subject", subject, "error", err)
		return
	}
	slog.Info("enqueued email", "to", to, "subject", subject, "job_id", res.Job.ID)
}
