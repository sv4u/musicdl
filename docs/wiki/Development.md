# Development

## Getting Started

### Prerequisites

- Go 1.24 or later
- Git
- yt-dlp (for audio downloads)
- ffmpeg (for audio conversion)

### Setup

```bash
# Clone repository
git clone git@github.com:sv4u/musicdl.git
cd musicdl

# Download dependencies
go mod download

# Build binary
go build -o musicdl ./control
```

### Project Structure

```
musicdl/
├── control/                  # Control platform
│   ├── main.go              # Main entry point (serve/download commands)
│   ├── server.go            # HTTP server setup
│   └── handlers/            # HTTP handlers
│       ├── handlers.go      # Base handlers struct
│       ├── dashboard.go     # Dashboard page
│       ├── status.go         # Status endpoints
│       ├── download.go       # Download control endpoints
│       ├── config.go         # Config management endpoints
│       ├── logs.go           # Log viewing endpoints
│       └── health.go         # Health check endpoints
├── download/                 # Download service
│   ├── service.go           # Download service implementation
│   ├── downloader.go        # Download orchestrator
│   ├── config/              # Configuration loading
│   │   ├── config.go        # Config models
│   │   └── loader.go        # YAML loader
│   ├── spotify/             # Spotify API client
│   │   ├── client.go        # Spotify client wrapper
│   │   ├── cache.go         # Response caching
│   │   ├── rate_limiter.go  # Rate limiting
│   │   └── rate_limit_tracker.go # Rate limit tracking
│   ├── audio/               # Audio provider
│   │   ├── provider.go      # Audio provider interface
│   │   └── ytdlp.go         # yt-dlp subprocess wrapper
│   ├── metadata/            # Metadata embedding
│   │   ├── embedder.go      # Metadata embedder interface
│   │   ├── mp3.go           # MP3 metadata
│   │   ├── flac.go          # FLAC metadata (stub)
│   │   ├── m4a.go           # M4A metadata (stub)
│   │   └── vorbis.go        # OGG/Opus metadata (stub)
│   └── plan/                # Plan architecture
│       ├── models.go        # Plan data structures
│       ├── generator.go     # Plan generation
│       ├── optimizer.go     # Plan optimization
│       └── executor.go      # Plan execution
├── go.mod                    # Go module definition
├── go.sum                    # Dependency checksums
├── config.yaml              # Example configuration
└── musicdl.Dockerfile        # Docker build file
```

## Running Tests

### All Tests

```bash
go test ./...
```

### Verbose Output

```bash
go test -v ./...
```

### Specific Package

```bash
go test ./download/spotify
```

### With Coverage

```bash
go test -cover ./...
```

### Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Tests

Integration tests require real API credentials. Set up a `.env` file:

```bash
SPOTIGO_CLIENT_ID=your_client_id
SPOTIGO_CLIENT_SECRET=your_client_secret
```

Then run integration tests:

```bash
go test -v ./download/spotify -tags=integration
```

## Code Style

This project follows Go best practices:

- **Formatting**: Use `gofmt` or `go fmt` (automatically applied)
- **Linting**: Use `golangci-lint` for comprehensive linting
- **Documentation**: Use Go doc comments for exported functions/types
- **Error Handling**: Always handle errors explicitly, use error wrapping

### Type Annotations

Go has static typing, so type annotations are required:

```go
func processDownloads(config *config.MusicDLConfig) (map[string]int, error) {
    // ...
}
```

### Documentation

Follow Go doc conventions:

```go
// GeneratePlan generates a complete download plan from configuration.
//
// It processes all songs, artists, playlists, and albums defined in the
// configuration and creates a hierarchical DownloadPlan structure.
//
// Returns the generated plan or an error if generation fails.
func (g *Generator) GeneratePlan(ctx context.Context) (*DownloadPlan, error) {
    // ...
}
```

## Testing Guidelines

### Test Organization

- **Unit Tests**: Test individual functions/structs in isolation
- **Integration Tests**: Test interactions between components (use build tags)
- **Test Files**: `*_test.go` files in the same package

### Test Naming

- Test functions: `TestFunctionName` or `TestStruct_MethodName`
- Test files: `*_test.go`
- Example tests: `ExampleFunctionName`

### Test Structure

Follow table-driven tests for multiple cases:

```go
func TestDownloader_DownloadTrack(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        wantErr bool
    }{
        {
            name:    "valid track",
            url:     "https://open.spotify.com/track/...",
            wantErr: false,
        },
        {
            name:    "invalid url",
            url:     "invalid",
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Mocking

Use interfaces for testability:

```go
type SpotifyClientInterface interface {
    GetTrack(ctx context.Context, trackID string) (*spotigo.Track, error)
}

// In tests, create mock implementations
type mockSpotifyClient struct {
    getTrackFunc func(ctx context.Context, trackID string) (*spotigo.Track, error)
}

func (m *mockSpotifyClient) GetTrack(ctx context.Context, trackID string) (*spotigo.Track, error) {
    return m.getTrackFunc(ctx, trackID)
}
```

## Contributing

### Workflow

1. Create a feature branch from `main`
2. Make your changes
3. Write/update tests
4. Ensure all tests pass: `go test ./...`
5. Format code: `go fmt ./...`
6. Submit a pull request

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
6. **Formatting**: Code must be formatted with `go fmt`

## Debugging

### Logging

The project uses Go's standard `log` package:

```go
import "log"

log.Printf("Debug: processing track %s", trackID)
log.Printf("Error: failed to download: %v", err)
```

### Debug Mode

Enable verbose logging by setting log level:

```go
log.SetFlags(log.LstdFlags | log.Lshortfile)
```

### Plan Inspection

Inspect plan files for debugging:

```go
import (
    "encoding/json"
    "os"
    "github.com/sv4u/musicdl/download/plan"
)

data, _ := os.ReadFile("download_plan.json")
var plan plan.DownloadPlan
json.Unmarshal(data, &plan)
stats := plan.GetStatistics()
fmt.Printf("Statistics: %+v\n", stats)
```

## Common Tasks

### Adding a New Audio Provider

1. Add provider to `audio.Provider` in `download/audio/provider.go`
2. Update `audio_providers` configuration option
3. Add tests for new provider
4. Update documentation

### Adding a New Configuration Option

1. Add field to `DownloadSettings` in `download/config/config.go`
2. Add default value
3. Update `config.yaml` example
4. Add validation in `Validate()` method
5. Update tests
6. Update documentation

### Modifying Plan Structure

1. Update `PlanItem` struct in `download/plan/models.go`
2. Update JSON serialization tags
3. Update plan generator/optimizer/executor
4. Update tests
5. Consider migration for existing plan files

## Dependencies

### Core Dependencies

- `github.com/sv4u/spotigo`: Spotify Web API client
- `github.com/gorilla/mux`: HTTP router
- `gopkg.in/yaml.v3`: YAML parsing
- `github.com/bogem/id3v2/v2`: MP3 metadata
- `github.com/mewkiz/flac`: FLAC metadata (for future use)
- `github.com/joho/godotenv`: Environment variable loading (for tests)

### External Tools

- `yt-dlp`: YouTube downloader (Python tool, called as subprocess)
- `ffmpeg`: Audio conversion

### Updating Dependencies

1. Update `go.mod` with new version
2. Run `go mod tidy` to update `go.sum`
3. Test changes thoroughly
4. Commit with `deps:` prefix

## CI/CD Integration

### Pre-commit Checks

Before pushing:

1. Run tests: `go test ./...`
2. Check formatting: `go fmt ./...`
3. Run linter: `golangci-lint run`
4. Check for race conditions: `go test -race ./...`

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

# Download dependencies
go mod download

# Verify module
go mod verify
```

### Test Failures

If tests fail:

1. Check Go version (requires 1.24+)
2. Ensure all dependencies are downloaded: `go mod download`
3. Check for environment variable requirements (for integration tests)
4. Review test output for specific errors

### Configuration Issues

If configuration validation fails:

1. Check YAML syntax
2. Verify version is "1.2"
3. Check required fields
4. Review error messages for specific issues

### Build Issues

If build fails:

```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
go mod download

# Verify build
go build ./control
```

## Resources

- [Go Documentation](https://go.dev/doc/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Testing](https://go.dev/doc/tutorial/add-a-test)
- [Conventional Commits](https://www.conventionalcommits.org/)
