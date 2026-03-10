# Copilot Coding Agent Instructions

## Project Summary

Huntarr2 is a Go web service that automates quality upgrade searches across *arr applications (Sonarr, Radarr, Lidarr, Whisparr). It runs as a single Docker container with a SQLite database, serving a web UI on port 9706. The codebase is roughly 75 Go source files plus templ templates, JS, and SQL migrations.

**Stack:** Go 1.26 (stdlib `net/http`), zerolog, SQLite via `modernc.org/sqlite` (pure Go, no CGO), templ templates, Alpine.js, Tailwind CSS.

## Build, Test, and Validate

Always run commands from the repository root. The `Makefile` has all targets.

### Generate templ code (required before build)

```
go tool templ generate
```

This compiles `.templ` files into `_templ.go` files. The `make build` target runs this automatically. If you modify any `.templ` file, always regenerate before building or testing.

### Build

```
make build
```

Produces `bin/huntarr2`. Uses `CGO_ENABLED=0`. Build takes ~3 seconds.

### Test

```
make test
```

Runs `go test -race ./...`. Tests take ~10 seconds total. Tests use in-memory SQLite databases via `internal/database/testdb/testdb.go`. No external services or Docker required.

### Lint

```
make lint
```

Runs `golangci-lint run ./...` (v2, config in `.golangci.yml`). Zero issues is required. Key linter rules that commonly cause failures:

- **misspell with locale UK**: all identifiers, comments, error messages, and string literals must use British English spelling (e.g. "colour", "initialise", "behaviour", "organisation").
- **depguard**: the standard `log` package is forbidden; use `github.com/rs/zerolog` instead.
- **forbidigo**: `fmt.Print*` is forbidden; use zerolog for output.
- **gosec**: security checks are enabled (some rules excluded in tests).
- **errorlint**: use `%w` for error wrapping, use `errors.Is`/`errors.As` for comparisons.

### Format check

```
make fmt-check
```

Verifies `gofmt` formatting. Run `make fmt` to auto-fix.

### Prettier (JS, JSON, YAML)

```
make prettier-check
```

Requires Node.js. Checks formatting of `web/static/js/**/*.js`, `**/*.json`, `**/*.yml`. Run `make prettier` to auto-fix.

### Full validation sequence (matches CI)

Always run these in order before considering a change complete:

```
go tool templ generate
make fmt-check
make test
make lint
```

### Go mod tidy

After adding or removing imports, always run:

```
go mod tidy
```

CI verifies `go.mod` and `go.sum` are tidy and contain no `replace` directives.

## CI Workflows (`.github/workflows/`)

Four workflows run on every PR to `main`:

| Workflow | File | What it checks |
|---|---|---|
| **Quality** | `quality.yml` | `go mod tidy` is clean, no `replace` directives, generated templ files up to date, golangci-lint passes |
| **Tests** | `tests.yml` | `make test`, coverage upload |
| **Format** | `format.yml` | `make fmt-check`, `make prettier-check` (Node 22) |
| **Vulnerability** | `vulnerability.yml` | `govulncheck ./...` |

All four must pass for a PR to merge.

## Project Layout

```
cmd/huntarr2/main.go           Entry point (zerolog setup, config, DB, server)
internal/
  activity/                     Activity log (domain, repository, service, SQLite impl)
  api/                          REST API handlers (one file per resource)
  arr/                          *arr client (HTTP client, adapters, history, library, quality)
  config/                       Configuration loading from environment variables
  cooldown/                     Per-item search cooldown tracking
  database/                     SQLite connection, migrations, error mapping, tx helper
    migrations/                 Goose SQL migrations (embedded via embed.go)
    testdb/                     Test helper: in-memory SQLite with migrations applied
  encrypt/                      AES-256-GCM encryption for API keys at rest
  instance/                     Instance CRUD (domain types, repository, service, SQLite)
  scheduler/                    Adaptive scheduling engine
  server/                       HTTP server, page handlers, middleware
  settings/                     Settings system (global + per-instance overrides)
web/
  templates/layouts/            Templ layout templates (base, sidebar)
  templates/pages/              Templ page templates (home, connections, settings, logs)
  static/                       Embedded static assets (JS, images)
    embed.go                    go:embed directive for static files
```

### Key configuration files

- `.golangci.yml` — linter config (UK locale, depguard, forbidigo rules)
- `.prettierrc.json` — Prettier config for JS/JSON/YAML
- `Makefile` — all build/test/lint/docker targets
- `Dockerfile` — multi-stage build (golang:1.26-alpine → alpine:3.23)
- `docker-compose.yml` — local development stack

## Architecture and Conventions

- **Layered architecture**: HTTP handlers (`internal/api/`) → service layer → repository interface → SQLite implementation. No SQL in handlers.
- **Dependency injection**: services receive `*sql.DB`, config, and other dependencies as struct fields. No globals except the zerolog logger.
- **Error wrapping**: always use `fmt.Errorf("context: %w", err)`.
- **Tests**: standard library `testing` only. Tests live in the same package (not `_test` packages). Table-driven tests for validation. Interfaces faked with local structs. Use `t.Helper()` and `t.Setenv()`.
- **Database**: single migration file `001_initial_schema.sql` (pre-launch policy). UUID primary keys. All schema changes go in this file, not new migration files.
- **UK English everywhere**: identifiers, comments, error messages, JSON fields, database columns. The misspell linter enforces this.

## Trust These Instructions

These instructions have been validated against the current codebase. Trust them and only search the codebase if the information here is incomplete or produces an error.
