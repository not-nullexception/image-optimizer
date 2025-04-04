.PHONY: build run test clean docker-build docker-up docker-down migrate-up migrate-down lint docker-migrate


# Go parameters
BINARY_NAME=image-optimizer
API_BINARY=api
WORKER_BINARY=worker
BUILD_DIR=./build
DOCKER_COMPOSE=docker-compose.yml

# Build the application
build:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(API_BINARY) ./cmd/api
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(WORKER_BINARY) ./cmd/worker 

# Run the application
run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

# Run tests
test:
	go test -v ./...

# Clean the build directory
clean:
	rm -rf $(BUILD_DIR)

# Docker
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-migrate:
	docker-compose run migrations

# Database migrations
migrate-up:
	migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/image_optimizer?sslmode=disable" up

migrate-down:
	migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/image_optimizer?sslmode=disable" down

# Lint the code
lint:
	golangci-lint run

# Generate mock files for testing
generate-mocks:
	mockgen -source=./internal/db/repository.go -destination=./internal/db/mock/repository_mock.go -package=mock
	mockgen -source=./internal/minio/client.go -destination=./internal/minio/mock/client_mock.go -package=mock
	mockgen -source=./internal/queue/rabbitmq/client.go -destination=./internal/queue/rabbitmq/mock/client_mock.go -package=mock

# Install necessary dependencies
deps:
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/golang/mock/mockgen@latest

# Run the application in Docker
docker-run: docker-build docker-up

# Initialize development environment
init: deps
	cp .env.example .env
	make docker-build

# All-in-one command to start development
dev: init docker-up