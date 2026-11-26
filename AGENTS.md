# AGENTS.md

Local web dashboard for `docker-shared-services` built with a Go HTTP server (`127.0.0.1:8787`) and an embedded React + TypeScript + Tailwind CSS frontend (`go:embed`).

## Architecture

- `cmd/dashboard/main.go`: CLI entrypoint (`--addr`, `--repo`, `--no-open`) with signal-based graceful shutdown.
- `internal/server/`: `net/http` `ServeMux` serving `/api/health` and SPA fallback over `fs.FS`.
- `internal/rules/`: Repository-level automated structural tests (300-line Go file limit).
- `web/`: React 19 + Vite + TypeScript (strict) + Tailwind CSS v4 application, embedded via `web/embed.go` (`//go:embed all:dist`).
- `docs/dev/`: Engineering standards and quality gate documentation.

## Commands

All workflows run through `Makefile`:

- `make dev`: Run the Go server (`--no-open`) and Vite dev server concurrently.
- `make build-web`: Build frontend assets into `web/dist`.
- `make build`: Build frontend assets and compile `./bin/dashboard`.
- `make format`: Format Go files with `gofmt`.
- `make format-check`: Verify Go formatting with `gofmt -l`.
- `make lint`: Run `golangci-lint run`.
- `make test`: Run `go test -race ./...`.
- `make check`: Run `format-check`, `lint`, `test`, plus `pnpm --dir web lint`, `typecheck`, and `build`.
- `make hooks`: Install git hooks via `prek` or `pre-commit`.

## Limits and Gotchas

- **300-line Go file limit**: Every `.go` file must stay at 300 lines or fewer, enforced by `internal/rules/size_rules_test.go`.
- **Localhost binding**: Default listen address is `127.0.0.1:8787` with no authentication; never bind `0.0.0.0` by default.
- **Secret masking**: Never log, return in API responses, or commit `.env` values, tokens, or credentials.
- **Single-flight operations**: Serialize mutating Docker or Compose actions per service/stack to avoid concurrent state corruption.
- **Context cancellation**: Always propagate `context.Context` from HTTP requests and OS signals into subprocesses, Docker SDK calls, and shutdown paths.
- **Build artifacts**: Never commit `.env`, secrets, `bin/`, `web/node_modules/`, or `web/dist/`.

## Detailed Standards

- [Go Standards](docs/dev/go-standards.md)
- [Frontend Standards](docs/dev/frontend-standards.md)
- [Quality Gate](docs/dev/quality-gate.md)
