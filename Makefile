.PHONY: help dev dev-api build-web build-api test-api docker-up docker-down

help:
	@echo "CBS Core Monorepo commands:"
	@echo "  make dev        - Run Next.js web frontend dev server"
	@echo "  make dev-api    - Run Go Core API backend server"
	@echo "  make build-web  - Build Next.js web frontend for production"
	@echo "  make build-api  - Build Go backend binary"
	@echo "  make test-api   - Run Go backend unit tests"
	@echo "  make docker-up  - Start local Postgres & Redis"

docker-up:
	docker compose up -d

docker-down:
	docker compose down

dev:
	cd apps/web && pnpm dev

dev-api:
	cd apps/api && go run cmd/server/main.go

build-web:
	cd apps/web && pnpm build

build-api:
	cd apps/api && go build -o bin/server ./cmd/server

test-api:
	cd apps/api && go test -v ./...

build: build-api build-web
