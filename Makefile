.PHONY: run migrate-up migrate-down create-admin test lint

MIGRATE ?= migrate
DB_URL ?= $(shell grep DATABASE_URL .env 2>/dev/null | cut -d= -f2-)
MIGRATIONS_PATH ?= internal/db/migrations

# Linux/macOS — no Windows use: go run ./cmd/migrate up | go run ./cmd/api
# Ou: .\dev.ps1

run:
	go run ./cmd/api

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

create-admin:
	go run ./cmd/create-admin -email $(EMAIL) -password $(PASSWORD) -name "$(NAME)"

test:
	go test ./... -count=1

lint:
	@which golangci-lint > /dev/null && golangci-lint run ./... || echo "golangci-lint não instalado, pulando"
