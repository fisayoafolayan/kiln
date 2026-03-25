.PHONY: up down setup generate run seed reset clean

up: ## Start Postgres
	docker compose up -d
	@echo "Waiting for Postgres..."
	@until docker compose exec postgres pg_isready -U blog -d blog -q 2>/dev/null; do sleep 1; done
	@echo "Postgres is ready."

down: ## Stop Postgres
	docker compose down

setup: up ## First-time setup: start DB, init module, generate code
	@command -v kiln >/dev/null 2>&1 || { echo "Installing kiln..."; go install github.com/fisayoafolayan/kiln/cmd/kiln@latest; }
	@test -f .env || cp .env.example .env
	@test -f go.mod || go mod init blog-api
	@set -a && . ./.env && set +a && kiln generate
	go mod tidy
	@echo ""
	@echo "Done! Run: make run"

generate: ## Regenerate code after schema changes
	@set -a && . ./.env && set +a && kiln generate

run: ## Start the server
	@if [ ! -f .env ]; then echo "Copy .env.example to .env first: cp .env.example .env"; exit 1; fi
	@set -a && . ./.env && set +a && go run cmd/server/main.go

seed: ## Re-apply seed data
	docker compose exec -T postgres psql -U blog -d blog < seed.sql

clean: ## Remove all generated code (use after changing kiln.yaml)
	rm -rf generated cmd models factory dbinfo dberrors docs

reset: down clean ## Reset everything: stop DB, remove generated code, re-setup
	docker compose down -v
	$(MAKE) setup

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"}; {printf "  %-12s %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
