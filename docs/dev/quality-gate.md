# Quality Gate

Every rule below states whether it is enforced by an automated tool or by code review.

## Local Gate (`make check`)

Before committing or opening a pull request, run `make check`. It executes in order:

1. `make format-check`: Verifies `gofmt -l cmd internal web` produces no output.
   - *Enforced by*: `make check` and pre-commit hook.
2. `make lint`: Runs `golangci-lint run` with `.golangci.yml`.
   - *Enforced by*: `make check` and pre-commit hook.
3. `make test`: Runs `go test -race ./...` (includes `internal/rules/size_rules_test.go`).
   - *Enforced by*: `make check` and pre-push hook.
4. `pnpm --dir web lint`: Runs `oxlint src`.
   - *Enforced by*: `make check`.
5. `pnpm --dir web typecheck`: Runs `tsc --noEmit`.
   - *Enforced by*: `make check`.
6. `pnpm --dir web build`: Runs `tsc -b && vite build`.
   - *Enforced by*: `make check`.

## CI and Git Hooks

- **GitHub Actions CI**: `.github/workflows/ci.yml` runs `make check` on every push and pull request.
  - *Enforced by*: GitHub Actions CI.
- **Git hooks**: `.pre-commit-config.yaml` runs `make format-check` and `make lint` on `pre-commit`, and `make test` on `pre-push`.
  - *Enforced by*: `prek` / `pre-commit` (installed via `make hooks`).
- **Clean repository**: Never commit `.env`, secrets, `bin/`, `web/node_modules/`, or `web/dist/`.
  - *Enforced by*: `.gitignore` and Review.
