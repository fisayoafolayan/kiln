# kiln Makefile - FOR KILN CONTRIBUTORS ONLY
# ─────────────────────────────────────────────────────────────────────────────
# This Makefile is for developing kiln itself.
# If you are a kiln user, you don't need this.
# Just run: kiln init && kiln generate
# ─────────────────────────────────────────────────────────────────────────────

BINARY       := kiln
VERSION      ?= dev
MODULE       := github.com/fisayoafolayan/kiln
BUILD_FLAGS  := -ldflags "-X $(MODULE)/internal/cmd.Version=$(VERSION)"

# ─────────────────────────────────────────────────────────────────────────────
# Database config
# ─────────────────────────────────────────────────────────────────────────────

# Postgres
PG_USER      := kiln
PG_PASS      := kiln
PG_DB        := blog
PG_PORT      := 5432
PG_DSN       := postgres://$(PG_USER):$(PG_PASS)@localhost:$(PG_PORT)/$(PG_DB)?sslmode=disable
PG_CONTAINER := kiln-test-postgres

# MySQL
MY_USER      := kiln
MY_PASS      := kiln
MY_DB        := blog
MY_PORT      := 3306
MY_DSN       := $(MY_USER):$(MY_PASS)@tcp(localhost:$(MY_PORT))/$(MY_DB)?parseTime=true
MY_CONTAINER := kiln-test-mysql

# SQLite (no container needed - just a file)
SQLITE_FILE  := $(PWD)/testdata/blog.db
SQLITE_DSN   := $(SQLITE_FILE)

# bob generator binaries (one per database dialect)
BOBGEN_PG     := bobgen-psql
BOBGEN_MYSQL  := bobgen-mysql
BOBGEN_SQLITE := bobgen-sqlite

# ─────────────────────────────────────────────────────────────────────────────
# Build
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: build
build: ## Build the kiln binary
	go build $(BUILD_FLAGS) -o $(BINARY) ./cmd/kiln

.PHONY: install
install: ## Install kiln to $GOPATH/bin
	go install $(BUILD_FLAGS) ./cmd/kiln

.PHONY: clean
clean: ## Remove build artifacts and test output
	rm -f $(BINARY)
	rm -rf testdata/e2e
	rm -f $(SQLITE_FILE)

# ─────────────────────────────────────────────────────────────────────────────
# Bob generators (dev internals)
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: bob/install/psql
bob/install/psql: ## Install bobgen-psql
	go install github.com/stephenafamo/bob/gen/bobgen-psql@latest
	@echo "  ✓ bobgen-psql installed"

.PHONY: bob/install/mysql
bob/install/mysql: ## Install bobgen-mysql
	go install github.com/stephenafamo/bob/gen/bobgen-mysql@latest
	@echo "  ✓ bobgen-mysql installed"

.PHONY: bob/install/sqlite
bob/install/sqlite: ## Install bobgen-sqlite
	go install github.com/stephenafamo/bob/gen/bobgen-sqlite@latest
	@echo "  ✓ bobgen-sqlite installed"

.PHONY: bob/install
bob/install: bob/install/psql bob/install/mysql bob/install/sqlite ## Install all bob generator binaries (contributors only)
	@echo "  ✓ All bob generators installed"

# ─────────────────────────────────────────────────────────────────────────────
# Postgres
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: pg/start
pg/start: ## Start the Postgres test container
	@if docker ps -q -f name=$(PG_CONTAINER) | grep -q .; then \
		echo "  $(PG_CONTAINER) is already running"; \
	else \
		docker run --rm -d \
			--name $(PG_CONTAINER) \
			-e POSTGRES_USER=$(PG_USER) \
			-e POSTGRES_PASSWORD=$(PG_PASS) \
			-e POSTGRES_DB=$(PG_DB) \
			-p $(PG_PORT):5432 \
			postgres:16; \
		echo "  Waiting for Postgres to be ready..."; \
		until docker exec $(PG_CONTAINER) pg_isready \
			-U $(PG_USER) -d $(PG_DB) --quiet 2>/dev/null; do \
			printf "."; \
			sleep 1; \
		done; \
		echo ""; \
		echo "  ✓ $(PG_CONTAINER) started and ready"; \
	fi

.PHONY: pg/stop
pg/stop: ## Stop the Postgres test container
	docker stop $(PG_CONTAINER) 2>/dev/null || true
	@echo "  ✓ $(PG_CONTAINER) stopped"

.PHONY: pg/migrate
pg/migrate: ## Apply the blog schema to Postgres
	@echo "  Applying Postgres schema..."
	@docker cp testdata/schemas/blog.postgres.sql $(PG_CONTAINER):/tmp/blog.sql
	@docker exec -e PGOPTIONS="-c client_min_messages=warning" \
		$(PG_CONTAINER) psql -U $(PG_USER) -d $(PG_DB) -f /tmp/blog.sql -q
	@echo "  ✓ Postgres schema applied"

.PHONY: pg/reset
pg/reset: pg/stop pg/start pg/migrate ## Reset Postgres (stop, start, migrate)

.PHONY: pg/psql
pg/psql: ## Open a psql shell against the test database
	docker exec -it $(PG_CONTAINER) psql -U $(PG_USER) -d $(PG_DB)

# ─────────────────────────────────────────────────────────────────────────────
# MySQL
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: my/start
my/start: ## Start the MySQL test container
	@if docker ps -q -f name=$(MY_CONTAINER) | grep -q .; then \
		echo "  $(MY_CONTAINER) is already running"; \
	else \
		docker run --rm -d \
			--name $(MY_CONTAINER) \
			-e MYSQL_USER=$(MY_USER) \
			-e MYSQL_PASSWORD=$(MY_PASS) \
			-e MYSQL_DATABASE=$(MY_DB) \
			-e MYSQL_ROOT_PASSWORD=root \
			-p $(MY_PORT):3306 \
			mysql:8; \
		echo "  Waiting for MySQL to be ready..."; \
		until docker exec $(MY_CONTAINER) mysqladmin ping \
			-h 127.0.0.1 -u$(MY_USER) -p$(MY_PASS) --silent 2>/dev/null; do \
			printf "."; \
			sleep 1; \
		done; \
		echo ""; \
		sleep 2; \
		echo "  ✓ $(MY_CONTAINER) started and ready"; \
	fi

.PHONY: my/stop
my/stop: ## Stop the MySQL test container
	docker stop $(MY_CONTAINER) 2>/dev/null || true
	@echo "  ✓ $(MY_CONTAINER) stopped"

.PHONY: my/migrate
my/migrate: ## Apply the blog schema to MySQL
	@echo "  Applying MySQL schema..."
	@docker cp testdata/schemas/blog.mysql.sql $(MY_CONTAINER):/tmp/blog.sql
	@docker exec $(MY_CONTAINER) mysql \
		-u $(MY_USER) -p$(MY_PASS) $(MY_DB) -e "source /tmp/blog.sql"
	@echo "  ✓ MySQL schema applied"

.PHONY: my/reset
my/reset: my/stop my/start my/migrate ## Reset MySQL (stop, start, migrate)

.PHONY: my/shell
my/shell: ## Open a mysql shell against the test database
	docker exec -it $(MY_CONTAINER) mysql -u $(MY_USER) -p$(MY_PASS) $(MY_DB)

# ─────────────────────────────────────────────────────────────────────────────
# SQLite (no container needed)
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: sqlite/migrate
sqlite/migrate: ## Apply the blog schema to SQLite
	@echo "  Applying SQLite schema..."
	@mkdir -p testdata
	@sqlite3 $(SQLITE_FILE) < testdata/schemas/blog.sqlite.sql
	@echo "  ✓ SQLite schema applied ($(SQLITE_FILE))"

.PHONY: sqlite/reset
sqlite/reset: ## Reset the SQLite database
	rm -f $(SQLITE_FILE)
	$(MAKE) sqlite/migrate

.PHONY: sqlite/shell
sqlite/shell: ## Open a sqlite3 shell
	sqlite3 $(SQLITE_FILE)

# ─────────────────────────────────────────────────────────────────────────────
# Stop all containers
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: db/stop/all
db/stop/all: pg/stop my/stop ## Stop all test containers

# ─────────────────────────────────────────────────────────────────────────────
# End-to-end tests
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: e2e/postgres
e2e/postgres: build bob/install/psql pg/start pg/migrate ## Run e2e test against Postgres
	@$(MAKE) _e2e \
		DRIVER=postgres \
		DSN="$(PG_DSN)" \
		BOB_GEN=psql \
		OUT=testdata/e2e/postgres

.PHONY: e2e/mysql
e2e/mysql: build bob/install/mysql my/start my/migrate ## Run e2e test against MySQL
	@$(MAKE) _e2e \
		DRIVER=mysql \
		DSN="$(MY_DSN)" \
		BOB_GEN=mysql \
		OUT=testdata/e2e/mysql

.PHONY: e2e/sqlite
e2e/sqlite: build bob/install/sqlite sqlite/migrate ## Run e2e test against SQLite
	@$(MAKE) _e2e \
		DRIVER=sqlite \
		DSN="$(SQLITE_DSN)" \
		BOB_GEN=sqlite \
		OUT=testdata/e2e/sqlite

.PHONY: e2e/all
e2e/all: e2e/postgres e2e/mysql e2e/sqlite ## Run e2e tests against all databases
	@echo ""
	@echo "  ✓ All e2e tests complete"
	@echo ""
	@echo "  Output:"
	@find testdata/e2e -type f | sort

# Internal target - not called directly
# Accepts: DRIVER, DSN, BOB_GEN, OUT
.PHONY: _e2e
_e2e:
	@echo ""
	@echo "  ── $(DRIVER) ──────────────────────────────────"
	$(eval E2E_DIR := testdata/e2e/$(DRIVER))
	@mkdir -p $(E2E_DIR)
	@{ \
		echo 'version: 1'; \
		echo 'database:'; \
		echo '  driver: "$(DRIVER)"'; \
		echo '  dsn: "$(DSN)"'; \
		echo 'output:'; \
		echo '  dir: "./generated"'; \
		echo '  package: generated'; \
		echo 'api:'; \
		echo '  base_path: "/api/v1"'; \
		echo '  framework: chi'; \
		echo 'bob:'; \
		echo '  enabled: true'; \
		echo '  models_dir: "./models"'; \
		echo 'generate:'; \
		echo '  models: true'; \
		echo '  store: true'; \
		echo '  handlers: true'; \
		echo '  router: true'; \
		echo '  openapi: true'; \
		echo 'openapi:'; \
		echo '  enabled: true'; \
		echo '  output: "./docs/openapi.yaml"'; \
		echo '  title: "Blog API ($(DRIVER))"'; \
		echo '  version: "1.0.0"'; \
		echo 'tables:'; \
		echo '  exclude:'; \
		echo '    - post_tags'; \
	} > $(E2E_DIR)/kiln.yaml
	@{ \
		echo '$(BOB_GEN):'; \
		echo '  dsn: "$(DSN)"'; \
		echo '  output: "./models"'; \
		echo '  pkg_name: "models"'; \
	} > $(E2E_DIR)/bobgen.yaml
	@echo "  Generated bobgen.yaml:"
	@cat $(E2E_DIR)/bobgen.yaml
	@echo ""
	cd $(E2E_DIR) && go mod init blogapi 2>/dev/null || true
	cd $(E2E_DIR) && ../../../kiln generate
	cd $(E2E_DIR) && go get github.com/stephenafamo/bob@v0.42.0
	cd $(E2E_DIR) && go get github.com/aarondl/opt
	cd $(E2E_DIR) && go get github.com/gofrs/uuid/v5
	@if [ "$(DRIVER)" = "postgres" ]; then \
		cd $(E2E_DIR) && go get github.com/jackc/pgx/v5; \
	elif [ "$(DRIVER)" = "mysql" ]; then \
		cd $(E2E_DIR) && go get github.com/go-sql-driver/mysql; \
	elif [ "$(DRIVER)" = "sqlite" ]; then \
		cd $(E2E_DIR) && go get github.com/mattn/go-sqlite3; \
	fi
	cd $(E2E_DIR) && go mod tidy
	@echo "  ✓ $(DRIVER) output written to $(E2E_DIR)/generated/"

.PHONY: e2e/inspect
e2e/inspect: ## List all generated files from the last e2e run
	@find testdata/e2e -type f | sort

.PHONY: e2e/clean
e2e/clean: ## Remove all e2e test output
	rm -rf testdata/e2e
	@echo "  ✓ e2e output cleaned"

# ─────────────────────────────────────────────────────────────────────────────
# Development
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run unit tests
	go test ./...

.PHONY: test/short
test/short: ## Run unit tests skipping compilation tests
	go test -short ./...

.PHONY: test/verbose
test/verbose: ## Run unit tests with verbose output
	go test -v ./...

.PHONY: test/cover
test/cover: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "  ✓ Coverage report written to coverage.html"

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: tidy
tidy: ## Tidy go modules
	go mod tidy

.PHONY: fmt
fmt: ## Format all Go source files
	gofmt -w .

# ─────────────────────────────────────────────────────────────────────────────
# Help
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help message
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Postgres:"
	@grep -E '^pg/[a-zA-Z_%-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-22s %s\n", $$1, $$2}'
	@echo ""
	@echo "MySQL:"
	@grep -E '^my/[a-zA-Z_%-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-22s %s\n", $$1, $$2}'
	@echo ""
	@echo "SQLite:"
	@grep -E '^sqlite/[a-zA-Z_%-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-22s %s\n", $$1, $$2}'
	@echo ""
	@echo "End-to-end:"
	@grep -E '^e2e/[a-zA-Z_%-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-22s %s\n", $$1, $$2}'
	@echo ""
	@echo "Build & dev:"
	@grep -E '^(build|install|clean|test|test/short|test/verbose|test/cover|lint|tidy|fmt|db/stop/all):.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-22s %s\n", $$1, $$2}'
	@echo ""
	@echo "Dev internals (contributors only):"
	@grep -E '^(bob/install):.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-22s %s\n", $$1, $$2}'
	@echo ""

.DEFAULT_GOAL := help
