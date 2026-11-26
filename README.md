# Docker Shared Services UI

Use this local web dashboard to view and manage your `docker-shared-services` stack.

## What you can do

- Check service status, health checks, CPU, memory, and disk usage.
- Start, stop, restart, or remove services and watch live output.
- Apply built-in presets, or save, edit, and delete custom presets.
- Check shared networks, add missing `.env` keys, and validate Compose files.
- Export packages and clean up unused resources.
- Switch between **System**, **Light**, and **Dark** themes.
- Move around fast with keyboard shortcuts.

## Before you start

- Install Go 1.27 or newer.
- Install Node.js and `pnpm`.
- Start Docker with Docker Compose v2 on your machine.
- Use macOS (`launchctl` and `plutil`) if you want a background service.

## Run on your machine

1. Build the app:

```sh
make build
```

2. Start the dashboard with the path to your `docker-shared-services` folder:

```sh
./bin/dashboard --repo /path/to/docker-shared-services
```

3. Wait for your browser to open `http://127.0.0.1:8787`. Add `--no-open` if you do not want the browser to open.

## Run with Docker Compose

1. Copy the example settings file:

```sh
cp .env.example .env
```

2. Open `.env` and set `DSS_REPO_PATH` to your local `docker-shared-services` folder.
3. Make sure `docker-shared-services` (`traefik` on `traefik_proxy`) and `docker-shared-services-proxy` (`caddy`) are running.
4. Build and start the service:

```sh
make docker-up
```

5. Open `https://console.sandbox.localhost` in your browser. If you changed `DOMAIN` in `.env`, open `https://console.${DOMAIN}` instead.

### Optional `.env` settings

| Setting | Default | What it does |
| --- | --- | --- |
| `DOMAIN` | `sandbox.localhost` | Sets the host rule (`console.${DOMAIN}`). |
| `SANDBOX_NAME` | Empty | Adds a prefix to the service name and `traefik_proxy` network. |
| `AUTH_MIDDLEWARE` | Empty | Sets a Traefik auth rule such as `basic-auth@file` or `forward-auth@file`. |
| `RESTART_POLICY` | `unless-stopped` | Sets when the service restarts. |
| `CONSOLE_CPUS_LIMIT` | `0.50` | Limits CPU usage for the service. |
| `CONSOLE_MEMORY_LIMIT` | `128M` | Limits memory usage for the service. |

## Run in the background on macOS

1. Build and install the background service (`com.tranthethang.dss-ui`):

```sh
DSS_REPO_PATH=/path/to/docker-shared-services make install-service
```

You can also run the installer script directly after `make build`:

```sh
./scripts/launchd/install.sh --repo /path/to/docker-shared-services
```

Installed files:

- App: `~/.local/bin/dss-ui`
- Service file: `~/Library/LaunchAgents/com.tranthethang.dss-ui.plist`
- Output: `~/Library/Logs/dss-ui.log`

2. To stop and remove the background service, run:

```sh
make uninstall-service
```

Add `--remove-bin` to `./scripts/launchd/uninstall.sh` if you also want to delete `~/.local/bin/dss-ui`.

## Run in development mode

Start the Go app and the Vite dev app together:

```sh
DSS_REPO_PATH=/path/to/docker-shared-services make dev
```

## Startup flags

| Flag | Environment variable | Default | What it does |
| --- | --- | --- | --- |
| `--repo <path>` | `DSS_REPO_PATH` | Required | Sets the path to your `docker-shared-services` folder. |
| `--addr <host:port>` | None | `127.0.0.1:8787` | Sets the local web address. |
| `--no-open` | None | `false` | Stops the app from opening your browser at startup. |
| `--version` | None | `false` | Prints the version, git hash, and build date, then exits. |

## Keyboard shortcuts

Shortcuts do not run while you type inside an input box.

| Key | What it does |
| --- | --- |
| `/` | Focuses the service search box. |
| `r` | Refreshes services and stats. |
| `p` | Opens the presets window. |
| `?` | Shows or hides the keyboard shortcuts help. |
| `Esc` | Closes the open window or panel. |

## Common Make commands

| Command | What it does |
| --- | --- |
| `make build-web` | Builds the web files into `web/dist`. |
| `make build` | Builds the web files and compiles `./bin/dashboard`. |
| `make build-linux` | Builds the web files and compiles `./bin/dashboard-linux` for Linux. |
| `make docker-build` | Builds `./bin/dashboard-linux` and the Docker image. |
| `make docker-up` | Builds the image and starts the `console` service. |
| `make docker-down` | Stops and removes the `console` service. |
| `make dev` | Starts the Go app (`--no-open`) and Vite dev app. |
| `make format` | Formats Go files with `gofmt`. |
| `make format-check` | Checks Go formatting with `gofmt -l`. |
| `make lint` | Runs `golangci-lint run`. |
| `make test` | Runs `go test -race ./...`. |
| `make check` | Runs formatting, lint, tests, and web build checks. |
| `make install-service` | Installs the macOS background service. |
| `make uninstall-service` | Removes the macOS background service. |
| `make hooks` | Installs git hooks with `prek` or `pre-commit`. |

## If something goes wrong

| You see | What to do |
| --- | --- |
| `repository path is required` | Pass `--repo /path/to/docker-shared-services` or set `DSS_REPO_PATH`. |
| `Docker daemon is unreachable` | Start Docker Desktop or the Docker daemon, then try again. |
| Port `8787` is already in use | Pass another local port with `--addr 127.0.0.1:8788`. |
| Background service does not start on macOS | Check `~/Library/Logs/dss-ui.log` and run `plutil -lint ~/Library/LaunchAgents/com.tranthethang.dss-ui.plist`. |
