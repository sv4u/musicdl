# Development

## Getting Started

### Prerequisites

- Go 1.24 or later
- Git
- yt-dlp (for audio downloads)
- ffmpeg (for audio conversion)

### Setup

```bash
git clone git@github.com:sv4u/musicdl.git
cd musicdl

go mod download
make build
# or: go build -o musicdl ./control
```

### Project Structure

```
musicdl/
├── control/                  # CLI entrypoint
│   ├── main.go               # Dispatch: plan, download, version
│   ├── cli.go                # planCommand, downloadCLICommand, exit codes
│   └── main_test.go          # CLI tests (usage, plan exit 1, download exit 2)
├── download/                 # Core download logic
│   ├── downloader.go         # Download orchestrator
│   ├── config/               # Config and hash
│   │   ├── config.go         # Config models and validation
│   │   ├── loader.go         # YAML loader (spec + legacy layout)
│   │   └── hash.go           # Config hash (SHA256, 16 hex)
│   ├── cache/                # Persistent caches
│   │   └── manager.go        # Spotify/YouTube/download cache JSON + TTL
│   ├── plan/                 # Plan generation and execution
│   │   ├── models.go         # DownloadPlan, PlanItem
│   │   ├── generator.go      # Plan generation
│   │   ├── optimizer.go     # Dedup, file checks
│   │   ├── executor.go       # Plan execution
│   │   ├── spec.go          # Spec JSON ↔ plan adapter
│   │   └── path.go          # GetPlanFilePath, etc.
│   ├── spotify/              # Spotify API client
│   ├── audio/                # yt-dlp provider
│   ├── metadata/             # Mutagen embedder
│   └── logging/              # Structured logger
├── go.mod
├── go.sum
├── config.yaml               # Example config
├── Makefile                  # build, test, docker (no proto)
└── musicdl.Dockerfile        # CLI-only image
```

## Running Tests

### Via Makefile

- `make test` – Unit tests (excludes integration/e2e)
- `make test-unit` – Same as above
- `make test-coverage` – Coverage in terminal
- `make test-cov-html` – HTML coverage report
- `make test-race` – Unit tests with race detector
- `make test-integration` – Integration tests (requires yt-dlp; optional Spotify credentials)
- `make test-specific FILE=./control/main_test.go` – Single test file
- `make test-function FILE=./control/main_test.go FUNC=TestPrintUsage` – Single test function

### Direct go test

```bash
go test ./... -tags="!integration !e2e" -v
go test ./control/... -v
go test -cover ./...
```

### Integration tests

Set environment or `.env`:

```bash
SPOTIGO_CLIENT_ID=your_client_id
SPOTIGO_CLIENT_SECRET=your_client_secret
```

Then:

```bash
go test ./... -v -tags=integration
# or: make test-integration
```

## Code Style

- **Formatting:** `go fmt ./...`
- **Linting:** `staticcheck ./...` and `golangci-lint run ./...`
- **Documentation:** Go doc comments for exported symbols
- **Errors:** Explicit handling; use `%w` for wrapping

## Contributing

1. Create a feature branch from `main`
2. Make changes; add/update tests
3. Run `go fmt ./...`, `staticcheck ./...`, `golangci-lint run ./...`, `go test ./...`
4. Submit a pull request

Commit messages: follow [Conventional Commits](https://www.conventionalcommits.org/) (e.g. `feat:`, `fix:`, `docs:`).

## Dependencies

- `github.com/sv4u/spotigo` – Spotify Web API
- `gopkg.in/yaml.v3` – YAML
- `github.com/bogem/id3v2/v2` – MP3 metadata
- `github.com/joho/godotenv` – Env loading (tests)

No gorilla/mux, gRPC, or protobuf. External tools: yt-dlp, ffmpeg.
