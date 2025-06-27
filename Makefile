.PHONY: build test run clean docker-build worker-*

# Variables
APP_NAME=ocf-worker
PORT=8081

# Build the application
build:
	go build -o bin/$(APP_NAME) cmd/generator/main.go

# Run tests
test:
	go test -v ./...

# Run tests including worker package
test-all:
	go test -v ./internal/worker/
	go test -v ./internal/api/
	go test -v ./internal/storage/...
	go test -v ./internal/config/

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
	rm -rf workspaces/
	rm -rf logs/

# Docker build
docker-build:
	docker build -f deployments/docker/Dockerfile -t $(APP_NAME):latest .

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
	mkdir -p storage logs workspaces
	chmod 755 storage logs workspaces

# ========================================
# WORKER COMMANDS - NEW IN v3.4
# ========================================

# Start worker with proper permissions
worker-start:
	./scripts/start-worker.sh

# Fix permissions issues
worker-fix-permissions:
	./scripts/fix-permissions.sh

# Fix Slidev issues
worker-fix-slidev:
	./scripts/fix-slidev.sh

# Debug Slidev installation
worker-debug-slidev:
	./scripts/debug-slidev.sh

# Test worker functionality
worker-test:
	./test_worker_api.sh

# Test basic API (compatibility)
worker-test-api:
	./test_storage_api.sh

# Worker development commands
worker-dev:
	./scripts/dev.sh

worker-dev-hot:
	./scripts/dev-hot.sh

worker-prod:
	./scripts/prod.sh

worker-stop:
	./scripts/stop.sh

# ========================================
# DOCKER COMMANDS (Updated)
# ========================================

# Development with worker
docker-dev: setup
	docker-compose up --build -d

# Development with hot reload and worker
docker-dev-hot: setup
	docker-compose --profile dev up --build -d

# Production with worker
docker-prod: setup
	docker-compose -f docker-compose.prod.yml up --build -d

# Stop all services
docker-stop:
	docker-compose down
	docker-compose --profile dev down
	docker-compose -f docker-compose.prod.yml down

# Clean everything including volumes
docker-clean:
	docker-compose down -v
	docker-compose --profile dev down -v
	docker system prune -f
	docker volume prune -f

# ========================================
# UTILITY COMMANDS
# ========================================

# View logs
logs:
	docker-compose logs -f

logs-worker:
	docker-compose logs -f ocf-worker

logs-db:
	docker-compose logs -f postgres-worker

# Shell access
shell-worker:
	docker-compose exec ocf-worker sh

shell-db:
	docker-compose exec postgres-worker psql -U ocf_worker -d ocf_worker_db

# ========================================
# MONITORING COMMANDS
# ========================================

# Check worker health
worker-health:
	curl -s http://localhost:8081/api/v1/worker/health | jq .

# Check worker stats
worker-stats:
	curl -s http://localhost:8081/api/v1/worker/stats | jq .

# Check overall health
health:
	curl -s http://localhost:8081/health | jq .

# Check storage info
storage-info:
	curl -s http://localhost:8081/api/v1/storage/info | jq .

# ========================================
# DATABASE OPERATIONS
# ========================================

# Database operations
db-migrate:
	docker-compose exec ocf-worker ocf-worker migrate

db-backup:
	docker-compose exec postgres-worker pg_dump -U ocf_worker -d ocf_worker_db > backup_$(shell date +%Y%m%d_%H%M%S).sql

db-restore:
	@echo "Usage: make db-restore FILE=backup_file.sql"
	@if [ -z "$(FILE)" ]; then echo "❌ FILE parameter required"; exit 1; fi
	docker-compose exec -T postgres-worker psql -U ocf_worker -d ocf_worker_db < $(FILE)

# ========================================
# TESTING COMMANDS
# ========================================

# Complete test suite
test-complete: test-all worker-test

# Integration tests
test-integration:
	@echo "🧪 Running integration tests..."
	docker-compose up -d
	sleep 10
	./test_storage_api.sh
	./test_worker_api.sh
	docker-compose down

# Performance test (basic)
test-performance:
	@echo "⚡ Running basic performance test..."
	./scripts/performance-test.sh

# ========================================
# DEVELOPMENT HELPERS
# ========================================

# Quick restart during development
restart: docker-stop docker-dev

# Reset everything (clean start)
reset: docker-clean setup docker-dev

# Show project status
status:
	@echo "📊 OCF Worker Status:"
	@echo "===================="
	@docker-compose ps || echo "Services not running"
	@echo ""
	@echo "🔍 Quick health check:"
	@curl -s http://localhost:8081/health 2>/dev/null | jq -r '.status // "API not available"' || echo "API not available"
	@echo ""
	@echo "📈 Worker stats:"
	@curl -s http://localhost:8081/api/v1/worker/stats 2>/dev/null | jq -r '.worker_pool.running // "Worker not available"' || echo "Worker not available"

# Show help
help:
	@echo "OCF Worker - Available Commands"
	@echo "==============================="
	@echo ""
	@echo "🏗️  Building:"
	@echo "  build                 Build the application"
	@echo "  docker-build         Build Docker image"
	@echo ""
	@echo "🧪 Testing:"
	@echo "  test                 Run unit tests"
	@echo "  test-all             Run all tests including worker"
	@echo "  worker-test          Test worker functionality"
	@echo "  test-integration     Full integration test"
	@echo ""
	@echo "🚀 Development:"
	@echo "  setup                Setup development environment"
	@echo "  worker-start         Start worker with proper setup"
	@echo "  worker-dev           Start in development mode"
	@echo "  worker-dev-hot       Start with hot reload"
	@echo "  restart              Quick restart services"
	@echo ""
	@echo "🔧 Maintenance:"
	@echo "  worker-fix-permissions Fix workspace permissions"
	@echo "  clean                Clean build artifacts"
	@echo "  docker-clean         Clean Docker resources"
	@echo "  reset                Complete reset"
	@echo ""
	@echo "📊 Monitoring:"
	@echo "  status               Show project status"
	@echo "  worker-health        Check worker health"
	@echo "  worker-stats         Show worker statistics"
	@echo "  logs                 Show all logs"
	@echo "  logs-worker          Show worker logs only"
	@echo ""
	@echo "For more details, see: make <command>"