# Makefile for GoCron project

.PHONY: help test test-unit test-integration test-coverage clean build run docker-build docker-run

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Test targets
test: ## Run all tests
	@echo "Running all tests..."
	go test -v ./...

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	go test -v ./internal/...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v ./integration_test.go

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	go test -v -race ./...

test-short: ## Run tests in short mode
	@echo "Running tests in short mode..."
	go test -v -short ./...

# Specific test targets
test-models: ## Run models tests
	@echo "Running models tests..."
	go test -v ./internal/models/

test-config: ## Run config tests
	@echo "Running config tests..."
	go test -v ./internal/config/

test-api: ## Run API tests
	@echo "Running API tests..."
	go test -v ./internal/api/

test-worker: ## Run worker tests
	@echo "Running worker tests..."
	go test -v ./internal/worker/

test-storage: ## Run storage tests
	@echo "Running storage tests..."
	go test -v ./internal/storage/postgres/

test-scheduler: ## Run scheduler tests
	@echo "Running scheduler tests..."
	go test -v ./internal/scheduler/

test-main: ## Run main function tests
	@echo "Running main function tests..."
	go test -v ./cmd/gocron/

# Build targets
build: ## Build the application
	@echo "Building application..."
	go build -o bin/gocron ./cmd/gocron/

build-linux: ## Build for Linux
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o bin/gocron-linux ./cmd/gocron/

build-windows: ## Build for Windows
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 go build -o bin/gocron.exe ./cmd/gocron/

build-darwin: ## Build for macOS
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build -o bin/gocron-darwin ./cmd/gocron/

build-all: build-linux build-windows build-darwin ## Build for all platforms

# Run targets
run: ## Run the application
	@echo "Running application..."
	go run ./cmd/gocron/

run-dev: ## Run in development mode with hot reload (requires air)
	@echo "Running in development mode..."
	air -c .air.toml

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t gocron:latest .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 --env-file .env gocron:latest

docker-test: ## Run tests in Docker
	@echo "Running tests in Docker..."
	docker run --rm -v $(PWD):/app -w /app golang:1.24 make test

# Database targets
db-migrate-up: ## Run database migrations up
	@echo "Running database migrations up..."
	migrate -path migrations -database "${DATABASE_URL}" up

db-migrate-down: ## Run database migrations down
	@echo "Running database migrations down..."
	migrate -path migrations -database "${DATABASE_URL}" down

db-migrate-create: ## Create new migration (usage: make db-migrate-create NAME=migration_name)
	@echo "Creating migration: $(NAME)"
	migrate create -ext sql -dir migrations $(NAME)

# Development targets
deps: ## Install dependencies
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

# Generate targets
generate: ## Run go generate
	@echo "Running go generate..."
	go generate ./...

sqlc-generate: ## Generate SQLC code
	@echo "Generating SQLC code..."
	sqlc generate

# Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean ./...

clean-cache: ## Clean go cache
	@echo "Cleaning go cache..."
	go clean -cache
	go clean -testcache
	go clean -modcache

# Security targets
security-scan: ## Run security scan
	@echo "Running security scan..."
	gosec ./...

vuln-check: ## Check for vulnerabilities
	@echo "Checking for vulnerabilities..."
	govulncheck ./...

# Benchmark targets
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

bench-cpu: ## Run CPU benchmarks
	@echo "Running CPU benchmarks..."
	go test -bench=. -cpuprofile=cpu.prof ./...

bench-mem: ## Run memory benchmarks
	@echo "Running memory benchmarks..."
	go test -bench=. -memprofile=mem.prof ./...

# Documentation targets
docs: ## Generate documentation
	@echo "Generating documentation..."
	godoc -http=:6060

# Environment setup
setup-dev: ## Setup development environment
	@echo "Setting up development environment..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# CI/CD targets
ci-test: ## Run tests for CI/CD
	@echo "Running CI/CD tests..."
	go test -v -race -coverprofile=coverage.out ./...

ci-build: ## Build for CI/CD
	@echo "Building for CI/CD..."
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/gocron ./cmd/gocron/

ci-lint: ## Run linting for CI/CD
	@echo "Running CI/CD linting..."
	golangci-lint run --timeout=5m

# Performance targets
profile-cpu: ## Profile CPU usage
	@echo "Profiling CPU usage..."
	go test -cpuprofile=cpu.prof -bench=. ./...
	go tool pprof cpu.prof

profile-mem: ## Profile memory usage
	@echo "Profiling memory usage..."
	go test -memprofile=mem.prof -bench=. ./...
	go tool pprof mem.prof

# Database test setup
test-db-setup: ## Setup test database
	@echo "Setting up test database..."
	@echo "Make sure PostgreSQL is running and create a test database"
	@echo "Example: createdb cron_test"
	@echo "Set TEST_DATABASE_URL environment variable"

test-with-db: test-db-setup ## Run tests with real database
	@echo "Running tests with real database..."
	TEST_DATABASE_URL="postgres://user:password@localhost:5432/cron_test?sslmode=disable" go test -v ./...

# All quality checks
check-all: fmt vet lint test-race security-scan ## Run all quality checks

# Release targets
release-test: ## Test release build
	@echo "Testing release build..."
	make clean
	make build
	./bin/gocron --version || echo "Version check failed"

tag-release: ## Tag a new release (usage: make tag-release VERSION=v1.0.0)
	@echo "Tagging release: $(VERSION)"
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

# Show test results in different formats
test-json: ## Run tests with JSON output
	@echo "Running tests with JSON output..."
	go test -json ./... > test-results.json

test-verbose: ## Run tests with verbose output
	@echo "Running tests with verbose output..."
	go test -v -count=1 ./...

# Load testing (if you have load testing tools)
load-test: ## Run load tests (requires custom load testing setup)
	@echo "Running load tests..."
	@echo "Implement your load testing here"

# Monitoring and metrics
metrics: ## Show code metrics
	@echo "Code metrics:"
	@echo "Lines of code:"
	@find . -name "*.go" -not -path "./vendor/*" | xargs wc -l | tail -1
	@echo "Number of packages:"
	@go list ./... | wc -l
	@echo "Number of tests:"
	@grep -r "func Test" --include="*_test.go" . | wc -l