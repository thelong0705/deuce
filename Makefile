COMPOSE   := docker compose
DB_USER   := deuce
DB_NAME   := deuce
CONTAINER := deuce-postgres
MIGRATIONS_DIR := db/postgres/migration
MIGRATE_IMAGE := migrate/migrate:v4.17.1
DB_URL        := postgres://$(DB_USER):$(DB_USER)@localhost:5432/$(DB_NAME)?sslmode=disable
SQLC_VERSION := 1.30.0
MIGRATE_NETWORK ?= container:$(CONTAINER)

MIGRATE := docker run --rm \
	-v "$(PWD)/$(MIGRATIONS_DIR):/migration" \
	--network $(MIGRATE_NETWORK) \
	$(MIGRATE_IMAGE) \
	-path=/migration -database "$(DB_URL)"

.PHONY: db-start db-down db-wait db-psql \
        migrate-up migrate-down migrate-drop migrate-version \
        sqlc-gen test test-cover mocks \
        lint fmt fmt-check

db-start:
	$(COMPOSE) up -d
	@$(MAKE) db-wait

db-down:
	$(COMPOSE) down

db-wait:
	@echo "waiting for postgres..."
	@until $(COMPOSE) exec -T postgres pg_isready -U $(DB_USER) -d $(DB_NAME) >/dev/null 2>&1; \
		do sleep 0.5; done
	@echo "postgres ready"

db-psql:
	$(COMPOSE) exec -it postgres psql -U $(DB_USER) -d $(DB_NAME)

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down 1

migrate-drop:
	$(MIGRATE) drop -f

migrate-version:
	$(MIGRATE) version

sqlc-gen:
	docker run --rm -v "$(PWD):/src" -w /src sqlc/sqlc:$(SQLC_VERSION) generate

COVERAGE_TOOL := github.com/vladopajic/go-test-coverage/v2@v2.19.0

test:
	go test -race -v -coverprofile=coverage.out ./...

cover-check:
	go run $(COVERAGE_TOOL) --config=.testcoverage.yml

test-cover: test cover-check

# Rules and exclusions live in .golangci.yml
GOLANGCI_IMAGE := golangci/golangci-lint:v2.13.2
GOLANGCI := docker run --rm -v "$(PWD):/app" -w /app $(GOLANGCI_IMAGE) golangci-lint

lint:
	$(GOLANGCI) run

# Rewrites files in place.
fmt:
	$(GOLANGCI) fmt

fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt'd, run 'make fmt':"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "all files are gofmt'd"

MOCKERY_VERSION := v2.53.7

mocks:
	go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION)
