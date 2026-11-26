# Go Engineering Standards

Every rule below states whether it is enforced by an automated tool or by code review.

## Layout and Size

- **300-line file limit**: Every `.go` source file must be 300 lines or fewer. Split large packages into cohesive files.
  - *Enforced by*: `go test -race ./internal/rules/...` (`TestGoFileLineLimits`).
- **Standard layout**: Keep entrypoints in `cmd/`, application packages in `internal/`, and embedded frontend bridge in `web/`.
  - *Enforced by*: Review.

## Formatting and Linting

- **gofmt formatting**: All `.go` files must pass `gofmt -l`.
  - *Enforced by*: `make format-check`.
- **golangci-lint v2**: Code must pass `.golangci.yml` (`govet`, `errcheck`, `staticcheck`, `revive`, `gosec`, `bodyclose`, `contextcheck`, `exhaustive`, `gocritic`, `unused`).
  - *Enforced by*: `make lint` (`golangci-lint run`).
- **Package comments**: Every Go package must have a top-level package comment.
  - *Enforced by*: `golangci-lint` (`revive` `package-comments`).

## HTTP and Concurrency

- **Server timeouts**: `http.Server` must set `ReadHeaderTimeout` (5s default) to mitigate Slowloris attacks.
  - *Enforced by*: `golangci-lint` (`gosec`) and Review.
- **Context propagation**: Pass `r.Context()` or signal `ctx` into all I/O, Docker SDK, and subprocess calls.
  - *Enforced by*: `golangci-lint` (`contextcheck`) and Review.
- **Race-free concurrency**: All tests must pass under the Go race detector.
  - *Enforced by*: `make test` (`go test -race ./...`).
- **Single-flight mutations**: Serialize mutating operations per target service or stack.
  - *Enforced by*: Review.
- **Secret masking**: Redact environment secrets before logging or returning JSON payloads.
  - *Enforced by*: Review.
