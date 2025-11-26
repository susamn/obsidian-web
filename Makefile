.PHONY: help install build test clean dev docker-build docker-up docker-down

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install dependencies (backend and frontend)
	@echo "Installing Go dependencies..."
	go mod download
	@echo "Installing frontend dependencies..."
	cd web && npm install

format: ## Format code both frontend and backend
	@echo "Formatting Go code..."
	go fmt ./...
	@echo "Formatting frontend code..."
	cd web && npm run format

build: format ## Build backend and frontend
	@echo "Building backend..."
	go build -o bin/server ./cmd/server
	@echo "Building frontend..."
	cd web && npm run build
	@echo "Copying frontend to backend..."
	rm -rf internal/public
	mkdir -p internal/public
	cp -r web/dist/* internal/public
	@echo "Copying themes to backend..."
	mkdir -p internal/public/assets/themes
	cp -r web/src/assets/themes/*.css internal/public/assets/themes/

run: build dev-backend ## Build and run the application


test: ## Run all tests
	@echo "Running backend tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Running frontend tests..."
	cd web && npm run test

test-coverage: ## Run tests with coverage report
	@echo "Running backend tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Running frontend tests with coverage..."
	cd web && npm run test:coverage

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf web/dist/
	rm -f coverage.out coverage.html
	cd web && rm -rf node_modules/

dev-backend: ## Run backend in development mode
	go run ./cmd/server/main.go

dev-frontend: ## Run frontend in development mode
	cd web && npm run dev

dev: ## Run both backend and frontend in development mode (requires terminal multiplexer)
	@echo "Start backend with: make dev-backend"
	@echo "Start frontend with: make dev-frontend"

docker-build: ## Build Docker images
	docker build -f docker/Dockerfile.backend -t obsidian-web-backend .
	docker build -f docker/Dockerfile.frontend -t obsidian-web-frontend ./web

docker-up: ## Start Docker containers
	docker-compose -f docker/docker-compose.yaml up -d

docker-down: ## Stop Docker containers
	docker-compose -f docker/docker-compose.yaml down

docker-logs: ## Show Docker logs
	docker-compose -f docker/docker-compose.yaml logs -f

lint: ## Run linters
	@echo "Linting Go code..."
	go vet ./...
	@echo "Linting frontend code..."
	cd web && npm run lint

fmt: ## Format code
	@echo "Formatting Go code..."
	go fmt ./...
	@echo "Formatting frontend code..."
	cd web && npm run format

tidy: ## Tidy Go modules
	go mod tidy
	go mod verify
