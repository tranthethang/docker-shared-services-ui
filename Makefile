.PHONY: build-web build build-linux dev format format-check lint test check hooks install-service uninstall-service docker-build docker-up docker-down

BINARY := bin/dashboard
LINUX_BINARY := bin/dashboard-linux
HOST_ARCH := $(shell uname -m)
TARGETARCH ?= $(if $(GOARCH),$(GOARCH),$(if $(filter arm64 aarch64,$(HOST_ARCH)),arm64,amd64))
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

build-web:
	pnpm --dir web build

build: build-web
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/dashboard

build-linux: build-web
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(TARGETARCH) go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(LINUX_BINARY) ./cmd/dashboard

dev:
	@echo "Starting Go API server and Vite dev server..."
	@trap 'kill 0' EXIT; \
	go run ./cmd/dashboard --no-open & \
	pnpm --dir web dev & \
	wait

format:
	gofmt -w cmd internal web

format-check:
	@unformatted="$$(gofmt -l cmd internal web)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted Go files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

lint:
	golangci-lint run

test:
	go test -race ./...

check: format-check lint test
	pnpm --dir web lint
	pnpm --dir web typecheck
	pnpm --dir web build

hooks:
	@if command -v prek >/dev/null 2>&1; then \
		prek install --hook-type pre-commit --hook-type pre-push; \
	elif command -v pre-commit >/dev/null 2>&1; then \
		pre-commit install --hook-type pre-commit --hook-type pre-push; \
	else \
		echo "Neither prek nor pre-commit is installed."; \
		exit 1; \
	fi

install-service: build
	./scripts/launchd/install.sh

uninstall-service:
	./scripts/launchd/uninstall.sh

docker-build: build-linux
	docker compose build

docker-up: docker-build
	docker compose up -d

docker-down:
	docker compose down
