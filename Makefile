.PHONY: all dev-api dev-web test build-api docker-up docker-down

# Local services via Docker
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Run Go Core API Backend
dev-api:
	cd apps/core-api && go run cmd/server/main.go

# Run Next.js Backoffice Frontend
dev-web:
	pnpm --filter backoffice-web dev

# Run Unit Tests
test-api:
	cd apps/core-api && go test -v ./...

# Build all
build:
	cd apps/core-api && go build -v ./...
	pnpm build
