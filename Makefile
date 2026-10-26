.PHONY: help dev-up dev-down api worker test lint fmt

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

dev-up: ## Start local PostgreSQL via Docker Compose
	docker compose -f docker-compose.dev.yml up -d

dev-down: ## Stop local PostgreSQL
	docker compose -f docker-compose.dev.yml down

api: ## Run the API server locally
	go run ./cmd/api/...

worker: ## Run the sync worker locally
	go run ./cmd/worker/...

test: ## Run all Go tests
	go test ./...

lint: ## Run go vet
	go vet ./...

fmt: ## Format Go source files
	gofmt -w .
