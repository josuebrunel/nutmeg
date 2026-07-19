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
	app.GET("/groups", h.Group.Index)
	app.GET("/groups/new", h.Group.New)
	app.POST("/groups", h.Group.Create)
	app.GET("/groups/:id", h.Group.Detail)
	app.GET("/groups/:id/edit", h.Group.Edit)
	app.POST("/groups/:id", h.Group.Update)
	app.DELETE("/groups/:id", h.Group.Delete)
	app.POST("/groups/:id/members", h.Group.AddMember)
	app.DELETE("/groups/:id/members/:memberId", h.Group.RemoveMember)
	app.GET("/groups/:id/detail-content", h.Group.DetailContent)
	app.GET("/groups/:id/match-modal", h.Match.LogMatchModal)
	app.POST("/groups/:id/matches", h.Match.Create)
	app.GET("/groups/:id/matches/:mid/edit", h.Match.EditModal)
	app.POST("/groups/:id/matches/:mid/update", h.Match.Update)
	app.DELETE("/groups/:id/matches/:mid", h.Match.Delete)
}
