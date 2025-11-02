.PHONY: help build run test clean migrate docker-up docker-down dev

# Variables
SERVICE_NAME=driver-location-service
BINARY_NAME=driver-location-service
GO=go
GOTEST=$(GO) test
GOBUILD=$(GO) build
GOCLEAN=$(GO) clean
GOMOD=$(GO) mod

# Colors for output
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m
COLOR_BLUE=\033[34m

help: ## Display this help message
	@echo "$(COLOR_BOLD)Driver Location Service - Available Commands:$(COLOR_RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""

install: ## Install dependencies
	@echo "$(COLOR_BLUE)Installing dependencies...$(COLOR_RESET)"
	cd ../../.. && $(GOMOD) download
	cd ../../.. && $(GOMOD) tidy
	@echo "$(COLOR_GREEN)✓ Dependencies installed$(COLOR_RESET)"

build: ## Build the application
	@echo "$(COLOR_BLUE)Building $(SERVICE_NAME)...$(COLOR_RESET)"
	cd ../../.. && $(GOBUILD) -o ./services/$(SERVICE_NAME)/bin/$(BINARY_NAME) ./services/$(SERVICE_NAME)/cmd/main.go
	@echo "$(COLOR_GREEN)✓ Build complete: bin/$(BINARY_NAME)$(COLOR_RESET)"

run: ## Run the application
	@echo "$(COLOR_BLUE)Starting $(SERVICE_NAME)...$(COLOR_RESET)"
	cd ../../.. && $(GO) run ./services/$(SERVICE_NAME)/cmd/main.go

dev: ## Run with hot reload (requires air)
	@echo "$(COLOR_BLUE)Starting development server with hot reload...$(COLOR_RESET)"
	@if command -v air > /dev/null; then \
		cd ../../.. && air -c ./services/$(SERVICE_NAME)/.air.toml; \
	else \
		echo "$(COLOR_YELLOW)⚠ 'air' not installed. Install with: go install github.com/cosmtrek/air@latest$(COLOR_RESET)"; \
		echo "$(COLOR_YELLOW)Falling back to regular run...$(COLOR_RESET)"; \
		$(MAKE) run; \
	fi

test: ## Run tests
	@echo "$(COLOR_BLUE)Running tests...$(COLOR_RESET)"
	cd ../../.. && $(GOTEST) -v -race -coverprofile=coverage.out ./services/$(SERVICE_NAME)/...
	@echo "$(COLOR_GREEN)✓ Tests complete$(COLOR_RESET)"

test-coverage: test ## Run tests with coverage report
	@echo "$(COLOR_BLUE)Generating coverage report...$(COLOR_RESET)"
	cd ../../.. && $(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)✓ Coverage report: coverage.html$(COLOR_RESET)"

lint: ## Run linter
	@echo "$(COLOR_BLUE)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint > /dev/null; then \
		cd ../../.. && golangci-lint run ./services/$(SERVICE_NAME)/...; \
		echo "$(COLOR_GREEN)✓ Linting complete$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ 'golangci-lint' not installed$(COLOR_RESET)"; \
		echo "Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin"; \
	fi

fmt: ## Format code
	@echo "$(COLOR_BLUE)Formatting code...$(COLOR_RESET)"
	cd ../../.. && $(GO) fmt ./services/$(SERVICE_NAME)/...
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

vet: ## Run go vet
	@echo "$(COLOR_BLUE)Running go vet...$(COLOR_RESET)"
	cd ../../.. && $(GO) vet ./services/$(SERVICE_NAME)/...
	@echo "$(COLOR_GREEN)✓ Vet complete$(COLOR_RESET)"

clean: ## Clean build artifacts
	@echo "$(COLOR_BLUE)Cleaning...$(COLOR_RESET)"
	cd ../../.. && $(GOCLEAN)
	rm -f ./bin/$(BINARY_NAME)
	rm -f coverage.out coverage.html
	@echo "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

docker-build: ## Build Docker image
	@echo "$(COLOR_BLUE)Building Docker image...$(COLOR_RESET)"
	cd ../../.. && docker build -t $(SERVICE_NAME):latest -f ./services/$(SERVICE_NAME)/Dockerfile .
	@echo "$(COLOR_GREEN)✓ Docker image built$(COLOR_RESET)"

docker-up: ## Start Docker Compose services (production)
	@echo "$(COLOR_BLUE)Starting Docker Compose services...$(COLOR_RESET)"
	docker-compose up -d
	@echo "$(COLOR_GREEN)✓ Services started$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Waiting for services to be ready...$(COLOR_RESET)"
	@sleep 10
	@echo "$(COLOR_GREEN)Services ready!$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Access URLs:$(COLOR_RESET)"
	@echo "  Service:     http://localhost:8082"
	@echo "  RabbitMQ UI: http://localhost:15672 (guest/guest)"
	@echo ""

docker-down: ## Stop Docker Compose services
	@echo "$(COLOR_BLUE)Stopping Docker Compose services...$(COLOR_RESET)"
	docker-compose down
	@echo "$(COLOR_GREEN)✓ Services stopped$(COLOR_RESET)"

docker-logs: ## View Docker Compose logs
	docker-compose logs -f

docker-dev-up: ## Start development environment with all tools
	@echo "$(COLOR_BLUE)Starting development environment...$(COLOR_RESET)"
	docker-compose -f docker-compose.dev.yml up -d
	@echo "$(COLOR_GREEN)✓ Development environment started$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Waiting for services to be ready...$(COLOR_RESET)"
	@sleep 10
	@echo "$(COLOR_GREEN)Development environment ready!$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Access URLs:$(COLOR_RESET)"
	@echo "  Service:     http://localhost:8082"
	@echo "  RabbitMQ UI: http://localhost:15672 (guest/guest)"
	@echo "  pgAdmin:     http://localhost:5050 (admin@ridehail.com/admin)"
	@echo "  Redis:       localhost:6379"
	@echo "  Mailhog:     http://localhost:8025"
	@echo ""

docker-dev-down: ## Stop development environment
	@echo "$(COLOR_BLUE)Stopping development environment...$(COLOR_RESET)"
	docker-compose -f docker-compose.dev.yml down
	@echo "$(COLOR_GREEN)✓ Development environment stopped$(COLOR_RESET)"

docker-dev-logs: ## View development environment logs
	docker-compose -f docker-compose.dev.yml logs -f

docker-restart: ## Restart Docker Compose services
	@echo "$(COLOR_BLUE)Restarting services...$(COLOR_RESET)"
	docker-compose restart
	@echo "$(COLOR_GREEN)✓ Services restarted$(COLOR_RESET)"

docker-rebuild: ## Rebuild and restart services
	@echo "$(COLOR_BLUE)Rebuilding and restarting services...$(COLOR_RESET)"
	docker-compose down
	cd ../../.. && docker build -t $(SERVICE_NAME):latest -f ./services/$(SERVICE_NAME)/Dockerfile .
	docker-compose up -d
	@echo "$(COLOR_GREEN)✓ Services rebuilt and restarted$(COLOR_RESET)"

docker-clean: ## Remove all containers, volumes, and images
	@echo "$(COLOR_BLUE)Cleaning up Docker resources...$(COLOR_RESET)"
	docker-compose down -v
	docker-compose -f docker-compose.dev.yml down -v
	docker rmi $(SERVICE_NAME):latest 2>/dev/null || true
	@echo "$(COLOR_GREEN)✓ Cleanup complete$(COLOR_RESET)"

docker-ps: ## Show running containers
	@echo "$(COLOR_BOLD)Running containers:$(COLOR_RESET)"
	@docker-compose ps

docker-stats: ## Show container resource usage
	@docker stats --no-stream $$(docker-compose ps -q)

migrate-up: ## Run database migrations up
	@echo "$(COLOR_BLUE)Running migrations up...$(COLOR_RESET)"
	@if [ -z "$(DATABASE_URL)" ]; then \
		export DATABASE_URL="postgres://postgres:postgres@localhost:5432/ride_hail?sslmode=disable"; \
	fi; \
	psql "$$DATABASE_URL" -f ../../../migrations/driver_location_service.sql
	@echo "$(COLOR_GREEN)✓ Migrations complete$(COLOR_RESET)"

migrate-down: ## Run database migrations down
	@echo "$(COLOR_BLUE)Rolling back migrations...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)⚠ Manual rollback required - check migration files$(COLOR_RESET)"

db-setup: docker-up ## Setup database with migrations
	@echo "$(COLOR_BLUE)Setting up database...$(COLOR_RESET)"
	@sleep 3
	$(MAKE) migrate-up
	@echo "$(COLOR_GREEN)✓ Database setup complete$(COLOR_RESET)"

rabbitmq-setup: docker-up ## Setup RabbitMQ exchanges and queues
	@echo "$(COLOR_BLUE)Setting up RabbitMQ...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)RabbitMQ will auto-configure on first connection$(COLOR_RESET)"
	@echo "$(COLOR_GREEN)✓ RabbitMQ ready$(COLOR_RESET)"

setup: install db-setup rabbitmq-setup ## Complete setup (install deps, db, rabbitmq)
	@echo "$(COLOR_GREEN)✓ Complete setup finished$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Next steps:$(COLOR_RESET)"
	@echo "  1. Copy .env.example to .env and configure"
	@echo "  2. Run '$(COLOR_GREEN)make run$(COLOR_RESET)' to start the service"
	@echo ""

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "$(COLOR_GREEN)✓ All checks passed$(COLOR_RESET)"

watch: ## Watch for changes and run tests
	@echo "$(COLOR_BLUE)Watching for changes...$(COLOR_RESET)"
	@if command -v watchexec > /dev/null; then \
		watchexec -e go -r -c -- make test; \
	else \
		echo "$(COLOR_YELLOW)⚠ 'watchexec' not installed$(COLOR_RESET)"; \
		echo "Install with: cargo install watchexec-cli"; \
	fi

health: ## Check service health
	@echo "$(COLOR_BLUE)Checking service health...$(COLOR_RESET)"
	@curl -f http://localhost:8082/health || echo "$(COLOR_YELLOW)⚠ Service not responding$(COLOR_RESET)"

endpoints: ## Display API endpoints
	@echo "$(COLOR_BOLD)Available API Endpoints:$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_GREEN)Health Check:$(COLOR_RESET)"
	@echo "  GET  http://localhost:8082/health"
	@echo ""
	@echo "$(COLOR_GREEN)Driver Operations:$(COLOR_RESET)"
	@echo "  POST http://localhost:8082/drivers/{id}/online"
	@echo "  POST http://localhost:8082/drivers/{id}/offline"
	@echo "  POST http://localhost:8082/drivers/{id}/location"
	@echo "  POST http://localhost:8082/drivers/{id}/start"
	@echo "  POST http://localhost:8082/drivers/{id}/complete"
	@echo "  GET  http://localhost:8082/drivers/{id}"
	@echo ""
	@echo "$(COLOR_GREEN)Internal:$(COLOR_RESET)"
	@echo "  GET  http://localhost:8082/internal/drivers/nearby"
	@echo ""
	@echo "$(COLOR_GREEN)WebSocket:$(COLOR_RESET)"
	@echo "  WS   ws://localhost:8082/ws/drivers/{id}"
	@echo ""

docs: ## Open API documentation
	@echo "$(COLOR_BLUE)Opening API documentation...$(COLOR_RESET)"
	@if [ -f API_DOCUMENTATION.md ]; then \
		if command -v mdcat > /dev/null; then \
			mdcat API_DOCUMENTATION.md | less -R; \
		else \
			cat API_DOCUMENTATION.md; \
		fi \
	else \
		echo "$(COLOR_YELLOW)⚠ API_DOCUMENTATION.md not found$(COLOR_RESET)"; \
	fi

stats: ## Show project statistics
	@echo "$(COLOR_BOLD)Project Statistics:$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_GREEN)Lines of Code:$(COLOR_RESET)"
	@find . -name '*.go' -not -path './vendor/*' | xargs wc -l | tail -1
	@echo ""
	@echo "$(COLOR_GREEN)File Count:$(COLOR_RESET)"
	@find . -name '*.go' -not -path './vendor/*' | wc -l
	@echo ""
	@echo "$(COLOR_GREEN)Test Coverage:$(COLOR_RESET)"
	@cd ../../.. && $(GO) test -cover ./services/$(SERVICE_NAME)/... 2>/dev/null | grep coverage || echo "Run 'make test' first"

version: ## Show version information
	@echo "$(COLOR_BOLD)Version Information:$(COLOR_RESET)"
	@echo "Service: $(SERVICE_NAME)"
	@echo "Go Version: $$($(GO) version)"
	@echo ""

.DEFAULT_GOAL := help
