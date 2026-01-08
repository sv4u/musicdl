# Development

## Getting Started

### Prerequisites

- Python 3.12
- pip
- Git

### Setup

```bash
# Clone repository
git clone git@github.com:sv4u/musicdl.git
cd musicdl

# Install dependencies
pip install -r requirements.txt
pip install -r test-requirements.txt
```

### Project Structure

```
musicdl/
├── core/                    # Core modules
│   ├── audio_provider.py    # Audio source provider (yt-dlp wrapper)
│   ├── cache.py             # In-memory caching
│   ├── config.py            # Configuration models
│   ├── downloader.py        # Download orchestrator
│   ├── exceptions.py        # Custom exceptions
│   ├── metadata.py          # Metadata embedding
│   ├── models.py            # Data models
│   ├── plan.py              # Plan data structures
│   ├── plan_executor.py     # Plan execution
│   ├── plan_generator.py    # Plan generation
│   ├── plan_optimizer.py    # Plan optimization
│   ├── rate_limiter.py     # Rate limiting
│   ├── spotify_client.py    # Spotify API client
│   └── utils.py             # Utility functions
├── scripts/                  # Utility scripts
│   ├── get_version.py       # Version extraction
│   └── healthcheck_server.py # Healthcheck HTTP server
├── tests/                    # Test suite
│   ├── unit/                # Unit tests
│   ├── integration/         # Integration tests
│   └── e2e/                 # End-to-end tests
├── download.py              # Main entry point
├── config.yaml              # Example configuration
└── requirements.txt         # Dependencies
```

## Running Tests

### All Tests

```bash
pytest
```

### Specific Test Categories

```bash
# Unit tests only
pytest tests/unit/

# Integration tests only
pytest tests/integration/

# End-to-end tests only
pytest tests/e2e/
```

### With Coverage

```bash
pytest --cov=core --cov-report=html --cov-report=xml
```

Coverage reports:

- HTML: `htmlcov/index.html`
- XML: `coverage.xml`

## Code Style

This project follows Python best practices:

- **Type Hints**: All functions and classes should have type annotations
- **Docstrings**: Use PEP 257 docstring conventions
- **Formatting**: Use consistent formatting (consider using `black` or `ruff`)
- **Linting**: Use `ruff` for linting

### Type Annotations

All functions should include type hints:

```python
def process_downloads(config: MusicDLConfig) -> Dict[str, Dict[str, int]]:
    """Process downloads and return statistics."""
    ...
```

### Docstrings

Follow PEP 257 conventions:

```python
def generate_plan(self) -> DownloadPlan:
    """
    Generate complete download plan from configuration.

    Returns:
        DownloadPlan with all items
    """
    ...
```

## Testing Guidelines

### Test Organization

- **Unit Tests**: Test individual functions/classes in isolation
- **Integration Tests**: Test interactions between components
- **End-to-End Tests**: Test complete workflows

### Test Naming

- Test files: `test_*.py`
- Test functions: `test_*`
- Test classes: `Test*`

### Test Structure

Follow Arrange-Act-Assert pattern:

```python
def test_download_track():
    # Arrange
    config = create_test_config()
    downloader = Downloader(config)
    
    # Act
    result = downloader.download_track(track_url)
    
    # Assert
    assert result.success
    assert result.file_path.exists()
```

### Mocking

Use `pytest-mock` for mocking:

```python
def test_spotify_client_rate_limit(mocker):
    # Mock external API calls
    mock_request = mocker.patch('requests.get')
    mock_request.return_value.status_code = 429
    
    # Test rate limit handling
    ...
```

## Contributing

### Workflow

1. Create a feature branch from `main`
2. Make your changes
3. Write/update tests
4. Ensure all tests pass
5. Submit a pull request

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add support for FLAC format
fix: resolve rate limiting issue
docs: update README with new options
test: add integration tests for plan executor
```

### Pull Request Process

1. **Title**: Clear, descriptive title
2. **Description**: Explain what and why
3. **Tests**: All tests must pass
4. **Coverage**: Maintain or improve test coverage
5. **Documentation**: Update docs if needed

## Debugging

### Logging

The project uses Python's `logging` module:

```python
import logging

logger = logging.getLogger(__name__)

logger.debug("Detailed debug information")
logger.info("General information")
logger.warning("Warning message")
logger.error("Error message")
```

### Debug Mode

Enable debug logging:

```python
import logging
logging.basicConfig(level=logging.DEBUG)
```

### Plan Inspection

Inspect plan files for debugging:

```python
from core.plan import DownloadPlan
from pathlib import Path

plan = DownloadPlan.load(Path("download_plan.json"))
print(plan.get_statistics())
```

## Common Tasks

### Adding a New Audio Provider

1. Add provider to `AudioProvider` class
2. Update `audio_providers` configuration option
3. Add tests for new provider
4. Update documentation

### Adding a New Configuration Option

1. Add field to `DownloadSettings` in `core/config.py`
2. Add default value
3. Update `config.yaml` example
4. Add validation if needed
5. Update tests
6. Update documentation

### Modifying Plan Structure

1. Update `PlanItem` dataclass in `core/plan.py`
2. Update serialization/deserialization
3. Update plan generator/optimizer/executor
4. Update tests
5. Consider migration for existing plan files

## Dependencies

### Core Dependencies

- `spotipy`: Spotify Web API client
- `yt-dlp`: YouTube downloader
- `mutagen`: Audio metadata manipulation
- `pydantic`: Configuration validation
- `PyYAML`: YAML file parsing
- `requests`: HTTP requests

### Development Dependencies

- `pytest`: Testing framework
- `pytest-cov`: Coverage reporting
- `pytest-mock`: Mocking utilities
- `ruff`: Linting and formatting

### Updating Dependencies

1. Update `requirements.txt` or `test-requirements.txt`
2. Test changes thoroughly
3. Update version pins if needed
4. Commit with `deps:` prefix

## CI/CD Integration

### Pre-commit Checks

Before pushing:

1. Run tests: `pytest`
2. Check coverage: `pytest --cov=core`
3. Run linter: `ruff check .`
4. Format code: `ruff format .`

### GitHub Actions

Workflows run automatically on:

- Pull requests
- Pushes to `main`

See [CI/CD Workflows](CI-CD.md) for details.

## Troubleshooting

### Import Errors

If you encounter import errors:

```bash
# Ensure you're in the project root
cd /path/to/musicdl

# Install in development mode
pip install -e .
```

### Test Failures

If tests fail:

1. Check Python version (requires 3.12)
2. Ensure all dependencies are installed
3. Check for environment variable requirements
4. Review test output for specific errors

### Configuration Issues

If configuration validation fails:

1. Check YAML syntax
2. Verify version is "1.2"
3. Check required fields
4. Review error messages for specific issues

## Resources

- [Python Type Hints](https://docs.python.org/3/library/typing.html)
- [PEP 257 - Docstring Conventions](https://peps.python.org/pep-0257/)
- [pytest Documentation](https://docs.pytest.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)
