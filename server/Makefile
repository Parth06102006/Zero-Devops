# Load environment variables from .env file if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

DATABASE_USER ?= buildapp
DATABASE_PASS ?= buildapp
DATABASE_PORT ?= 5432
DATABASE_NAME ?= buildapp

# Database configuration
POSTGRES_USER     ?= $(DATABASE_USER)
POSTGRES_PASSWORD ?= $(DATABASE_PASS)
POSTGRES_ADDRESS  ?= 127.0.0.1:$(DATABASE_PORT)
POSTGRES_DATABASE ?= $(DATABASE_NAME)

# RabbitMQ
RABBITMQ_CONNECTION_STRING ?= amqp://buildapp:buildapp@127.0.0.1:5672/

# Exporting bin folder to the path for makefile
export PATH   := $(CURDIR)/bin:$(PATH)
# Default Shell
export SHELL  := bash
# Type of OS: Linux or Darwin. Lowercased for download consistency.
export OSTYPE := $(shell uname -s | tr '[:upper:]' '[:lower:]')


# --- Tooling & Variables ----------------------------------------------------------------
include ./misc/make/tools.Makefile
include ./misc/make/help.Makefile

# ~~~ Development Environment ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

up: dev-air                     ## Startup / Spinup Docker Compose and air
down: docker-stop               ## Stop Docker
destroy: docker-teardown clean  ## Teardown (removes volumes, tmp files, etc...)

install-deps: goose air gotestsum tparse mockery ## Install Development Dependencies (localy).
deps: $(GOOSE) $(AIR) $(GOTESTSUM) $(TPARSE) $(MOCKERY) ## Checks for Global Development Dependencies.
deps:
	@echo "Required Tools Are Available"

dev-env: ## Bootstrap Environment (with a Docker-Compose help).
	@ docker-compose up -d --build pgsql
	@ echo "Waiting for pgsql to be healthy..."
	@ until docker inspect --format='{{.State.Health.Status}}' zero_devops_pgsql 2>/dev/null | grep -q "healthy"; do sleep 1; done
	@ if [ -n "$$(docker ps -q -f name=zero_devops_rabbitmq)" ]; then \
		echo "RabbitMQ is already running."; \
	elif [ -n "$$(docker ps -a -q -f name=zero_devops_rabbitmq)" ]; then \
		echo "Starting existing RabbitMQ container..."; \
		docker start zero_devops_rabbitmq; \
	else \
		echo "Creating and starting RabbitMQ container..."; \
		docker-compose up -d --build rabbitmq; \
	fi
	@ echo "Waiting for rabbitmq to be healthy..."
	@ until docker inspect --format='{{.State.Health.Status}}' zero_devops_rabbitmq 2>/dev/null | grep -q "healthy"; do sleep 1; done

dev-env-test: dev-env ## Run application (within a Docker-Compose help)
	@ $(MAKE) image-build
	docker-compose up web

dev-air: dev-env build $(AIR) ## Starts AIR ( Continuous Development app).
	@ if [ "$(OS)" = "Windows_NT" ]; then \
		export PATH="$$PATH:/c/Windows/System32/WindowsPowerShell/v1.0"; \
	fi
	air

docker-stop:
	@ docker-compose down

docker-teardown:
	@ docker-compose down --remove-orphans -v

# ~~~ Code Actions ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

lint: $(GOLANGCI) ## Runs golangci-lint with predefined configuration
	@echo "Applying linter"
	golangci-lint version
	golangci-lint run -c .golangci.yaml ./...

# -trimpath - will remove the filepathes from the reports, good to same money on network trafic,
#             focus on bug reports, and find issues fast.
# - race    - adds a racedetector, in case of racecondition, you can catch report with sentry.
#             https://golang.org/doc/articles/race_detector.html
#
# todo(butuzov): add additional flags to compiler to have an `version` flag.
build: ## Builds binary
	@ printf "Building aplication... "
	@ go build -trimpath -o engine$(EXE) ./cmd/server/
	@ echo "done"

build-race: ## Builds binary (with -race flag)
	@ printf "Building aplication with race flag... "
	@ go build -trimpath -race -o engine$(EXE) ./cmd/server/
	@ echo "done"


go-generate: $(MOCKERY) ## Runs go generte ./...
	go generate ./...

TESTS_ARGS := --format testname --jsonfile gotestsum.json.out
TESTS_ARGS += --max-fails 2
TESTS_ARGS += -- ./...
TESTS_ARGS += -test.parallel 2
TESTS_ARGS += -test.count    1
TESTS_ARGS += -test.failfast
TESTS_ARGS += -test.coverprofile   coverage.out
TESTS_ARGS += -test.timeout        5s
TESTS_ARGS += -race

run-tests: $(GOTESTSUM)
	@ gotestsum $(TESTS_ARGS) -short

tests: run-tests $(TPARSE) ## Run Tests & parse details
	@cat gotestsum.json.out | $(TPARSE) -all -top -notests

# ~~~ Docker Build ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.ONESHELL:
image-build:
	@ echo "Docker Build"
	@ DOCKER_BUILDKIT=0 docker build --file Dockerfile --tag go-clean-arch .

# ~~~ Database Migrations ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

POSTGRES_DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_ADDRESS)/$(POSTGRES_DATABASE)?sslmode=disable
export GOOSE_DRIVER ?= postgres
export GOOSE_DBSTRING ?= $(POSTGRES_DSN)

migrate-up: $(GOOSE) ## Apply all (or N up) migrations.
	@ read -p "How many migration you wants to perform (default value: [all]): " N; \
	if [ -z "$${N}" ]; then \
		goose -dir migrations up; \
	else \
		goose -dir migrations up $${N}; \
	fi

.PHONY: migrate-down
migrate-down: $(GOOSE) ## Apply all (or N down) migrations.
	@ read -p "How many migration you wants to perform (default value: [all]): " N; \
	if [ -z "$${N}" ]; then \
		goose -dir migrations down; \
	else \
		goose -dir migrations down $${N}; \
	fi

.PHONY: migrate-drop
migrate-drop: $(GOOSE) ## Drop everything inside the database.
	goose -dir migrations reset

.PHONY: migrate-create
migrate-create: $(GOOSE) ## Create a set of up/down migrations with a specified name.
	@ read -p "Please provide name for the migration: " Name; \
	goose create $${Name} -dir migrations --sql-header "-- +goose Up\n-- +goose Down"

# ~~~ Cleans ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

clean: clean-artifacts clean-docker

clean-artifacts: ## Removes Artifacts (*.out)
	@printf "Cleanning artifacts... "
	@rm -f *.out
	@echo "done."

clean-docker: ## Removes dangling docker images
	@ docker image prune -f
