package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"filippo.io/csrf"
	"github.com/josuebrunel/ezauth"
	ezcfg "github.com/josuebrunel/ezauth/pkg/config"
	"github.com/josuebrunel/gopkg/xenv"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/stephenafamo/bob"
	httpSwagger "github.com/swaggo/http-swagger"

	"nutmeg/internal/config"
	"nutmeg/internal/database"
	"nutmeg/internal/email"
	"nutmeg/internal/handler"
	"nutmeg/internal/llm"
	appmw "nutmeg/internal/middleware"
	"nutmeg/internal/repository"
	"nutmeg/internal/router"
	"nutmeg/internal/service"
	"nutmeg/internal/worker"
	_ "nutmeg/docs"
	"nutmeg/migrations"
)

// buildLLMClient picks the LLMGenerator implementation for provider — the
// one place that knows how to construct either client, shared by the real
// server startup and the -llmtest smoke-test path below.
func buildLLMClient(provider, apiKey, model, baseURL string) service.LLMGenerator {
	switch provider {
	case "google":
		return llm.NewGoogleClient(apiKey, model, 2*time.Minute)
	default:
		return llm.NewClient(baseURL, model, 2*time.Minute)
	}
}

// runLLMTest sends a sample roast prompt to the configured LLM provider and
// prints the response — a quick way to try out a model or confirm a
// provider is reachable before wiring it up for real, without needing a
// database or the rest of the server. Mirrors the standalone ollama-test
// command this replaced, but goes through the same provider selection as
// the real app so it works for Ollama or Google alike.
func runLLMTest(provider, apiKey, model, baseURL string) {
	stats := repository.PlayerStats{MatchesPlayed: 5, Wins: 2, Draws: 1, Losses: 2, Goals: 3, Assists: 1}
	history := []repository.PlayerMatchResult{
		{GoalsScored: 0},
		{GoalsScored: 0},
		{GoalsScored: 0},
	}
	prompt := (&service.CommentaryService{}).BuildPrompt("Chris", stats, history, true, false)

	fmt.Println("--- prompt ---")
	fmt.Println(prompt)

	client := buildLLMClient(provider, apiKey, model, baseURL)
	fmt.Printf("\n--- response (provider=%s, model=%s) ---\n", provider, client.Model())

	response, err := client.Generate(context.Background(), prompt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(response)
}

// @title Nutmeg API
// @version 1.0
// @description JSON API for Nutmeg, a self-hosted pickup-soccer stats tracker.
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer token obtained from POST /auth/api/login (see ezauth's JSON auth endpoints).
func main() {
	migrateFlag := flag.String("migrate", "", "run migrations (up or down) then exit, without starting the server")
	llmtestFlag := flag.Bool("llmtest", false, "send a sample roast prompt to the configured LLM provider, then exit")
	providerFlag := flag.String("provider", "", "override LLM_PROVIDER for -llmtest")
	modelFlag := flag.String("model", "", "override LLM_MODEL for -llmtest")
	urlFlag := flag.String("url", "", "override LLM_BASE_URL for -llmtest (ollama only)")
	flag.Parse()

	var cfg config.Config
	if err := xenv.Load(&cfg); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if *llmtestFlag {
		provider := cfg.LLM.Provider
		if *providerFlag != "" {
			provider = *providerFlag
		}
		model := cfg.LLM.Model
		if *modelFlag != "" {
			model = *modelFlag
		}
		baseURL := cfg.LLM.BaseURL
		if *urlFlag != "" {
			baseURL = *urlFlag
		}
		runLLMTest(provider, cfg.LLM.APIKey, model, baseURL)
		return
	}

	db, err := database.Open(cfg.Database.DSN, database.PoolConfig{
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
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

	// River is Nutmeg's first background-job dependency, introduced for
	// async LLM commentary generation — riverdatabasesql (not riverpgxv5)
	// because the app opens its DB as a *sql.DB, not a pgx pool. River
	// manages its own job-queue schema via rivermigrate, separate from
	// and in addition to the app's own goose-based migrations above.
	riverDriver := riverdatabasesql.New(db)
	migrator, err := rivermigrate.New(riverDriver, nil)
	if err != nil {
		slog.Error("failed to create river migrator", "error", err)
		os.Exit(1)
	}
	if _, err := migrator.Migrate(context.Background(), rivermigrate.DirectionUp, nil); err != nil {
		slog.Error("failed to run river migrations", "error", err)
		os.Exit(1)
	}

	llmClient := buildLLMClient(cfg.LLM.Provider, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.BaseURL)
	commentarySvc := service.NewCommentaryService(repo, llmClient)
	activitySvc := service.NewActivityService(repo, llmClient)
	articleSvc := service.NewMatchArticleService(repo, llmClient)
	emailClient := email.NewClient(cfg.Email.SMTPHost, cfg.Email.SMTPPort, cfg.Email.Username, cfg.Email.Password, cfg.Email.From)

	workers := river.NewWorkers()
	river.AddWorker(workers, &worker.GenerateCommentaryWorker{Service: commentarySvc})
	river.AddWorker(workers, &worker.GenerateGroupNewsWorker{Service: activitySvc})
	river.AddWorker(workers, &worker.GenerateMatchArticleWorker{Service: articleSvc})
	river.AddWorker(workers, &worker.SendEmailWorker{Client: emailClient})

	riverClient, err := river.NewClient(riverDriver, &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 2},
		},
		Workers: workers,
	})
	if err != nil {
		slog.Error("failed to create river client", "error", err)
		os.Exit(1)
	}

	os.Setenv("EZAUTH_API_KEY", "no-need")
	authCfg, err := ezcfg.LoadConfig()
	if err != nil {
		slog.Error("failed to load ezauth config", "error", err)
		os.Exit(1)
	}
	authCfg.Redirects.AfterLogin = "/dashboard"
	authCfg.Pages.Login = "/login"
	authCfg.Pages.Register = "/register"
	authCfg.Debug = cfg.Debug
	if os.Getenv("EZAUTH_RATE_LIMIT_ENABLED") == "" {
		// ezauth defaults rate limiting off; without it, login/register/
		// password-reset have no brute-force protection. Still overridable
		// via EZAUTH_RATE_LIMIT_ENABLED=false if an operator needs it off.
		authCfg.RateLimit.Enabled = true
	}

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
	e.Use(appmw.Location)

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

	handler.SetBaseURL(cfg.BaseURL)
	h := handler.New(auth, repo, commentarySvc, articleSvc, riverClient)

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
	// Deliberately no "/article" suffix, so this doubles as the clean,
	// canonical, shareable URL for a match (used both as the HTMX overlay
	// target and as the social-preview-able direct link — see
	// GroupHandler.PublicMatchArticle).
	e.GET("/groups/:id/matches/:mid", h.Group.PublicMatchArticle)
	// Detail does its own auth check (redirecting to the public leaderboard
	// above when unauthenticated) rather than living behind the blanket
	// LoginRequiredMiddleware, since that middleware can only redirect to a
	// fixed static login path, not a dynamic per-group URL.
	e.GET("/groups/:id", h.Group.Detail)

	// Authenticated routes
	app := e.Group("")
	app.Use(echo.WrapMiddleware(auth.LoginRequiredMiddleware))
	// Rejects state-changing (non-GET/HEAD/OPTIONS) cross-origin requests via
	// Sec-Fetch-Site/Origin checks — no token needed in forms/HTMX requests.
	// Not applied to apiGroup below, which is Bearer-JWT authenticated (no
	// ambient browser credentials, so not CSRF-vulnerable).
	app.Use(echo.WrapMiddleware(csrf.New().Handler))
	router.Register(app, auth, repo, commentarySvc, articleSvc, riverClient)

	// JSON API (/api/v1), authenticated via ezauth's JWT Bearer middleware
	// rather than the cookie session used by the routes above.
	apiGroup := e.Group("/api/v1")
	apiGroup.Use(echo.WrapMiddleware(auth.AuthMiddleware))
	router.RegisterAPI(apiGroup, auth, repo, commentarySvc, riverClient)

	publicAPI := e.Group("/api/v1/public")
	router.RegisterPublicAPI(publicAPI, auth, repo, commentarySvc, riverClient)

	// InstanceName must be "nutmeg" (matching -instanceName on `make swag`
	// and docs.SwaggerInfonutmeg's registration) — the default instance
	// name ("swagger") collides with ezauth's own embedded Swagger docs,
	// which registers under that name in the same process-wide swag
	// registry as soon as ezauth.NewWithDB is called above.
	e.GET("/swagger/*", echo.WrapHandler(httpSwagger.Handler(httpSwagger.InstanceName("nutmeg"))))

	sc := echo.StartConfig{Address: cfg.Addr}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := riverClient.Start(ctx); err != nil {
		slog.Error("failed to start river client", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := riverClient.Stop(context.Background()); err != nil {
			slog.Error("failed to stop river client", "error", err)
		}
	}()

	slog.Info("starting server", "addr", cfg.Addr)
	if err := sc.Start(ctx, e); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
