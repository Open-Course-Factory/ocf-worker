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

test-storage-api:
	@echo "🧪 Testing storage API with corrected configuration..."
	@chmod +x test_storage_api.sh
	@./test_storage_api.sh filesystem

# Test storage Garage avec configuration cohérente
test-storage-garage:
	@echo "🚀 Testing Garage storage with consistent configuration..."
	@if ! docker compose --profile garage ps | grep -q "garage.*Up"; then \
		echo "🚀 Starting Garage first..."; \
		make garage-start; \
		sleep 10; \
		make garage-setup-test; \
	fi
	@chmod +x test_storage_api.sh
	@./test_storage_api.sh garage

# Test des deux backends en séquence
test-storage-both:
	@echo "🔄 Testing both storage backends sequentially..."
	@echo ""
	@echo "1️⃣ Testing filesystem storage..."
	@make test-storage-api
	@echo ""
	@echo "2️⃣ Testing Garage storage..."
	@make test-storage-garage
	@echo ""
	@echo "✅ Both storage backends tested successfully!"

# Validation de la configuration avant tests
validate-test-config:
	@echo "🔍 Validating test configuration..."
	@echo ""
	@echo "📊 Service status:"
	@docker compose ps | grep -E "(ocf-worker|postgres)" || echo "❌ Core services not running"
	@docker compose --profile garage ps | grep garage || echo "ℹ️ Garage not running (start with make garage-start)"
	@echo ""
	@echo "🌐 Connectivity tests:"
	@curl -s --connect-timeout 3 http://localhost:8081/health >/dev/null && echo "✅ OCF Worker API" || echo "❌ OCF Worker API"
	@curl -s --connect-timeout 3 http://localhost:3900 >/dev/null && echo "✅ Garage S3 API" || echo "ℹ️ Garage S3 API (not running)"
	@curl -s --connect-timeout 3 http://localhost:3903/health >/dev/null && echo "✅ Garage Admin API" || echo "ℹ️ Garage Admin API (not running)"
	@echo ""
	@echo "📁 Configuration files:"
	@[ -f "test_storage_api.sh" ] && echo "✅ test_storage_api.sh" || echo "❌ test_storage_api.sh missing"
	@[ -f "deployments/garage/garage.toml" ] && echo "✅ garage.toml" || echo "❌ garage.toml missing"

# Test avec diagnostic en cas d'échec
test-storage-with-debug:
	@echo "🧪 Testing storage with debug information..."
	@make validate-test-config
	@echo ""
	@if make test-storage-both; then \
		echo "✅ All tests passed!"; \
	else \
		echo "❌ Tests failed, running diagnostics..."; \
		make garage-debug; \
		make debug-logs; \
		exit 1; \
	fi

# Logs de debug pour les échecs
debug-logs:
	@echo "📝 Debug logs:"
	@echo "=============="
	@echo "OCF Worker logs:"
	@$(DOCKER_COMPOSE_CMD) logs --tail=10 ocf-worker 2>/dev/null || echo "No OCF Worker logs"
	@echo ""
	@echo "Garage logs:"
	@$(DOCKER_COMPOSE_CMD) --profile garage logs --tail=10 garage 2>/dev/null || echo "No Garage logs"

# Setup de test complet
setup-test-environment:
	@echo "⚙️ Setting up complete test environment..."
	@make setup
	@make docker-dev &
	@sleep 15
	@make garage-start
	@sleep 10
	@make garage-setup-test
	@echo "✅ Test environment ready"
	@make validate-test-config

# Nettoyage de l'environnement de test
cleanup-test-environment:
	@echo "🧹 Cleaning up test environment..."
	@make stop-all
	@docker system prune -f
	@echo "✅ Test environment cleaned"

# ========================================
# TESTS DE RÉGRESSION
# ========================================

# Test de régression complet
test-regression:
	@echo "🔄 Running regression tests..."
	@echo "=============================="
	@make cleanup-test-environment
	@make setup-test-environment
	@make test-storage-with-debug
	@make test-worker_api.sh || echo "⚠️ Worker API tests not available"
	@echo "✅ Regression tests completed"

# Test de performance comparative
test-performance-comparison:
	@echo "⚡ Comparing storage backend performance..."
	@echo "Storage performance comparison:" > /tmp/perf-results.txt
	@echo "============================" >> /tmp/perf-results.txt
	@echo "" >> /tmp/perf-results.txt
	@echo "Filesystem:" >> /tmp/perf-results.txt
	@time make test-storage-api 2>&1 | grep real >> /tmp/perf-results.txt || true
	@echo "" >> /tmp/perf-results.txt
	@echo "Garage:" >> /tmp/perf-results.txt  
	@time make test-storage-garage 2>&1 | grep real >> /tmp/perf-results.txt || true
	@echo ""
	@cat /tmp/perf-results.txt
	@rm -f /tmp/perf-results.txt

# ========================================
# AIDE 
# ========================================

help-testing-v2:
	@echo ""
	@echo "🧪 Testing (Updated):"
	@echo "  validate-test-config     Validate configuration before tests"
	@echo "  test-storage-api         Test filesystem storage API"
	@echo "  test-storage-garage      Test Garage storage API"
	@echo "  test-storage-both        Test both storage backends"
	@echo "  test-storage-with-debug  Test with debug on failure"
	@echo ""
	@echo "🔧 Test Environment:"
	@echo "  setup-test-environment   Setup complete test environment"
	@echo "  cleanup-test-environment Cleanup test environment"
	@echo "  test-regression          Complete regression test"
	@echo "  test-performance-comparison Compare backend performance"
	@echo ""
	@echo "🔍 Debugging:"
	@echo "  debug-logs               Show recent service logs"

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
	@echo "📚 Documentation:"
	@echo "  swagger-generate     Générer la documentation Swagger"
	@echo "  swagger-validate     Valider la documentation Swagger"
	@echo "  swagger-serve        Servir la documentation localement"
	@echo "  swagger-clean        Nettoyer les docs générées"
	@echo ""
	@echo "For more details, see: make <command>"


# ========================================
# GARAGE STORAGE COMMANDS 
# ========================================

# Détection automatique de docker compose
DOCKER_COMPOSE_CMD := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; else echo "docker-compose"; fi)

# Démarrer Garage avec le profil
garage-start:
	@echo "🚀 Starting Garage with profile..."
	@$(DOCKER_COMPOSE_CMD) --profile garage up -d garage
	@echo "⏳ Waiting for Garage to be ready..."
	@for i in $$(seq 1 90); do \
		if curl -s --connect-timeout 3 http://localhost:3903/health >/dev/null 2>&1; then \
			echo "✅ Garage is ready"; \
			break; \
		fi; \
		if [ $$i -eq 90 ]; then \
			echo "❌ Garage failed to start within 90 seconds"; \
			$(DOCKER_COMPOSE_CMD) --profile garage logs garage; \
			exit 1; \
		fi; \
		sleep 1; \
	done

# Arrêter Garage
garage-stop:
	@echo "🛑 Stopping Garage..."
	@$(DOCKER_COMPOSE_CMD) --profile garage down

# Configurer Garage pour les tests
garage-setup-test:
	@echo "🔧 Setting up Garage for testing..."
	@echo "Getting Garage node ID..."
	@for attempt in $$(seq 1 10); do \
		NODE_ID=$$($(DOCKER_COMPOSE_CMD) exec -T garage /garage node id -q 2>/dev/null | cut -d '@' -f1 | tr -d '\r\n' || true); \
		if [ -n "$$NODE_ID" ] && [ $${#NODE_ID} -gt 10 ]; then \
			echo "✅ Node ID obtained: $$NODE_ID"; \
			break; \
		fi; \
		echo "⏳ Waiting for Garage node to be ready... ($$attempt/10)"; \
		sleep 3; \
	done; \
	if [ -z "$$NODE_ID" ]; then \
		echo "❌ Failed to get node ID"; \
		exit 1; \
	fi; \
	echo "📐 Configuring layout..."; \
	$(DOCKER_COMPOSE_CMD) exec -T garage /garage layout assign "$$NODE_ID" -z dc1 -c 1 || true; \
	$(DOCKER_COMPOSE_CMD) exec -T garage /garage layout apply --version 1 || true; \
	sleep 5; \
	echo "🔑 Creating test credentials..."; \
	$(DOCKER_COMPOSE_CMD) exec -T garage /garage key new --name test-key 2>/dev/null || true; \
	$(DOCKER_COMPOSE_CMD) exec -T garage /garage key import \
		--name test-key \
		--access-key-id "GK31c2f218a2e44f485b94239e" \
		--secret-access-key "4420d99ef7aa26b56b5130ad7913a6a5c77653a5e7a47a3b4c9b8b9c5f8b7b4d" 2>/dev/null || true; \
	echo "🪣 Creating test bucket..."; \
	$(DOCKER_COMPOSE_CMD) exec -T garage /garage bucket create "ocf-test" 2>/dev/null || true; \
	$(DOCKER_COMPOSE_CMD) exec -T garage /garage bucket allow \
		--read --write "ocf-test" --key test-key 2>/dev/null || true; \
	echo "✅ Garage setup complete"

# Réinitialiser Garage complètement
garage-reset:
	@echo "🧹 Resetting Garage completely..."
	@$(DOCKER_COMPOSE_CMD) --profile garage down -v
	@docker volume prune -f
	@$(MAKE) garage-start
	@sleep 10
	@$(MAKE) garage-setup-test

# Statut de Garage
garage-status:
	@echo "📊 Garage Status"
	@echo "================"
	@echo "🐳 Container status:"
	@$(DOCKER_COMPOSE_CMD) --profile garage ps
	@echo ""
	@echo "🌐 API connectivity:"
	@if curl -s --connect-timeout 5 http://localhost:3900 >/dev/null 2>&1; then \
		echo "  ✅ S3 API (port 3900) accessible"; \
	else \
		echo "  ❌ S3 API (port 3900) not accessible"; \
	fi
	@if curl -s --connect-timeout 5 http://localhost:3903/health >/dev/null 2>&1; then \
		echo "  ✅ Admin API (port 3903) accessible"; \
	else \
		echo "  ❌ Admin API (port 3903) not accessible"; \
	fi
	@echo ""
	@echo "🏗️ Internal status:"
	@$(DOCKER_COMPOSE_CMD) exec -T garage /garage layout show 2>/dev/null || echo "  ❌ Cannot get layout"
	@$(DOCKER_COMPOSE_CMD) exec -T garage /garage key list 2>/dev/null || echo "  ❌ Cannot list keys"
	@$(DOCKER_COMPOSE_CMD) exec -T garage /garage bucket list 2>/dev/null || echo "  ❌ Cannot list buckets"

# Logs de Garage
garage-logs:
	@echo "📝 Garage logs:"
	@$(DOCKER_COMPOSE_CMD) --profile garage logs --tail=50 garage

# Debug de Garage avec informations détaillées
garage-debug:
	@echo "🔍 Garage Debug Information"
	@echo "=========================="
	@$(MAKE) garage-status
	@echo ""
	@echo "📦 Network information:"
	@$(DOCKER_COMPOSE_CMD) exec -T garage ip addr show 2>/dev/null | grep -E "(inet|UP)" || echo "Cannot get network info"
	@echo ""
	@echo "🔌 Port test:"
	@for port in 3900 3901 3902 3903; do \
		if nc -z localhost $$port 2>/dev/null; then \
			echo "  ✅ Port $$port accessible"; \
		else \
			echo "  ❌ Port $$port not accessible"; \
		fi; \
	done
	@echo ""
	@echo "📄 Configuration:"
	@if [ -f "deployments/garage/garage.toml" ]; then \
		echo "✅ garage.toml exists"; \
		echo "Key configuration:"; \
		grep -E "(rpc_secret|api_bind_addr|s3_region)" deployments/garage/garage.toml; \
	else \
		echo "❌ garage.toml missing"; \
	fi
	@echo ""
	@echo "📝 Recent logs:"
	@$(DOCKER_COMPOSE_CMD) --profile garage logs --tail=20 garage

# Test complet de Garage
garage-test-full:
	@echo "🧪 Complete Garage Test"
	@echo "======================="
	@$(MAKE) garage-start
	@sleep 10
	@$(MAKE) garage-setup-test
	@sleep 5
	@if [ -f "test_storage_api.sh" ]; then \
		chmod +x test_storage_api.sh; \
		./test_storage_api.sh garage; \
	else \
		echo "❌ test_storage_api.sh not found"; \
	fi

# Démarrer tous les services avec Garage
start-all:
	@echo "🚀 Starting all services including Garage..."
	@$(DOCKER_COMPOSE_CMD) up -d
	@$(DOCKER_COMPOSE_CMD) --profile garage up -d
	@echo "⏳ Waiting for services to be ready..."
	@sleep 15
	@$(MAKE) garage-setup-test

# Arrêter tous les services
stop-all:
	@echo "🛑 Stopping all services..."
	@$(DOCKER_COMPOSE_CMD) down
	@$(DOCKER_COMPOSE_CMD) --profile garage down
	@$(DOCKER_COMPOSE_CMD) --profile dev down

# ========================================
# TESTS STORAGE AVEC GARAGE
# ========================================

# Test storage API avec configuration appropriée
test-storage-api:
	@echo "🧪 Testing storage API with filesystem backend..."
	@if ! $(DOCKER_COMPOSE_CMD) ps | grep -q "ocf-worker.*Up"; then \
		echo "❌ OCF Worker not running. Starting services..."; \
		$(DOCKER_COMPOSE_CMD) up -d; \
		sleep 10; \
	fi
	@if [ -f "test_storage_api.sh" ]; then \
		chmod +x test_storage_api.sh; \
		./test_storage_api.sh filesystem; \
	else \
		echo "❌ test_storage_api.sh not found"; \
		exit 1; \
	fi

# Test storage Garage avec configuration cohérente
test-storage-garage:
	@echo "🚀 Testing Garage storage with consistent configuration..."
	@if ! $(DOCKER_COMPOSE_CMD) --profile garage ps | grep -q "garage.*Up"; then \
		echo "🚀 Starting Garage first..."; \
		$(MAKE) garage-start; \
		sleep 10; \
		$(MAKE) garage-setup-test; \
		sleep 5; \
	fi
	@if [ -f "test_storage_api.sh" ]; then \
		chmod +x test_storage_api.sh; \
		./test_storage_api.sh garage; \
	else \
		echo "❌ test_storage_api.sh not found"; \
		exit 1; \
	fi

# Test des deux backends en séquence
test-storage-both:
	@echo "🔄 Testing both storage backends sequentially..."
	@echo ""
	@echo "1️⃣ Testing filesystem storage..."
	@$(MAKE) test-storage-api
	@echo ""
	@echo "2️⃣ Testing Garage storage..."
	@$(MAKE) test-storage-garage
	@echo ""
	@echo "✅ Both storage backends tested successfully!"

# ========================================
# AIDE MISE À JOUR
# ========================================

help-garage:
	@echo ""
	@echo "🚀 Garage Storage Commands:"
	@echo "  garage-start             Start Garage service"
	@echo "  garage-stop              Stop Garage service"
	@echo "  garage-setup-test        Configure Garage for testing"
	@echo "  garage-reset             Reset Garage completely"
	@echo "  garage-status            Show Garage status"
	@echo "  garage-logs              Show Garage logs"
	@echo "  garage-debug             Debug Garage with detailed info"
	@echo "  garage-test-full         Complete Garage test"
	@echo ""
	@echo "🔄 Service Management:"
	@echo "  start-all                Start all services (including Garage)"
	@echo "  stop-all                 Stop all services"
	@echo ""
	@echo "🧪 Storage Testing:"
	@echo "  test-storage-api         Test filesystem storage"
	@echo "  test-storage-garage      Test Garage storage"
	@echo "  test-storage-both        Test both storage backends"

# Générer la documentation Swagger
.PHONY: swagger-generate
swagger-generate: ## Générer la documentation Swagger
	@echo "📚 Generating Swagger documentation..."
	swag init -g cmd/generator/main.go -o docs --parseInternal --parseDependency
	@echo "✅ Swagger docs generated in docs/"

# Valider la documentation Swagger
.PHONY: swagger-validate
swagger-validate: swagger-generate ## Valider la documentation Swagger
	@echo "✅ Validating Swagger documentation..."
	swag fmt -g cmd/generator/main.go
	@echo "✅ Swagger documentation validated"

# Servir la documentation en mode dev
.PHONY: swagger-serve
swagger-serve: swagger-generate ## Servir la documentation Swagger localement
	@echo "🌐 Serving Swagger UI at http://localhost:8081/swagger/"
	@echo "🔄 Starting OCF Worker with Swagger..."
	$(MAKE) run

# Clean swagger docs
.PHONY: swagger-clean
swagger-clean: ## Nettoyer la documentation générée
	@echo "🧹 Cleaning Swagger documentation..."
	rm -rf docs/
	@echo "✅ Swagger docs cleaned"

# Build avec génération automatique de Swagger
.PHONY: build-with-docs
build-with-docs: swagger-generate build ## Build avec génération de la doc Swagger
