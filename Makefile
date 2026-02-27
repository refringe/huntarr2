.PHONY: build run generate test test-cover lint vet fmt fmt-check prettier prettier-check tidy docker-build docker-up docker-down docker-logs migrate-up migrate-down migrate-status clean hooks

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Build

generate:
	go tool templ generate

build: generate
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/huntarr2 ./cmd/huntarr2

run: build
	./bin/huntarr2

# Quality

test:
	go test -race ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	test -z "$$(gofmt -l .)"

prettier:
	npx --yes prettier@3 --write 'web/static/js/**/*.js' '**/*.json' '**/*.yml'

prettier-check:
	npx --yes prettier@3 --check 'web/static/js/**/*.js' '**/*.json' '**/*.yml'

# Dependencies

tidy:
	go mod tidy

# Docker

docker-build:
	docker compose build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(DATE)

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f huntarr2

# Database

migrate-up:
	goose -dir internal/database/migrations sqlite3 "$(DATABASE_PATH)" up

migrate-down:
	goose -dir internal/database/migrations sqlite3 "$(DATABASE_PATH)" down

migrate-status:
	goose -dir internal/database/migrations sqlite3 "$(DATABASE_PATH)" status

# Setup

hooks:
	@ln -sf ../../.githooks/pre-commit .git/hooks/pre-commit
	@echo "pre-commit hook installed"

# Clean

clean:
	rm -rf bin/ coverage.out
