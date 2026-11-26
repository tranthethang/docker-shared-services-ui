# Docker Shared Services UI

Local web dashboard for managing a `docker-shared-services` stack. Built as a single Go binary with an embedded React, TypeScript, and Tailwind CSS frontend (`go:embed`), bound to `127.0.0.1:8787` by default.

## Features

- **Service catalog & live telemetry**: Inspect service state, health checks, containers, CPU/memory usage, and Docker disk breakdown.
- **Lifecycle & batch operations**: Start, stop, restart, or tear down individual services or apply batch selections with live streaming console output.
- **Built-in & custom presets**: Apply built-in stack presets or save, edit, and delete custom presets in `localStorage`.
- **Environment & maintenance tools**: Check shared Docker networks, merge missing `.env` keys from `.env.example`, validate Compose configs, export standalone packages, and prune unused resources.
- **Theme & keyboard shortcuts**: Switch between `System`, `Light`, and `Dark` themes, and navigate with `/`, `r`, `p`, `?`, and `Esc`.

## Prerequisites

- **Go** 1.27+
- **pnpm** (with Node.js)
- **Docker** daemon running locally (with Docker Compose v2)
- **macOS** (`launchctl`, `plutil`) if installing the background `launchd` service

## Quick Start

Build the single self-contained binary (`./bin/dashboard`):

```sh
make build
```

Run the dashboard against your `docker-shared-services` repository:

```sh
./bin/dashboard --repo /path/to/docker-shared-services
```

On startup, the dashboard automatically opens `http://127.0.0.1:8787` in your default browser unless `--no-open` is passed.

## Development Mode

Run the Go API server (`--no-open`) and the Vite dev server concurrently:

```sh
DSS_REPO_PATH=/path/to/docker-shared-services make dev
```

## CLI Flags & Environment Variables

| Flag | Environment Variable | Default | Description |
| --- | --- | --- | --- |
| `--repo <path>` | `DSS_REPO_PATH` | *(required)* | Path to the `docker-shared-services` repository. |
| `--addr <host:port>` | — | `127.0.0.1:8787` | HTTP listen address. |
| `--no-open` | — | `false` | Do not open the browser automatically on startup. |
| `--version` | — | `false` | Print build version, commit SHA, and build date, then exit. |

## macOS `launchd` Background Service

Install the dashboard as a macOS LaunchAgent (`com.tranthethang.dss-ui`) that starts at login (`RunAtLoad`) and stays running (`KeepAlive`) with `--no-open`:

```sh
DSS_REPO_PATH=/path/to/docker-shared-services make install-service
```

Or invoke the installer script directly:

```sh
make build
./scripts/launchd/install.sh --repo /path/to/docker-shared-services [--addr 127.0.0.1:8787]
```

This installs:
- Binary: `~/.local/bin/dss-ui`
- Plist: `~/Library/LaunchAgents/com.tranthethang.dss-ui.plist`
- Logs: `~/Library/Logs/dss-ui.log`

To stop and remove the LaunchAgent:

```sh
make uninstall-service
# Or also remove ~/.local/bin/dss-ui:
./scripts/launchd/uninstall.sh --remove-bin
```

## Keyboard Shortcuts

| Key | Action |
| --- | --- |
| `/` | Focus service search |
| `r` | Refresh services and telemetry |
| `p` | Open service presets modal |
| `?` | Show or hide keyboard shortcuts help |
| `Esc` | Close the active modal or drawer |

Shortcuts are ignored while typing inside `<input>`, `<textarea>`, `<select>`, or `contenteditable` elements.

## Make Targets

- `make build-web`: Build frontend assets into `web/dist`.
- `make build`: Build frontend assets and compile `./bin/dashboard` with `-ldflags` version metadata.
- `make dev`: Run Go server (`--no-open`) and Vite dev server concurrently.
- `make format`: Format Go files with `gofmt`.
- `make format-check`: Verify Go formatting with `gofmt -l`.
- `make lint`: Run `golangci-lint run`.
- `make test`: Run `go test -race ./...`.
- `make check`: Run `format-check`, `lint`, `test`, plus `pnpm --dir web lint`, `typecheck`, and `build`.
- `make install-service`: Build and install the macOS `launchd` user agent.
- `make uninstall-service`: Unload and remove the macOS `launchd` user agent.
- `make hooks`: Install git hooks via `prek` or `pre-commit`.

## Troubleshooting

- **`repository path is required`**: Pass `--repo /path/to/docker-shared-services` or export `DSS_REPO_PATH`.
- **`Docker daemon is unreachable`**: Ensure Docker Desktop (or the Docker daemon) is running and accessible to your user.
- **Port `8787` already in use**: Pass a different localhost port via `--addr 127.0.0.1:8788`.
- **`launchd` service not starting**: Inspect `~/Library/Logs/dss-ui.log` and verify `plutil -lint ~/Library/LaunchAgents/com.tranthethang.dss-ui.plist`.
