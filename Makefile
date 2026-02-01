.PHONY: all build run test clean docker-up docker-down server cli benchmark seed

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
SERVER_BINARY=bin/server
CLI_BINARY=bin/fanout

# Build targets
all: build

build: build-server build-cli

build-server:
	$(GOBUILD) -o $(SERVER_BINARY) ./cmd/server

build-cli:
	$(GOBUILD) -o $(CLI_BINARY) ./cmd/cli

run: docker-up
	$(GOBUILD) -o $(SERVER_BINARY) ./cmd/server
	./$(SERVER_BINARY)

server: build-server
	./$(SERVER_BINARY)

cli: build-cli
	./$(CLI_BINARY)

# Docker commands
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Development
dev: docker-up
	air

# Testing
test:
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Benchmarking shortcuts
benchmark:
	./$(CLI_BINARY) benchmark --strategy all --tweets 1000 --concurrent 50

seed:
	./$(CLI_BINARY) seed --users 10000 --avg-followers 150 --celebrities 50

# Dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Cleanup
clean:
	rm -rf bin/
	docker-compose down -v

# Web UI
web-install:
	cd web && pnpm install

web-dev:
	cd web && pnpm dev

web-build:
	cd web && pnpm build

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build both server and CLI"
	@echo "  build-server - Build API server"
	@echo "  build-cli    - Build CLI tool"
	@echo "  run          - Start docker and run server"
	@echo "  server       - Run API server"
	@echo "  cli          - Run CLI tool"
	@echo "  docker-up    - Start PostgreSQL and Redis"
	@echo "  docker-down  - Stop PostgreSQL and Redis"
	@echo "  test         - Run tests"
	@echo "  benchmark    - Run benchmark suite"
	@echo "  seed         - Seed database with test data"
	@echo "  clean        - Clean build artifacts and volumes"
	@echo "  web-dev      - Start web UI dev server"
	@echo "  web-build    - Build web UI for production"
