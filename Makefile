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

# Docker commands
docker-dev:
	./scripts/dev.sh

docker-dev-hot:
	./scripts/dev-hot.sh

docker-prod:
	./scripts/prod.sh

docker-stop:
	./scripts/stop.sh

# Utility commands
logs:
	docker-compose logs -f

logs-db:
	docker-compose logs -f postgres-worker

shell-worker:
	docker-compose exec ocf-worker sh

shell-db:
	docker-compose exec postgres-worker psql -U ocf_worker -d ocf_worker_db

# Database operations
db-migrate:
	docker-compose exec ocf-worker ocf-worker migrate

db-backup:
	docker-compose exec postgres-worker pg_dump -U ocf_worker -d ocf_worker_db > backup_$(shell date +%Y%m%d_%H%M%S).sql

db-restore:
	@echo "Usage: make db-restore FILE=backup_file.sql"
	@if [ -z "$(FILE)" ]; then echo "❌ FILE parameter required"; exit 1; fi
	docker-compose exec -T postgres-worker psql -U ocf_worker -d ocf_worker_db < $(FILE)
