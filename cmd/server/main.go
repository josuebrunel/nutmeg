package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/josuebrunel/ezauth"
	ezcfg "github.com/josuebrunel/ezauth/pkg/config"
	"github.com/josuebrunel/gopkg/xenv"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/stephenafamo/bob"

	"nutmeg/internal/config"
	"nutmeg/internal/database"
	"nutmeg/internal/handler"
	"nutmeg/internal/repository"
	"nutmeg/internal/router"
	"nutmeg/migrations"
)

func main() {
	migrateFlag := flag.String("migrate", "", "run migrations (up or down) then exit, without starting the server")
	flag.Parse()

	var cfg config.Config
	if err := xenv.Load(&cfg); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := database.Open(cfg.Database.DSN)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if *migrateFlag != "" {
		switch *migrateFlag {
		case "up":
			if err := database.Migrate(db, migrations.FS); err != nil {
				slog.Error("migration up failed", "error", err)
				os.Exit(1)
			}
			slog.Info("migration up complete")
		case "down":
			if err := database.MigrateDown(db, migrations.FS); err != nil {
				slog.Error("migration down failed", "error", err)
				os.Exit(1)
			}
			slog.Info("migration down complete")
		default:
			slog.Error(`invalid -migrate value, must be "up" or "down"`, "value", *migrateFlag)
			os.Exit(1)
		}
		return
	}

	if err := database.Migrate(db, migrations.FS); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	bdb := bob.NewDB(db)
	repo := repository.New(bdb)

	os.Setenv("EZAUTH_API_KEY", "no-need")
	authCfg, err := ezcfg.LoadConfig()
	if err != nil {
		slog.Error("failed to load ezauth config", "error", err)
		os.Exit(1)
	}
	authCfg.Redirects.AfterLogin = "/dashboard"
	authCfg.Pages.Login = "/login"
	authCfg.Pages.Register = "/register"
	authCfg.Debug = true

	auth, err := ezauth.NewWithDB(&authCfg, db, "auth")
	if err != nil {
		slog.Error("failed to initialize auth", "error", err)
		os.Exit(1)
	}
	if err := auth.Migrate(); err != nil {
		slog.Error("failed to run auth migrations", "error", err)
		os.Exit(1)
	}

	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(echo.WrapMiddleware(auth.SessionMiddleware))

	// no-cache (not "no store") so browsers always revalidate with the server
	// before reusing a cached CSS/JS file — otherwise a deploy that changes
	// static/css/input.css or static/js/*.js can go unnoticed by a returning
	// visitor's browser for as long as its cache stays fresh.
	e.Static("/static", "static", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
			return next(c)
		}
	})

	e.Any("/auth/*", echo.WrapHandler(auth.Handler))

	h := handler.New(auth, repo)

	// Public routes
	e.GET("/", h.Home.Landing)
	e.GET("/login", h.Auth.Login)
	e.GET("/register", h.Auth.Register)
	e.GET("/health", func(c *echo.Context) error {
		if err := db.PingContext(c.Request().Context()); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.GET("/groups/:id/leaderboard", h.Group.PublicLeaderboard)
	e.GET("/groups/:id/players/:memberId", h.Group.PlayerProfile)

	// Authenticated routes
	app := e.Group("")
	app.Use(echo.WrapMiddleware(auth.LoginRequiredMiddleware))
	router.Register(app, auth, repo)

	sc := echo.StartConfig{Address: cfg.Addr}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("starting server", "addr", cfg.Addr)
	if err := sc.Start(ctx, e); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
