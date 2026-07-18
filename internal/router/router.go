package router

import (
	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/handler"
	"nutmeg/internal/repository"
)

func Register(e *echo.Group, auth *ezauth.EzAuth, repo *repository.Repository) {
	h := handler.New(auth, repo)

	e.GET("/", h.Home.Index)
	e.GET("/groups", h.Group.Index)
	e.GET("/groups/new", h.Group.New)
	e.POST("/groups", h.Group.Create)
	e.GET("/groups/:id", h.Group.Detail)
	e.GET("/groups/:id/edit", h.Group.Edit)
	e.POST("/groups/:id", h.Group.Update)
	e.DELETE("/groups/:id", h.Group.Delete)
	e.POST("/groups/:id/members", h.Group.AddMember)
	e.DELETE("/groups/:id/members/:uid", h.Group.RemoveMember)
}
