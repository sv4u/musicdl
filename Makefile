.PHONY: help test test-unit test-integration test-e2e test-coverage test-cov-html test-cov-xml test-fast install-test-deps clean-test clean docker-build docker-build-latest docker-build-versioned docker-clean docker-clean-all docker-push docker-push-latest docker-push-versioned docker-build-push docker-check

# Default Python interpreter
PYTHON := python3
PIP := pip3

# Test directories
TEST_DIR := tests
UNIT_DIR := tests/unit
INTEGRATION_DIR := tests/integration
E2E_DIR := tests/e2e

# Coverage directories
COV_DIR := htmlcov
COV_XML := coverage.xml

# Docker configuration
DOCKERFILE := musicdl.Dockerfile
IMAGE_NAME := ghcr.io/sv4u/musicdl
TAG ?= latest
DOCKER_IMAGE := $(IMAGE_NAME):$(TAG)
DOCKER_BUILD_CONTEXT := .

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

install-test-deps: ## Install test dependencies
	$(PIP) install -r test-requirements.txt

test: ## Run all tests
	pytest

test-unit: ## Run only unit tests (fast, no external dependencies)
	pytest $(UNIT_DIR) -m "not integration and not e2e"

test-integration: ## Run only integration tests (requires real APIs/credentials)
	pytest $(INTEGRATION_DIR) -m integration

test-e2e: ## Run only end-to-end tests (requires full environment)
	pytest $(E2E_DIR) -m e2e

test-fast: ## Run fast tests (unit tests only, no coverage)
	pytest $(UNIT_DIR) -m "not integration and not e2e" --no-cov

test-coverage: ## Run all tests with coverage report
	pytest --cov=core --cov-report=term-missing

test-cov-html: ## Run tests and generate HTML coverage report
	pytest --cov=core --cov-report=html
	@echo "Coverage report generated in $(COV_DIR)/index.html"

test-cov-xml: ## Run tests and generate XML coverage report (for CI)
	pytest --cov=core --cov-report=xml
	@echo "Coverage report generated in $(COV_XML)"

test-verbose: ## Run tests with verbose output
	pytest -v -s

test-specific: ## Run specific test file (usage: make test-specific FILE=tests/unit/test_cache.py)
	@if [ -z "$(FILE)" ]; then \
		echo "Error: FILE variable required. Usage: make test-specific FILE=tests/unit/test_cache.py"; \
		exit 1; \
	fi
	pytest $(FILE) -v

test-function: ## Run specific test function (usage: make test-function FILE=tests/unit/test_cache.py::TestTTLCache::test_cache_set_and_get)
	@if [ -z "$(FILE)" ]; then \
		echo "Error: FILE variable required. Usage: make test-function FILE=tests/unit/test_cache.py::TestTTLCache::test_cache_set_and_get"; \
		exit 1; \
	fi
	pytest $(FILE) -v

test-parallel: ## Run tests in parallel (faster execution)
	pytest -n auto

test-watch: ## Run tests in watch mode (requires pytest-watch: pip install pytest-watch)
	ptw

clean-test: ## Clean test artifacts (coverage reports, cache, etc.)
	rm -rf $(COV_DIR)
	rm -f $(COV_XML)
	rm -rf .pytest_cache
	rm -rf .coverage
	find . -type d -name __pycache__ -exec rm -r {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete
	find . -type f -name "*.pyo" -delete

clean: clean-test docker-clean-all ## Clean all generated files including test artifacts and Docker images

test-check: ## Check if test dependencies are installed
	@python3 -c "import pytest" 2>/dev/null && echo "✓ pytest installed" || echo "✗ pytest not installed - run 'make install-test-deps'"
	@python3 -c "import pytest_mock" 2>/dev/null && echo "✓ pytest-mock installed" || echo "✗ pytest-mock not installed - run 'make install-test-deps'"
	@python3 -c "import pytest_cov" 2>/dev/null && echo "✓ pytest-cov installed" || echo "✗ pytest-cov not installed - run 'make install-test-deps'"

test-lint: ## Run tests and check for linting issues (if pylint/flake8 available)
	@echo "Running tests with linting checks..."
	pytest --flake8 --pylint 2>/dev/null || pytest

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

