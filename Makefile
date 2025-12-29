.PHONY: help test test-unit test-integration test-e2e test-coverage test-cov-html test-cov-xml test-fast install-test-deps clean-test clean

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

clean: clean-test ## Clean all generated files including test artifacts

test-check: ## Check if test dependencies are installed
	@python3 -c "import pytest" 2>/dev/null && echo "✓ pytest installed" || echo "✗ pytest not installed - run 'make install-test-deps'"
	@python3 -c "import pytest_mock" 2>/dev/null && echo "✓ pytest-mock installed" || echo "✗ pytest-mock not installed - run 'make install-test-deps'"
	@python3 -c "import pytest_cov" 2>/dev/null && echo "✓ pytest-cov installed" || echo "✗ pytest-cov not installed - run 'make install-test-deps'"

test-lint: ## Run tests and check for linting issues (if pylint/flake8 available)
	@echo "Running tests with linting checks..."
	pytest --flake8 --pylint 2>/dev/null || pytest

