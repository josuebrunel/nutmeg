package router

import (
	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/handler"
	"nutmeg/internal/repository"
)

func Register(app *echo.Group, auth *ezauth.EzAuth, repo *repository.Repository) {
	h := handler.New(auth, repo)

	app.GET("/dashboard", h.Home.Dashboard)
	app.GET("/stats", h.Home.Stats)
	app.GET("/account", h.Account.Edit)
	app.POST("/account", h.Account.Update)
	app.POST("/account/password", h.Account.UpdatePassword)
	app.GET("/groups", h.Group.Index)
	app.GET("/groups/new", h.Group.New)
	app.POST("/groups", h.Group.Create)
	app.GET("/groups/:id", h.Group.Detail)
	app.GET("/groups/:id/edit", h.Group.Edit)
	app.POST("/groups/:id", h.Group.Update)
	app.DELETE("/groups/:id", h.Group.Delete)
	app.POST("/groups/:id/members", h.Group.AddMember)
	app.POST("/groups/:id/members/import", h.Group.ImportMembers)
	app.DELETE("/groups/:id/members/:memberId", h.Group.RemoveMember)
	app.GET("/groups/:id/members/:memberId/edit", h.Group.EditMemberForm)
	app.POST("/groups/:id/members/:memberId", h.Group.UpdateMember)
	app.POST("/groups/:id/members/:memberId/promote", h.Group.PromoteMember)
	app.POST("/groups/:id/members/:memberId/demote", h.Group.DemoteMember)
	app.POST("/groups/:id/join-requests", h.Group.RequestJoin)
	app.POST("/groups/:id/join-requests/:reqId/approve", h.Group.ApproveJoinRequest)
	app.POST("/groups/:id/join-requests/:reqId/reject", h.Group.RejectJoinRequest)
	app.GET("/groups/:id/detail-content", h.Group.DetailContent)
	app.GET("/groups/:id/leaderboard-full", h.Group.LeaderboardFull)
	app.GET("/groups/:id/roster-full", h.Group.RosterFull)
	app.GET("/groups/:id/matches-full", h.Group.MatchesFull)
	app.GET("/groups/:id/match-modal", h.Match.LogMatchModal)
	app.POST("/groups/:id/matches", h.Match.Create)
	app.GET("/groups/:id/matches/:mid/edit", h.Match.EditModal)
	app.POST("/groups/:id/matches/:mid/update", h.Match.Update)
	app.DELETE("/groups/:id/matches/:mid", h.Match.Delete)
}
