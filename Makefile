.PHONY: help build proto proto-clean test test-unit test-integration test-e2e test-coverage test-cov-html test-cov-xml test-race test-verbose test-specific test-function clean clean-test docker-build docker-build-latest docker-build-versioned docker-clean docker-clean-all docker-push docker-push-latest docker-push-versioned docker-build-push docker-check

# Go configuration
GO := go
GO_VERSION := 1.24
BINARY_NAME := musicdl
BUILD_DIR := ./control

# Coverage directories
COV_DIR := coverage
COV_OUT := coverage.out
COV_HTML := $(COV_DIR)/coverage.html
COV_XML := $(COV_DIR)/coverage.xml

# Docker configuration
DOCKERFILE := musicdl.Dockerfile
IMAGE_NAME := ghcr.io/sv4u/musicdl
TAG ?= latest
DOCKER_IMAGE := $(IMAGE_NAME):$(TAG)
DOCKER_BUILD_CONTEXT := .

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: proto ## Build the musicdl binary (includes proto generation)
	@echo "Building $(BINARY_NAME)..."
	@VERSION=$$(git describe --exact-match --tags HEAD 2>/dev/null || \
		git describe --tags HEAD 2>/dev/null || \
		git rev-parse --short HEAD 2>/dev/null || \
		echo "dev"); \
	echo "Building with version: $$VERSION"; \
	$(GO) build -ldflags="-X main.Version=$$VERSION" -o $(BINARY_NAME) $(BUILD_DIR)
	@echo "✓ Built $(BINARY_NAME)"

proto: ## Generate Go code from Protocol Buffers definition
	@echo "Generating proto code..."
	@command -v protoc >/dev/null 2>&1 || (echo "✗ protoc is not installed. Install Protocol Buffers compiler: https://grpc.io/docs/protoc-installation/" && exit 1)
	@if [ ! -f "download/proto/musicdl.proto" ]; then \
		echo "✗ Error: Proto file not found: download/proto/musicdl.proto"; \
		exit 1; \
	fi
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		download/proto/musicdl.proto
	@echo "✓ Proto code generated"

proto-clean: ## Clean generated proto code
	@echo "Cleaning generated proto code..."
	@rm -f download/proto/*.pb.go
	@echo "✓ Proto code cleaned"

test: ## Run all unit tests (excludes integration and e2e by default)
	@$(GO) test ./... -tags="!integration !e2e" -v

test-unit: ## Run only unit tests (fast, no external dependencies)
	@$(GO) test ./... -v -tags="!integration !e2e"

test-integration: ## Run only integration tests (requires real APIs/credentials)
	@echo "⚠ Integration tests require Spotify API credentials"
	@$(GO) test ./... -v -tags=integration

test-e2e: ## Run only end-to-end tests (requires full environment)
	@echo "⚠ E2E tests require full environment setup"
	@$(GO) test ./... -v -tags=e2e

test-coverage: ## Run unit tests with coverage report (terminal output)
	@$(GO) test ./... -tags="!integration !e2e" -coverprofile=$(COV_OUT)
	@$(GO) tool cover -func=$(COV_OUT)

test-cov-html: ## Run unit tests and generate HTML coverage report
	@echo "Generating HTML coverage report..."
	@mkdir -p $(COV_DIR)
	@$(GO) test ./... -tags="!integration !e2e" -coverprofile=$(COV_OUT)
	@$(GO) tool cover -html=$(COV_OUT) -o $(COV_HTML)
	@echo "✓ Coverage report generated in $(COV_HTML)"

test-cov-xml: ## Run unit tests and generate XML coverage report (for CI)
	@echo "Generating XML coverage report..."
	@mkdir -p $(COV_DIR)
	@$(GO) test ./... -tags="!integration !e2e" -coverprofile=$(COV_OUT)
	@if ! command -v gocov >/dev/null 2>&1; then \
		echo "Installing gocov tools..."; \
		$(GO) install github.com/axw/gocov/gocov@latest; \
		$(GO) install github.com/AlekSi/gocov-xml@latest; \
	fi
	@gocov convert $(COV_OUT) | gocov-xml > $(COV_XML)
	@echo "✓ Coverage report generated in $(COV_XML)"

test-race: ## Run unit tests with race detector (excludes integration/e2e)
	@echo "Running unit tests with race detector..."
	@$(GO) test ./... -tags="!integration !e2e" -race -v

test-verbose: ## Run tests with verbose output
	@$(GO) test ./... -v

test-specific: ## Run specific test file (usage: make test-specific FILE=./download/service_test.go)
	@if [ -z "$(FILE)" ]; then \
		echo "Error: FILE variable required. Usage: make test-specific FILE=./download/service_test.go"; \
		exit 1; \
	fi
	@$(GO) test -v $(FILE)

test-function: ## Run specific test function (usage: make test-function FILE=./download/service_test.go FUNC=TestNewService)
	@if [ -z "$(FILE)" ] || [ -z "$(FUNC)" ]; then \
		echo "Error: FILE and FUNC variables required."; \
		echo "Usage: make test-function FILE=./download/service_test.go FUNC=TestNewService"; \
		exit 1; \
	fi
	@$(GO) test -v -run $(FUNC) $(FILE)

clean-test: ## Clean test artifacts (coverage reports, cache, etc.)
	@echo "Cleaning test artifacts..."
	@rm -rf $(COV_DIR)
	@rm -f $(COV_OUT)
	@rm -rf .pytest_cache
	@rm -rf .coverage
	@find . -type d -name __pycache__ -exec rm -r {} + 2>/dev/null || true
	@find . -type f -name "*.pyc" -delete
	@find . -type f -name "*.pyo" -delete
	@find . -type f -name "*.test" -delete
	@find . -type f -name "*.out" -delete
	@echo "✓ Test artifacts cleaned"

clean: clean-test ## Clean all generated files including test artifacts, binaries, and Docker images
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@echo "✓ Build artifacts cleaned"
	@$(MAKE) docker-clean-all

test-check: ## Check if Go is installed and version is correct
	@command -v $(GO) >/dev/null 2>&1 || (echo "✗ Go is not installed or not in PATH" && exit 1)
	@echo "✓ Go is installed: $$($(GO) version)"
	@INSTALLED_VERSION=$$($(GO) version | awk '{print $$3}' | sed 's/go//'); \
	REQUIRED_VERSION="$(GO_VERSION)"; \
	if [ "$$INSTALLED_VERSION" != "$$REQUIRED_VERSION" ]; then \
		echo "⚠ Warning: Go version $$INSTALLED_VERSION installed, but $$REQUIRED_VERSION is recommended"; \
	else \
		echo "✓ Go version matches requirement: $$REQUIRED_VERSION"; \
	fi

docker-build: ## Build Docker image (usage: make docker-build [TAG=v1.2.3])
	@echo "Building Docker image: $(DOCKER_IMAGE)"
	@if [ ! -f "$(DOCKERFILE)" ]; then \
		echo "✗ Error: Dockerfile not found: $(DOCKERFILE)"; \
		exit 1; \
	fi
	docker build -f $(DOCKERFILE) -t $(DOCKER_IMAGE) $(DOCKER_BUILD_CONTEXT)
	@echo "✓ Docker image built successfully: $(DOCKER_IMAGE)"

docker-build-latest: ## Build Docker image with :latest tag
	@$(MAKE) docker-build TAG=latest

docker-build-versioned: ## Build Docker image with versioned tag (usage: make docker-build-versioned TAG=v1.2.3)
	@if [ -z "$(TAG)" ] || [ "$(TAG)" = "latest" ]; then \
		echo "✗ Error: TAG variable required and must not be 'latest'"; \
		echo "  Usage: make docker-build-versioned TAG=v1.2.3"; \
		exit 1; \
	fi
	@if ! echo "$(TAG)" | grep -qE '^v?[0-9]+\.[0-9]+\.[0-9]+'; then \
		echo "⚠ Warning: TAG '$(TAG)' does not match semantic versioning pattern (vX.Y.Z)"; \
	fi
	@$(MAKE) docker-build TAG=$(TAG)

docker-clean: ## Remove local Docker image
	@echo "Removing Docker image: $(DOCKER_IMAGE)"
	@if docker image inspect $(DOCKER_IMAGE) >/dev/null 2>&1; then \
		docker rmi $(DOCKER_IMAGE) && echo "✓ Removed $(DOCKER_IMAGE)"; \
	else \
		echo "Image $(DOCKER_IMAGE) not found or already removed"; \
	fi

docker-clean-all: ## Remove all musicdl Docker images (latest and versioned)
	@echo "Removing all musicdl Docker images..."
	@IMAGES=$$(docker images $(IMAGE_NAME) --format "{{.Repository}}:{{.Tag}}" 2>/dev/null); \
	if [ -z "$$IMAGES" ]; then \
		echo "No musicdl images found"; \
	else \
		echo "$$IMAGES" | xargs docker rmi 2>/dev/null || true; \
		echo "✓ Removed all musicdl Docker images"; \
	fi

docker-push: ## Push Docker image to registry (usage: make docker-push [TAG=v1.2.3])
	@echo "Pushing Docker image: $(DOCKER_IMAGE)"
	@if ! docker image inspect $(DOCKER_IMAGE) >/dev/null 2>&1; then \
		echo "✗ Error: Image $(DOCKER_IMAGE) not found locally"; \
		echo "  Build the image first: make docker-build TAG=$(TAG)"; \
		exit 1; \
	fi
	@docker push $(DOCKER_IMAGE) || (echo "✗ Failed to push image. Ensure you are authenticated: docker login ghcr.io" && exit 1)
	@echo "✓ Docker image pushed successfully: $(DOCKER_IMAGE)"

docker-push-latest: ## Push Docker image with :latest tag
	@$(MAKE) docker-push TAG=latest

docker-push-versioned: ## Push Docker image with versioned tag (usage: make docker-push-versioned TAG=v1.2.3)
	@if [ -z "$(TAG)" ] || [ "$(TAG)" = "latest" ]; then \
		echo "✗ Error: TAG variable required and must not be 'latest'"; \
		echo "  Usage: make docker-push-versioned TAG=v1.2.3"; \
		exit 1; \
	fi
	@if ! echo "$(TAG)" | grep -qE '^v?[0-9]+\.[0-9]+\.[0-9]+'; then \
		echo "⚠ Warning: TAG '$(TAG)' does not match semantic versioning pattern (vX.Y.Z)"; \
	fi
	@$(MAKE) docker-push TAG=$(TAG)

docker-build-push: ## Build and push Docker image (usage: make docker-build-push [TAG=v1.2.3])
	@echo "Building and pushing Docker image: $(DOCKER_IMAGE)"
	@$(MAKE) docker-build TAG=$(TAG)
	@$(MAKE) docker-push TAG=$(TAG)
	@echo "✓ Build and push completed successfully"

docker-check: ## Check Docker setup and authentication
	@echo "Checking Docker setup..."
	@command -v docker >/dev/null 2>&1 || (echo "✗ Docker is not installed or not in PATH" && exit 1)
	@echo "✓ Docker is installed: $$(docker --version)"
	@docker info >/dev/null 2>&1 || (echo "✗ Docker daemon is not running" && exit 1)
	@echo "✓ Docker daemon is running"
	@echo "Checking authentication with ghcr.io..."
	@AUTH_CONFIGURED=0; \
	CRED_HELPER=0; \
	if [ -f "$$HOME/.docker/config.json" ]; then \
		if grep -qE '"ghcr\.io"|"https://ghcr\.io"' "$$HOME/.docker/config.json" 2>/dev/null; then \
			AUTH_CONFIGURED=1; \
		fi; \
		if grep -qE '"credsStore"|"credHelpers"' "$$HOME/.docker/config.json" 2>/dev/null; then \
			CRED_HELPER=1; \
			AUTH_CONFIGURED=1; \
		fi; \
	fi; \
	if [ "$$AUTH_CONFIGURED" -eq 1 ]; then \
		echo "  Registry configured ($$([ "$$CRED_HELPER" -eq 1 ] && echo "using credential helper" || echo "credentials in config file"))"; \
		echo "  Testing authentication with lightweight operation..."; \
		docker manifest inspect $(IMAGE_NAME):latest >/dev/null 2>&1; \
		EXIT_CODE=$$?; \
		if [ $$EXIT_CODE -eq 0 ]; then \
			echo "✓ Authenticated with ghcr.io (credentials valid)"; \
		elif [ $$EXIT_CODE -eq 1 ]; then \
			echo "⚠ Registry configured but authentication failed (credentials may be expired or invalid)"; \
			echo "  Run: docker login ghcr.io"; \
		else \
			echo "⚠ Registry configured (cannot verify credentials - image may not exist or be private)"; \
			echo "  To verify: docker pull $(IMAGE_NAME):latest"; \
		fi; \
	else \
		echo "⚠ Not authenticated with ghcr.io (required for push operations)"; \
		echo "  Run: docker login ghcr.io"; \
	fi
	@echo "Checking for existing musicdl images..."
	@IMAGES=$$(docker images $(IMAGE_NAME) --format "{{.Repository}}:{{.Tag}}" 2>/dev/null | head -5); \
	if [ -n "$$IMAGES" ]; then \
		echo "✓ Found existing images:"; \
		echo "$$IMAGES" | sed 's/^/  - /'; \
	else \
		echo "  (no images found)"; \
	fi
