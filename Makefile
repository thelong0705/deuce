# Local secrets and overrides, kept out of the repo. Absent is fine: every flag
# below has a default that works against docker compose.
ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE   := docker compose
DB_USER   := deuce
DB_NAME   ?= deuce
CONTAINER := deuce-postgres
DB_SERVICE := postgres
DB_VOLUME  := deuce-pgdata
REDIS_SERVICE := redis
TEST_DB_NAME := deuce_test
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

.PHONY: db-start db-down db-wait db-psql db-test-create db-reset \
        redis-start redis-down redis-wait redis-cli dev \
        migrate-up migrate-down migrate-version \
        sqlc-gen build test test-cover mocks \
        lint fmt fmt-check \
        server web web-install

db-start:
	$(COMPOSE) up -d $(DB_SERVICE)
	@$(MAKE) db-wait

db-down:
	$(COMPOSE) down $(DB_SERVICE)

db-wait:
	@echo "waiting for postgres..."
	@until $(COMPOSE) exec -T postgres pg_isready -U $(DB_USER) -d $(DB_NAME) >/dev/null 2>&1; \
		do sleep 0.5; done
	@echo "postgres ready"

db-psql:
	$(COMPOSE) exec -it postgres psql -U $(DB_USER) -d $(DB_NAME)

db-test-create:
	@$(COMPOSE) exec -T postgres createdb -U $(DB_USER) $(TEST_DB_NAME) 2>/dev/null \
		|| echo "$(TEST_DB_NAME) already exists"

db-reset:
	$(COMPOSE) down $(DB_SERVICE)
	-@docker volume rm $(DB_VOLUME) >/dev/null 2>&1
	@$(MAKE) db-start
	@$(MAKE) db-test-create
	@$(MAKE) migrate-up
	@$(MAKE) migrate-up DB_NAME=$(TEST_DB_NAME)

redis-start:
	$(COMPOSE) up -d $(REDIS_SERVICE)
	@$(MAKE) redis-wait

redis-down:
	$(COMPOSE) down $(REDIS_SERVICE)

redis-wait:
	@echo "waiting for redis..."
	@until $(COMPOSE) exec -T $(REDIS_SERVICE) redis-cli ping >/dev/null 2>&1; \
		do sleep 0.5; done
	@echo "redis ready"

redis-cli:
	$(COMPOSE) exec -it $(REDIS_SERVICE) redis-cli

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down 1

migrate-version:
	$(MIGRATE) version

sqlc-gen:
	docker run --rm -v "$(PWD):/src" -w /src sqlc/sqlc:$(SQLC_VERSION) generate

COVERAGE_TOOL := github.com/vladopajic/go-test-coverage/v2@v2.19.0

build:
	go build -v ./...

# dev brings up everything the server needs and then runs it. The server pings
# both on startup and stops if either is missing, so they are waited for first.
dev:
	@$(MAKE) db-start
	@$(MAKE) redis-start
	@$(MAKE) server

# SERVER_FLAGS is what the server is configured with. A flag left empty is the
# same as not passing it, so an absent .env still starts a working server.
SERVER_FLAGS = $(strip -db-url="$(DB_URL)" \
	$(if $(HTTP_ADDR),-http-addr="$(HTTP_ADDR)") \
	$(if $(REDIS_ADDR),-redis-addr="$(REDIS_ADDR)"))

server:
	go run ./cmd/deuce $(SERVER_FLAGS)

web-install:
	npm --prefix web install

web:
	npm --prefix web run dev

test:
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
