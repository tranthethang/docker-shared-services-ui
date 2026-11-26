.PHONY: build-web build dev format format-check lint test check hooks

BINARY := bin/dashboard

build-web:
	pnpm --dir web build

build: build-web
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/dashboard

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
