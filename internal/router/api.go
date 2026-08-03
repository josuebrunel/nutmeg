package router

import (
	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/handler"
	"nutmeg/internal/handler/api"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
)

// RegisterAPI registers the authenticated JSON API (/api/v1) — a
// JSON-ified, full-CRUD mirror of the HTML app's authenticated route table
// in Register above. app is expected to already have auth.AuthMiddleware
// (JWT Bearer) applied, not the cookie SessionMiddleware.
func RegisterAPI(app *echo.Group, auth *ezauth.EzAuth, repo *repository.Repository, commentarySvc *service.CommentaryService, jobs handler.JobEnqueuer) {
	h := api.New(auth, repo, commentarySvc, jobs)

	app.GET("/dashboard", h.Home.Dashboard)
	app.GET("/stats", h.Home.Stats)

	app.GET("/account", h.Account.Get)
	app.PATCH("/account", h.Account.Update)
	app.POST("/account/password", h.Account.UpdatePassword)

	app.GET("/groups", h.Group.List)
	app.POST("/groups", h.Group.Create)
	app.GET("/groups/:id", h.Group.Get)
	app.PATCH("/groups/:id", h.Group.Update)
	app.DELETE("/groups/:id", h.Group.Delete)

	app.GET("/groups/:id/members", h.Group.ListMembers)
	app.POST("/groups/:id/members", h.Group.AddMember)
	app.POST("/groups/:id/members/import", h.Group.ImportMembers)
	app.PATCH("/groups/:id/members/:memberId", h.Group.UpdateMember)
	app.DELETE("/groups/:id/members/:memberId", h.Group.RemoveMember)
	app.POST("/groups/:id/members/:memberId/promote", h.Group.PromoteMember)
	app.POST("/groups/:id/members/:memberId/demote", h.Group.DemoteMember)

	app.GET("/groups/:id/join-requests", h.Group.ListJoinRequests)
	app.POST("/groups/:id/join-requests", h.Group.RequestJoin)
	app.POST("/groups/:id/join-requests/:reqId/approve", h.Group.ApproveJoinRequest)
	app.POST("/groups/:id/join-requests/:reqId/reject", h.Group.RejectJoinRequest)

	app.GET("/groups/:id/leaderboard", h.Group.Leaderboard)
	app.POST("/groups/:id/players/:memberId/regenerate-commentary", h.Group.RegenerateCommentary)

	app.GET("/groups/:id/matches", h.Match.List)
	app.POST("/groups/:id/matches", h.Match.Create)
	app.GET("/groups/:id/matches/:mid", h.Match.Get)
	app.PATCH("/groups/:id/matches/:mid", h.Match.Update)
	app.DELETE("/groups/:id/matches/:mid", h.Match.Delete)
}

// RegisterPublicAPI registers the unauthenticated read-only JSON endpoints —
// a group's leaderboard and a single player's public profile — mirroring
// the HTML app's public leaderboard/player-profile pages. No auth
// middleware should be applied to pub.
func RegisterPublicAPI(pub *echo.Group, auth *ezauth.EzAuth, repo *repository.Repository, commentarySvc *service.CommentaryService, jobs handler.JobEnqueuer) {
	h := api.New(auth, repo, commentarySvc, jobs)

	pub.GET("/groups/:id/leaderboard", h.Group.Leaderboard)
	pub.GET("/groups/:id/players/:memberId", h.Group.PlayerProfile)
}
