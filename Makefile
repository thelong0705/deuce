COMPOSE   := docker compose
DB_USER   := deuce
DB_NAME   := deuce
CONTAINER := deuce-postgres
MIGRATIONS_DIR := db/postgres/migration
MIGRATE_IMAGE := migrate/migrate:v4.17.1
SQLC_VERSION := 1.30.0
MIGRATE_NETWORK ?= container:$(CONTAINER)

# Tests own a database of their own, so running them cannot disturb whatever is
# in the one the server uses.
TEST_DB_NAME ?= deuce_test

dsn = postgres://$(DB_USER):$(DB_USER)@localhost:5432/$(1)?sslmode=disable
DB_URL      := $(call dsn,$(DB_NAME))
TEST_DB_URL := $(call dsn,$(TEST_DB_NAME))

migrate = docker run --rm \
	-v "$(PWD)/$(MIGRATIONS_DIR):/migration" \
	--network $(MIGRATE_NETWORK) \
	$(MIGRATE_IMAGE) \
	-path=/migration -database "$(1)"

.PHONY: db-start db-down db-wait db-psql db-psql-test db-test-create \
        migrate-up migrate-down migrate-drop migrate-version \
        migrate-test-up migrate-test-drop \
        sqlc-gen build test test-cover mocks \
        lint fmt fmt-check \
        server web web-install

db-start:
	$(COMPOSE) up -d
	@$(MAKE) db-wait
	@$(MAKE) db-test-create

db-down:
	$(COMPOSE) down

db-wait:
	@echo "waiting for postgres..."
	@until $(COMPOSE) exec -T postgres pg_isready -U $(DB_USER) -d $(DB_NAME) >/dev/null 2>&1; \
		do sleep 0.5; done
	@echo "postgres ready"

db-psql:
	$(COMPOSE) exec -it postgres psql -U $(DB_USER) -d $(DB_NAME)

db-psql-test:
	$(COMPOSE) exec -it postgres psql -U $(DB_USER) -d $(TEST_DB_NAME)

# Postgres has no CREATE DATABASE IF NOT EXISTS, so ask first. Doing it here
# rather than in an init script means an existing volume gets the test database
# too, without being wiped.
db-test-create:
	@$(COMPOSE) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) -tAc \
		"SELECT 1 FROM pg_database WHERE datname = '$(TEST_DB_NAME)'" | grep -q 1 \
		|| $(COMPOSE) exec -T postgres createdb -U $(DB_USER) $(TEST_DB_NAME)
	@echo "test database $(TEST_DB_NAME) ready"

migrate-up:
	$(call migrate,$(DB_URL)) up

migrate-down:
	$(call migrate,$(DB_URL)) down 1

migrate-drop:
	$(call migrate,$(DB_URL)) drop -f

migrate-version:
	$(call migrate,$(DB_URL)) version

migrate-test-up:
	$(call migrate,$(TEST_DB_URL)) up

migrate-test-drop:
	$(call migrate,$(TEST_DB_URL)) drop -f

sqlc-gen:
	docker run --rm -v "$(PWD):/src" -w /src sqlc/sqlc:$(SQLC_VERSION) generate

COVERAGE_TOOL := github.com/vladopajic/go-test-coverage/v2@v2.19.0

build:
	go build -v ./...

server:
	go run ./cmd/deuce

web-install:
	npm --prefix web install

web:
	npm --prefix web run dev

# The test database is migrated first, so a new migration cannot be forgotten
# and show up as a puzzling query failure.
test: migrate-test-up
	go test -race -v -coverprofile=coverage.out ./...

cover-check:
	go run $(COVERAGE_TOOL) --config=.testcoverage.yml

test-cover: test cover-check

GOLANGCI_IMAGE := golangci/golangci-lint:v2.13.2
GOLANGCI := docker run --rm -v "$(PWD):/app" -w /app $(GOLANGCI_IMAGE) golangci-lint

lint:
	$(GOLANGCI) run

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
