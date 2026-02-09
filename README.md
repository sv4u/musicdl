# musicdl

![Tests & Coverage](https://github.com/sv4u/musicdl/actions/workflows/test-and-coverage.yml/badge.svg)
![Docker Build & Test](https://github.com/sv4u/musicdl/actions/workflows/docker-build-and-test.yml/badge.svg)
![Security & SBOM](https://github.com/sv4u/musicdl/actions/workflows/security-and-sbom.yml/badge.svg)
![Version](https://img.shields.io/github/v/tag/sv4u/musicdl?label=version&sort=semver)
![License](https://img.shields.io/github/license/sv4u/musicdl)
![Go](https://img.shields.io/badge/go-1.24-blue?logo=go&logoColor=white)

A CLI tool for downloading music from Spotify by sourcing audio from YouTube, YouTube Music, and SoundCloud, then embedding metadata into downloaded files.

## Features

- **CLI Tool** – Commands: `musicdl plan`, `musicdl download`, `musicdl api`
- **Web Interface** – Modern Vue 3 + TypeScript UI (port 80 in Docker)
- **API Server** – RESTful Go HTTP server for web and CLI coordination (port 5000)
- **Plan-based workflow** – Generate a download plan (with config hash), then run downloads from that plan
- **Config hash** – Plan file is named by config content hash (`.cache/download_plan_<hash>.json`); download rejects plan if config changed
- **Multiple formats** – MP3, FLAC, M4A, and Opus output
- **Flexible config** – Supports both spec layout (top-level `spotify`, `threads`, `rate_limits`) and legacy layout under `download`
- **Spotify and YouTube playlists** – Playlists can be Spotify playlist URLs or YouTube playlist URLs; each track is planned and downloaded (YouTube playlists require `yt-dlp` on PATH for plan generation)
- **Real-time monitoring** – Progress tracking, log viewer, and rate limit alerts

## Installation

### From Source

```bash
git clone git@github.com:sv4u/musicdl.git
cd musicdl
go build -o musicdl ./control
```

### Docker

```bash
docker pull ghcr.io/sv4u/musicdl:latest
```

## Quick Start

1. **Get Spotify API credentials** from [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)

2. **Create a configuration file** (e.g. `config.yaml`):

```yaml
version: "1.2"

download:
  client_id: "your_spotify_client_id"
  client_secret: "your_spotify_client_secret"
  format: "mp3"
  bitrate: "320k"
  output: "{artist}/{album}/{title}.{output-ext}"

songs:
  - name: "Example Song"
    url: "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
```

1. **Generate the download plan**:

```bash
musicdl plan config.yaml
```

1. **Run downloads** (uses the plan saved under `.cache/`):

```bash
musicdl download config.yaml
```

## Commands

### `musicdl plan [--no-tui] <config-file>`

Generate a download plan from the config and save it to `.cache/download_plan_<hash>.json`. The hash is derived from the config file content. When stdout is a terminal, a TUI shows progress and recent errors; logs are written to `.logs/run_<timestamp>/plan.log`. Use `--no-tui` or run in a non-TTY (e.g. CI) for file-only logging and a brief summary to stdout.

```bash
musicdl plan config.yaml
musicdl plan --no-tui config.yaml
```

**Exit codes:** 0 = success, 1 = configuration error, 2 = network error, 3 = file system error

### `musicdl download [--no-tui] <config-file>`

Load the plan for the given config (by hash) and run the download. You must run `musicdl plan` first. When stdout is a terminal, a TUI shows download progress and recent errors; logs are written to `.logs/run_<timestamp>/download.log`. Use `--no-tui` or run in a non-TTY for file-only logging and a brief summary to stdout.

```bash
musicdl download config.yaml
musicdl download --no-tui config.yaml
```

**Exit codes:** 0 = success, 1 = configuration error, 2 = plan file not found or hash mismatch, 3 = network error, 4 = file system error, 5 = partial success (some failures)

### `musicdl api [--port PORT]`

Start the HTTP API server for web interface and programmatic access. Default port is 5000 (configurable via `--port` or `MUSICDL_API_PORT` environment variable).

```bash
musicdl api
musicdl api --port 8080
MUSICDL_API_PORT=9000 musicdl api
```

The API server provides endpoints for:
- Configuration management (`GET /api/config`, `POST /api/config`)
- Download plan generation (`POST /api/download/plan`)
- Download execution (`POST /api/download/run`)
- Status monitoring (`GET /api/download/status`, `GET /api/rate-limit-status`)
- Log retrieval (`GET /api/logs`)

### `musicdl version`

Show version information.

```bash
musicdl version
# or
musicdl --version
```

## Environment Variables

- **MUSICDL_CACHE_DIR** – Cache directory (default: `.cache` under current directory). Plan and caches live here.
- **MUSICDL_LOG_DIR** – Log directory (default: `.logs`). Each run creates `.logs/run_<timestamp>/` with `plan.log` or `download.log`.
- **MUSICDL_NO_TUI** – If set, disables the TUI even when stdout is a terminal (same effect as `--no-tui`).
- **MUSICDL_WORK_DIR** – Working directory for relative paths (default: current directory).
- **MUSICDL_LOG_LEVEL** – Log level for diagnostics (optional).

## Configuration

Configuration is a YAML file (version 1.2). Two layouts are supported.

### Spec layout (top-level)

```yaml
version: "1.2"
threads: 4

spotify:
  client_id: "your_client_id"
  client_secret: "your_client_secret"

rate_limits:
  spotify_retries: 3
  youtube_retries: 3
  youtube_bandwidth: 1048576

download:
  format: "mp3"
  bitrate: "320k"
  output: "{artist}/{album}/{title}.{output-ext}"
  audio_providers:
    - "youtube-music"
    - "youtube"
  overwrite: "skip"

songs:
  - name: "Song Name"
    url: "https://open.spotify.com/track/..."
```

### Legacy layout (all under `download`)

```yaml
version: "1.2"

download:
  client_id: "your_spotify_client_id"
  client_secret: "your_spotify_client_secret"
  threads: 4
  format: "mp3"
  bitrate: "320k"
  output: "{artist}/{album}/{title}.{output-ext}"
  audio_providers:
    - "youtube-music"
    - "youtube"
  overwrite: "skip"

songs:
  - name: "Song Name"
    url: "https://open.spotify.com/track/..."
```

If both are present, legacy fields take precedence. The `output` field must contain `{title}`. `threads` must be between 1 and 16.

### Output template placeholders

- `{artist}` – Artist name  
- `{album}` – Album name  
- `{title}` – Track title  
- `{track-number}` – Track number (zero-padded)  
- `{disc-number}` – Disc number (zero-padded; 00 when unknown or single disc)  
- `{output-ext}` – File extension (e.g. mp3, flac)

### Music sources

Songs, artists, playlists, and albums are configured as lists. Extended format (name + url) is recommended; simple format (key: url) is also supported.

- **Songs** – Spotify track URLs or YouTube video URLs.
- **Artists** – Spotify artist URLs only (discography is expanded).
- **Playlists** – Spotify playlist URLs or **YouTube playlist URLs**. For YouTube playlists, `musicdl plan` uses `yt-dlp` to resolve the playlist and its tracks; ensure `yt-dlp` is on your PATH.
- **Albums** – Spotify album URLs only.

```yaml
songs:
  - name: "Song Name"
    url: "https://open.spotify.com/track/..."

artists:
  - name: "Artist Name"
    url: "https://open.spotify.com/artist/..."

playlists:
  - name: "Spotify Playlist"
    url: "https://open.spotify.com/playlist/..."
    create_m3u: true
  - name: "YouTube Playlist"
    url: "https://www.youtube.com/playlist?list=PL..."
    create_m3u: true

albums:
  - name: "Album Name"
    url: "https://open.spotify.com/album/..."
    create_m3u: true
```

## Files & Directories

- **Config file** – Your YAML config (e.g. `config.yaml`).
- **.cache/** (or `MUSICDL_CACHE_DIR`) – Plan and caches:
  - `download_plan_<16-hex>.json` – Generated plan (name = config hash).
  - `spotify_cache.json`, `youtube_cache.json`, `download_cache.json` – Optional caches.
  - `temp/` – Temporary files during download (cleaned after run).

## Docker

### Web Interface (Recommended)

```bash
docker run --rm -p 80:3000 \
  -v /path/to/workspace:/download \
  -v /path/to/config.yaml:/download/config.yaml:ro \
  ghcr.io/sv4u/musicdl:latest
```

Access the web interface at `http://localhost`

Features:
- Modern Vue 3 UI with real-time progress tracking
- Built-in configuration editor
- Log viewer with live updates
- Rate limit alerts with countdown timers

### CLI Commands (Traditional)

```bash
docker run --rm -v /path/to/workspace:/download \
  -v /path/to/config.yaml:/download/config.yaml:ro \
  ghcr.io/sv4u/musicdl:latest musicdl plan config.yaml

docker run --rm -v /path/to/workspace:/download \
  -v /path/to/config.yaml:/download/config.yaml:ro \
  ghcr.io/sv4u/musicdl:latest musicdl download config.yaml
```

### Docker Compose

For development with both API and web server:

```bash
docker-compose -f docker-compose.web.yml up
```

Access at `http://localhost:3000` (backend) and `http://localhost:5000` (API)

Working directory in the image is `/download`. Set `MUSICDL_CACHE_DIR` if you want cache elsewhere (e.g. `MUSICDL_CACHE_DIR=/download/.cache`).

## Troubleshooting

- **"Plan file not found. Run 'musicdl plan' first."**  
  Run `musicdl plan <config-file>` before `musicdl download <config-file>`. The plan is stored under `.cache/` using a hash of your config.

- **"Plan file does not match configuration. Regenerate plan."**  
  The config file was changed after the plan was generated. Run `musicdl plan <config-file>` again, then `musicdl download <config-file>`.

- **Configuration error (exit 1)**  
  Check YAML syntax, required fields (`version`, `download.client_id`/`spotify.client_id`, `download.client_secret`/`spotify.client_secret`), and that `download.output` contains `{title}` and `threads` is 1–16.

- **YouTube playlist has no tracks in plan**  
  Plan generation for YouTube playlists uses `yt-dlp` with `--flat-playlist --dump-json`. Ensure `yt-dlp` is installed and on your PATH when running `musicdl plan`. If the playlist appears in the plan with zero tracks, check that the playlist URL is correct and that `yt-dlp` can access it (e.g. `yt-dlp --flat-playlist --dump-json "<playlist-url>"`).

## Development

### Quick Start

```bash
./dev.sh
```

This starts:
- Go API server on `http://localhost:5000`
- Express backend on `http://localhost:3000`
- Vue frontend dev server on `http://localhost:5173`

### Manual Setup

**Backend:**
```bash
cd webserver/backend
npm install
PORT=3000 GO_API_HOST=localhost GO_API_PORT=5000 npm run dev
```

**Frontend:**
```bash
cd webserver/frontend
npm install
npm run dev
```

**Go API Server:**
```bash
go build -o musicdl ./control
./musicdl api
```

### Building for Production

```bash
# Build everything
cd webserver/backend && npm run build && cd ..
cd frontend && npm run build && cd ..

# Build Docker image
docker build -f musicdl.Dockerfile -t ghcr.io/sv4u/musicdl:latest .
```

### Project Structure

```
musicdl/
├── control/                 # Go CLI and API server
│   ├── api.go              # HTTP API server implementation
│   ├── cli.go              # CLI commands
│   └── main.go             # Entry point
├── download/               # Core music download logic
├── webserver/              # Web UI (Node.js + Vue)
│   ├── backend/            # Express.js + TypeScript
│   ├── frontend/           # Vue 3 + TypeScript + Vite
│   └── README.md           # Web server documentation
├── musicdl.Dockerfile      # Multi-stage Docker build
├── docker-compose.web.yml  # Development compose file
├── dev.sh                  # Development startup script
└── README.md              # This file
```

## See Also

For architecture, CI/CD, and development details, see the [GitHub Wiki](https://github.com/sv4u/musicdl/wiki).

## License

See LICENSE file.
