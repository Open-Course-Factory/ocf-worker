.PHONY: build test run clean docker-build

# Variables
APP_NAME=ocf-worker
PORT=8081

# Build the application
build:
	go build -o bin/$(APP_NAME) cmd/generator/main.go

# Run tests
test:
	go test -v ./...

# Run the application
run:
	go run cmd/generator/main.go

# Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf storage/

# Docker build
docker-build:
	docker build -f deployments/docker/Dockerfile -t $(APP_NAME):latest .

# Run with docker-compose (development)
docker-dev:
	docker-compose -f deployments/docker/docker-compose.dev.yml up --build

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Generate docs (requires swag: go install github.com/swaggo/swag/cmd/swag@latest)
docs:
	swag init -g cmd/generator/main.go

# Install dependencies
deps:
	go mod download
	go mod tidy

# Setup development environment
setup: deps
	cp .env.example .env
	mkdir -p storage logs
