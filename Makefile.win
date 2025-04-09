.PHONY: build run-api run-worker test clean \
        podman-build podman-up podman-down podman-migrate podman-logs podman-run \
        init-win dev-win prod-run

#############################
# Local Build & Test Commands
#############################

# Build the application binaries (Windows-specific commands)
build:
	if not exist $(BUILD_DIR) mkdir $(BUILD_DIR)
	go build -o $(BUILD_DIR)\$(API_BINARY).exe .\cmd\api
	go build -o $(BUILD_DIR)\$(WORKER_BINARY).exe .\cmd\worker

# Run the API locally
run-api:
	go run .\cmd\api

# Run the Worker locally
run-worker:
	go run .\cmd\worker

# Run tests
test:
	go test -v .\...

# Clean the build directory
clean:
	if exist $(BUILD_DIR) rmdir /s /q $(BUILD_DIR)

#############################
# Podman Container Commands (Development)
#############################

# Build containers with --no-cache using the development profile
podman-build:
	podman-compose --profile dev build --no-cache

# Bring up containers with development profile
podman-up:
	podman-compose --profile dev up -d

# Shut down containers using the development profile
podman-down:
	podman-compose --profile dev down

# Run database migrations using the development profile
podman-migrate:
	podman-compose run migrations

# Follow container logs (development profile)
podman-logs:
	podman-compose logs -f

# Combined target to build and bring up containers (development)
podman-run: podman-build podman-up

#############################
# Windows Development Environment Initialization
#############################

# Initialize the Windows dev environment: copy .env.example to .env and build containers
init-win:
	cmd /c copy .env.example .env
	podman-compose build --no-cache

# All-in-one command for Windows development environment
dev-win: init-win podman-up

#############################
# Production Target (All Services)
#############################

# Production target: start all containers without specifying a profile
prod-run:
	podman-compose up -d

