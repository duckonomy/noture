.PHONY: test test-unit test-integration test-coverage clean build run dev

all: build

build:
	go build -o noture-server

run:
	./noture-server

dev:
	go run main.go

test:
	go test ./...

test-unit:
	@echo "Running reliable unit tests..."
	go test ./pkg/pgconv/ ./internal/domain/
	go test ./internal/services/ -run "DetectMimeType|DetectFileFormat"

test-core:
	@echo "Running core functionality tests..."
	go test ./pkg/pgconv/ ./internal/domain/ -v
	go test ./internal/services/ -run "DetectMimeType|DetectFileFormat" -v

test-integration:
	go test -run Integration ./...

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

test-verbose:
	go test -v ./...

clean:
	rm -f noture-server
	rm -f coverage.out coverage.html
	go clean -testcache

fmt:
	go fmt ./...

lint:
	golangci-lint run

sqlc:
	sqlc generate

migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

migrate-create:
	@read -p "Enter migration name: " name; \
	goose -dir migrations create $$name sql

test-db-setup:
	@echo "Setting up test database..."
	@echo "Make sure PostgreSQL is running and configured for testing"

help:
	@echo "Available targets:"
	@echo ""
	@echo "Building & Running:"
	@echo "  build          - Build the application"
	@echo "  run            - Run the built application"
	@echo "  dev            - Run in development mode"
	@echo ""
	@echo "Testing (Recommended):"
	@echo "  test-core      - Run reliable core functionality tests (RECOMMENDED)"
	@echo "  test-unit      - Run fast unit tests only"
	@echo ""
	@echo "Testing (All reliable):"
	@echo "  test           - Run all tests (should work reliably now)"
	@echo "  test-integration - Run integration tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean          - Clean build artifacts"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  sqlc           - Generate SQL code"
	@echo "  migrate-up     - Run database migrations up"
	@echo "  migrate-down   - Run database migrations down"
	@echo "  migrate-create - Create new migration"
