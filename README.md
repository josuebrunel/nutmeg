# Soccer Stats (Nutmeg)

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-336791?style=flat&logo=postgresql)](https://www.postgresql.org)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![Tests](https://github.com/josuebrunel/nutmeg/actions/workflows/build-and-push.yml/badge.svg)](https://github.com/josuebrunel/nutmeg/actions/workflows/build-and-push.yml)

A web application for tracking soccer matches, teams, players, and statistics within groups. Create groups, invite members, record match results, and visualise stats — all in a clean, responsive UI powered by HTMX and DaisyUI.

## Overview

**Soccer Stats** (codenamed Nutmeg) lets you organise informal soccer groups — such as weekly pick-up games or a local league — and track everything from match scores to individual player contributions. It provides:

- **Group-based organisation** — each group operates independently with its own set of teams, matches, and members.
- **Role-based access** — group admins can manage members, edit settings, and delete the group; regular members can view and participate.
- **Match tracking** — record home/away teams, scores, and individual events such as goals and assists.
- **Authentication** — built-in registration, login, and session management via Ezauth.

## Features

- User authentication (register, login, logout) with session management
- Group CRUD (create, read, update, delete)
- Member management (add/remove members, role assignment)
- Team and match management (scores, events)
- Responsive sidebar layout with contextual navigation
- Flash messages for success and error feedback
- HTMX-powered interactions for a SPA-like experience
- CDN-loaded Chart.js for future stats visualisation
- Docker Compose for local development with PostgreSQL
- Hot-reload development workflow with Air + Templ

## Tech Stack

| Layer              | Technology                                                                                                                                       |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Language**       | Go 1.25+                                                                                                                                         |
| **Web Framework**  | [Echo v5](https://github.com/labstack/echo/v5)                                                                                                   |
| **Templates**      | [Templ](https://github.com/a-h/templ) (type-safe HTML templates)                                                                                 |
| **Frontend**       | [HTMX 2.x](https://htmx.org), [DaisyUI 5](https://daisyui.com), [Tailwind CSS 4](https://tailwindcss.com), [Chart.js 4](https://www.chartjs.org) |
| **Database**       | PostgreSQL 17 (via [pgx](https://github.com/jackc/pgx/v5))                                                                                       |
| **Query Builder**  | [Bob](https://github.com/stephenafamo/bob) + [scan.StructMapper](https://github.com/stephenafamo/scan)                                           |
| **Migrations**     | [Goose v3](https://github.com/pressly/goose/v3) (embedded, no global registry)                                                                   |
| **Authentication** | [Ezauth](https://github.com/josuebrunel/ezauth)                                                                                                  |
| **Configuration**  | [Xenv](https://github.com/josuebrunel/gopkg/xenv)                                                                                                |
| **Hot Reload**     | [Air](https://github.com/air-verse/air)                                                                                                          |

## Architecture

The application follows a clean **layered architecture** within the `internal/` package:

```
┌─────────────────────────────────────────────────────┐
│                     Router                          │
│           (internal/router/router.go)                │
├─────────────────────────────────────────────────────┤
│                   Handler                           │
│  (internal/handler/) — HTTP handlers, parsing,      │
│   flash messages, form validation, layout wiring    │
├─────────────────────────────────────────────────────┤
│                   Service                           │
│  (internal/service/) — business logic, authorisation│
│   checks, orchestration of repository calls         │
├─────────────────────────────────────────────────────┤
│                  Repository                         │
│  (internal/repository/) — SQL queries via Bob ORM   │
│   (psql dialect), data access layer                 │
├─────────────────────────────────────────────────────┤
│                     Model                           │
│  (internal/model/) — domain structs with db tags    │
├─────────────────────────────────────────────────────┤
│                   Database                          │
│  (internal/database/) — pgx pool open, Goose        │
│   migrations                                        │
├─────────────────────────────────────────────────────┤
│                   Views (Templ)                     │
│  (views/) — type-safe HTML templates, layout,       │
│   page components, SVG icons                        │
└─────────────────────────────────────────────────────┘
```

### Entity flow (example: Group)

```
Model → Repository → Service → Handler → Templ View → Route
```

Every entity follows the same convention: one file each for model, repository operations, service logic, handler methods, and views (list, form, detail).

### Authentication

- **Ezauth** handles all auth concerns: registration, login, logout, session middleware, and login-required middleware.
- `SessionMiddleware` is applied globally to the Echo instance.
- `LoginRequiredMiddleware` is applied only to the authenticated app group (not the `/auth/*` routes).
- Users table is managed by Ezauth in the `auth` schema.

## Database Schema

The database contains five core tables:

| Table           | Description                                                                              |
| --------------- | ---------------------------------------------------------------------------------------- |
| `groups`        | Soccer groups; each group has a name, optional description, and creator                  |
| `group_players` | Many-to-many relationship between users and groups; includes role (`admin` or `member`)  |
| `teams`         | Teams within a group; each team has a name and optional colour                           |
| `matches`       | Matches between two teams; stores home/away scores, notes, and when the match was played |
| `match_events`  | Individual events within a match (goals, assists); links to the scoring team and players |

Indexes cover the foreign-key columns for efficient lookups.

## Getting Started

### Prerequisites

- Go 1.25+
- PostgreSQL 17 (or Docker)
- [Templ CLI](https://github.com/a-h/templ) — `go install github.com/a-h/templ/cmd/templ@latest`
- (Optional) [Air](https://github.com/air-verse/air) for hot reload — `go install github.com/air-verse/air@latest`

### Setup

1. **Clone the repository**

   ```bash
   git clone git@github.com:josuebrunel/nutmeg.git
   cd nutmeg
   ```

2. **Create environment file**

   ```bash
   cp .env.example .env
   ```

   Edit `.env` with your database credentials and secrets.

3. **Start the database**

   ```bash
   docker compose up -d db
   ```

4. **Run the application**

   ```bash
   make run
   ```

   This runs `templ generate`, builds the binary, and starts the server on `:8080`.

5. **Open the app**

   Visit [http://localhost:8080](http://localhost:8080) in your browser.

### Development with hot reload

```bash
make dev
```

This starts Templ's file watcher (for automatic `.templ` → `.go` generation) and Air (for automatic Go recompilation on file changes).

## Development Commands

| Command             | Description                                             |
| ------------------- | ------------------------------------------------------- |
| `make dev`          | Start hot-reload development server (Templ watch + Air) |
| `make build`        | Generate Templ code and build the binary                |
| `make run`          | Build and run the server                                |
| `make db`           | Start the PostgreSQL container only                     |
| `make docker-up`    | Build and start all Docker services                     |
| `make docker-down`  | Stop all Docker services                                |
| `make migrate`      | Run pending Goose migrations                            |
| `make migrate-down` | Roll back the last migration                            |
| `make templ-gen`    | Regenerate Templ template code                          |
| `make test`         | Run all tests (sequential, single-run)                  |
| `make clean`        | Remove build artifacts and generated files              |

## Project Structure

```
.
├── .air.toml                    # Air hot-reload configuration
├── .env.example                 # Environment variable template
├── .env                         # Environment variables (git-ignored)
├── Dockerfile                   # Multi-stage Docker build
├── docker-compose.yml           # PostgreSQL + app services
├── Makefile                     # Development commands
├── go.mod / go.sum              # Go module dependencies
├── cmd/server/main.go           # Application entry point
├── internal/
│   ├── assert/                  # Test assertion helpers
│   ├── config/                  # Environment-based configuration
│   ├── database/                # Database connection + migrations
│   ├── handler/                 # HTTP handlers (auth, group, home)
│   ├── middleware/              # Auth middleware (wrapper)
│   ├── model/                   # Domain structs with db tags
│   ├── render/                  # Templ rendering helpers
│   ├── repository/              # Data access layer (Bob psql queries)
│   ├── router/                  # Route registration
│   └── service/                 # Business logic layer
├── migrations/                  # SQL migration files (embedded)
├── static/css/                  # Static assets (CSS)
├── views/
│   ├── components/              # Reusable Templ components (icons)
│   ├── layout/                  # Base layout with sidebar
│   └── pages/                   # Page-specific templates
│       ├── auth/                # Login, Register
│       ├── groups/              # List, Form, Detail
│       ├── home/                # Dashboard
│       └── matches, players, stats, teams/  # Placeholder stubs
```

## API Routes

All authenticated routes are registered in `internal/router/router.go`:

| Method   | Path                       | Handler            | Description             |
| -------- | -------------------------- | ------------------ | ----------------------- |
| `GET`    | `/login`                   | Auth.Login         | Login page              |
| `GET`    | `/register`                | Auth.Register      | Registration page       |
| `GET`    | `/`                        | Home.Index         | Dashboard               |
| `GET`    | `/groups`                  | Group.Index        | List user's groups      |
| `GET`    | `/groups/new`              | Group.New          | New group form          |
| `POST`   | `/groups`                  | Group.Create       | Create a group          |
| `GET`    | `/groups/:id`              | Group.Detail       | Group details + members |
| `GET`    | `/groups/:id/edit`         | Group.Edit         | Edit group form         |
| `POST`   | `/groups/:id`              | Group.Update       | Update a group          |
| `DELETE` | `/groups/:id`              | Group.Delete       | Delete a group          |
| `POST`   | `/groups/:id/members`      | Group.AddMember    | Add member by email     |
| `DELETE` | `/groups/:id/members/:uid` | Group.RemoveMember | Remove a member         |

Auth routes (`/auth/*`) are handled by Ezauth automatically and include login, register, logout, and callback endpoints.

## Testing

Tests are written using the standard `testing` package with custom assertion helpers in `internal/assert/`.

```bash
# Run all tests
make test

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test -v ./internal/service/...
```

The test suite includes:
- **Service layer tests** (`internal/service/group_test.go`) — full coverage of group CRUD, member management, and authorisation logic using mock repositories.
- **Model tests** (`internal/model/errors_test.go`) — sentinel error comparisons.

## Docker

### Services

The `docker-compose.yml` defines three services:

1. **`dev`** — development container with hot reload, mounted source code
2. **`nutmeg`** — production container built from the multi-stage Dockerfile
3. **`db`** — PostgreSQL 17 Alpine

### Usage

```bash
# Development environment
docker compose up dev

# Production image
docker compose up nutmeg

# Start just the database (for local development)
docker compose up -d db
```

The application is exposed on port **8380** (mapped to container port 8080). The database is exposed on port **8381** (mapped to container port 5432).

## Configuration

All configuration is loaded from the environment (or `.env` file) using Xenv.

| Variable                       | Default                 | Description                     |
| ------------------------------ | ----------------------- | ------------------------------- |
| `ADDR`                         | `:8080`                 | Server listen address           |
| `BASE_URL`                     | `http://localhost:8080` | Base URL for redirects          |
| `DEBUG`                        | `false`                 | Enable debug mode               |
| `DB_DSN`                       | *(required)*            | PostgreSQL connection string    |
| `EZAUTH_JWT_SECRET`            | *(required)*            | JWT signing secret              |
| `EZAUTH_DB_DIALECT`            | `postgres`              | Auth database dialect           |
| `EZAUTH_DB_DSN`                | *(required)*            | Auth database connection string |
| `EZAUTH_DEBUG`                 | `true`                  | Auth debug mode                 |
| `EZAUTH_REDIRECT_AFTER_LOGIN`  | `/groups`               | Post-login redirect             |
| `EZAUTH_REDIRECT_AFTER_LOGOUT` | `/login`                | Post-logout redirect            |

## Non-Negotiable Rules

The project enforces several architectural rules (documented in `prompt.txt`):

1. **Echo v5** — handlers take `*echo.Context` (pointer); shutdown uses `StartConfig`; no `Shutdown()` method.
2. **Bob** — `bob.NewDB(*sql.DB)` returns a `bob.DB` value type, not a pointer.
3. **Goose** — use `goose.WithDisableGlobalRegistry(true)`; never use global registry functions.
4. **Ezauth** — `SessionMiddleware` on `e.Use` (global); `LoginRequiredMiddleware` only on the app group.
5. **Templ** — no `if` inside function call arguments; use `@{ }` code blocks instead.
6. **Models** — use `db` struct tags for `scan.StructMapper`.
7. **Migrations** — use `IF NOT EXISTS` / `IF EXISTS` for idempotency.
8. **Router** — wire all routes in a single `Register()` function called with an `echo.Group`.
9. **Handlers** — group into sub-handlers on a top-level `Handler` struct; use `page()` helper to wrap in layout.

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.