# musicdl

![Tests & Coverage](https://github.com/sv4u/musicdl/actions/workflows/test-and-coverage.yml/badge.svg)
![Docker Build & Test](https://github.com/sv4u/musicdl/actions/workflows/docker-build-and-test.yml/badge.svg)
![Security & SBOM](https://github.com/sv4u/musicdl/actions/workflows/security-and-sbom.yml/badge.svg)
![Version](https://img.shields.io/github/v/tag/sv4u/musicdl?label=version&sort=semver)
![License](https://img.shields.io/github/license/sv4u/musicdl)
![Go](https://img.shields.io/badge/go-1.24-blue?logo=go&logoColor=white)

A Go-based control platform for downloading music from Spotify by sourcing audio from YouTube, YouTube Music, and SoundCloud, then embedding metadata into downloaded files.

## Features

- **Web-based UI** - Dashboard, status monitoring, configuration editor, and log viewer
- **REST API** - Programmatic control of downloads and configuration
- **Decoupled Architecture** - Web server and download service run as separate processes with gRPC communication
- **Plan-based Downloads** - Generates, optimizes, and executes download plans with parallel processing
- **Real-time Updates** - Server-Sent Events (SSE) for live status and log streaming
- **Configuration Management** - YAML-based config with validation and in-browser editing
- **Multiple Formats** - Supports MP3, FLAC, M4A, and Opus output formats
- **Smart Caching** - Spotify API, audio search, and file existence caching

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

2. **Create a configuration file** (`config.yaml`):

```yaml
version: "1.2"

download:
  client_id: "your_spotify_client_id"
  client_secret: "your_spotify_client_secret"
  format: "mp3"
  bitrate: "320k"

songs:
  - name: "Example Song"
    url: "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
```

3. **Start the control platform**:

```bash
musicdl serve --port 8080 --config config.yaml
```

4. **Access the web UI** at `http://localhost:8080` and click "Start Download"

## Commands

### `musicdl serve`

Start the control platform web server (web UI and REST API).

```bash
musicdl serve [flags]
```

**Flags:**
- `--port` (default: `8080`) - HTTP server port
- `--config` (default: `/scripts/config.yaml`) - Path to configuration file
- `--plan-path` (default: `/var/lib/musicdl/plans`) - Path to plan files directory
- `--log-path` (default: `/var/lib/musicdl/logs/musicdl.log`) - Path to log file

**Example:**
```bash
musicdl serve --port 8080 --config ./config.yaml --plan-path ./plans --log-path ./logs/musicdl.log
```

### `musicdl download`

Run the download service directly (one-shot or daemon mode). Normally, the web server manages this automatically.

```bash
musicdl download [flags]
```

**Flags:**
- `--config` (default: `/scripts/config.yaml`) - Path to configuration file
- `--plan-path` (default: `/var/lib/musicdl/plans`) - Path to plan files directory
- `--log-path` (default: `/var/lib/musicdl/logs/musicdl.log`) - Path to log file
- `--daemon` - Run as long-running daemon (default: one-shot mode)

**Examples:**
```bash
# One-shot mode (runs until completion)
musicdl download --config config.yaml

# Daemon mode (runs until interrupted)
musicdl download --config config.yaml --daemon
```

### `musicdl download-service`

Start the download service gRPC server (internal use, spawned by web server).

```bash
musicdl download-service [flags]
```

**Flags:**
- `--port` (default: `30025`) - gRPC server port
- `--plan-path` (default: `/var/lib/musicdl/plans`) - Path to plan files directory
- `--log-path` (default: `/var/lib/musicdl/logs/musicdl.log`) - Path to log file

### `musicdl version`

Show version information.

```bash
musicdl version
```

## Configuration

Configuration is stored in a YAML file (version 1.2). See `config.yaml` for a complete example.

### Required Settings

```yaml
version: "1.2"

download:
  client_id: "your_spotify_client_id"      # Required
  client_secret: "your_spotify_client_secret"  # Required
```

### Basic Download Settings

```yaml
download:
  threads: 4              # Parallel download threads (default: 4)
  max_retries: 3          # Retry attempts for failed downloads (default: 3)
  format: "mp3"          # Output format: mp3, flac, m4a, opus (default: mp3)
  bitrate: "320k"         # Audio bitrate: 128k, 192k, 256k, 320k (default: 128k)
  output: "{artist}/{album}/{track-number} - {title}.{output-ext}"  # File naming pattern
  audio_providers:        # Audio sources in order of preference
    - "youtube-music"     # Options: youtube-music, youtube, soundcloud
    - "youtube"
  overwrite: "skip"       # Behavior when file exists: skip, overwrite, metadata (default: skip)
```

### Output Template Placeholders

The `output` field supports these placeholders:

- `{artist}` - Artist name
- `{album}` - Album name
- `{title}` - Track title
- `{track-number}` - Track number (zero-padded, e.g., "01", "02")
- `{output-ext}` - File extension based on format (e.g., "mp3", "flac")

**Example:**
```yaml
output: "{artist}/{album}/{track-number} - {title}.{output-ext}"
# Results in: "Artist Name/Album Name/01 - Song Title.mp3"
```

### Music Sources

All source types support two formats: **simple** (dict) and **extended** (list with name/url).

#### Songs

```yaml
# Extended format (recommended)
songs:
  - name: "Song Name"
    url: "https://open.spotify.com/track/..."

# Simple format (legacy)
songs:
  - "Song Name": "https://open.spotify.com/track/..."
```

#### Artists

```yaml
# Extended format (recommended)
artists:
  - name: "Artist Name"
    url: "https://open.spotify.com/artist/..."

# Simple format (legacy)
artists:
  - "Artist Name": "https://open.spotify.com/artist/..."
```

#### Playlists

```yaml
# Extended format with M3U support
playlists:
  - name: "Playlist Name"
    url: "https://open.spotify.com/playlist/..."
    create_m3u: true  # Optional: generate .m3u playlist file

# Simple format (legacy)
playlists:
  - "Playlist Name": "https://open.spotify.com/playlist/..."
```

#### Albums

```yaml
# Extended format with M3U support
albums:
  - name: "Album Name"
    url: "https://open.spotify.com/album/..."
    create_m3u: true  # Optional: generate .m3u playlist file

# Simple format (legacy)
albums:
  - "Album Name": "https://open.spotify.com/album/..."
```

### Advanced Settings

#### Cache Configuration

```yaml
download:
  cache_max_size: 1000                    # Spotify API cache size (default: 1000)
  cache_ttl: 3600                         # Cache TTL in seconds (default: 3600 = 1 hour)
  audio_search_cache_max_size: 500       # Audio search cache size (default: 500)
  audio_search_cache_ttl: 86400          # Audio search cache TTL (default: 86400 = 24 hours)
  file_existence_cache_max_size: 10000   # File existence cache size (default: 10000)
  file_existence_cache_ttl: 3600         # File existence cache TTL (default: 3600 = 1 hour)
```

#### Spotify API Rate Limiting

```yaml
download:
  spotify_rate_limit_enabled: true        # Enable rate limiting (default: true)
  spotify_rate_limit_requests: 10         # Max requests per window (default: 10)
  spotify_rate_limit_window: 1.0         # Time window in seconds (default: 1.0)
  spotify_max_retries: 3                 # Max retry attempts (default: 3)
  spotify_retry_base_delay: 1.0          # Base delay for exponential backoff (default: 1.0)
  spotify_retry_max_delay: 120.0        # Max delay in seconds (default: 120.0)
```

#### Download Rate Limiting

```yaml
download:
  download_rate_limit_enabled: true       # Enable download rate limiting (default: true)
  download_rate_limit_requests: 2        # Max concurrent downloads (default: 2)
  download_rate_limit_window: 1.0        # Time window in seconds (default: 1.0)
  download_bandwidth_limit: 1048576      # Bandwidth limit in bytes/sec (1MB/sec, default: 1048576)
```

#### UI Configuration

```yaml
ui:
  history_path: ""                        # History directory (default: plan_path/history)
  history_retention: 0                    # Number of runs to keep (0 = unlimited, default: 0)
  snapshot_interval: 10                   # Progress snapshot interval in seconds (default: 10)
  log_path: ""                           # Log file path (default: uses --log-path flag)
```

## Web UI

Access the web UI at `http://localhost:8080` (or your configured port):

- **Dashboard** (`/`) - Overview with status and quick actions
- **Status** (`/status`) - Detailed download status and plan progress
- **Config** (`/config`) - YAML configuration editor with validation
- **Logs** (`/logs`) - Log viewer with filtering and search

## REST API

### Health & Status

- `GET /api/health` - Health check (returns JSON)
- `GET /api/health/stats` - Detailed health statistics
- `GET /api/status` - Download service status
- `GET /api/status/stream` - Server-Sent Events stream for real-time status updates

### Download Control

- `POST /api/download/start` - Start download
- `POST /api/download/stop` - Stop download
- `GET /api/download/status` - Get download status
- `POST /api/download/reset` - Reset download state (stops service, clears plan files)

### Configuration

- `GET /api/config` - Get configuration (returns YAML)
- `PUT /api/config` - Update configuration (queued for next download)
- `POST /api/config/validate` - Validate configuration without saving
- `GET /api/config/digest` - Get configuration digest and version

### Plan & Logs

- `GET /api/plan/items` - Get plan items with filtering (query params: `status`, `type`, `sort`, `order`, `search`, `hierarchy`)
- `GET /api/logs` - Get logs with filtering (query params: `level`, `search`, `start_time`, `end_time`, `max_lines`)
- `GET /api/logs/stream` - Server-Sent Events stream for real-time log streaming

## Examples

### Minimal Configuration

```yaml
version: "1.2"

download:
  client_id: "your_client_id"
  client_secret: "your_client_secret"
  format: "mp3"
  bitrate: "320k"

songs:
  - name: "Example Song"
    url: "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
```

### Complete Configuration

```yaml
version: "1.2"

download:
  # Spotify API credentials (required)
  client_id: "your_spotify_client_id"
  client_secret: "your_spotify_client_secret"

  # Basic settings
  threads: 8
  max_retries: 5
  format: "flac"
  bitrate: "320k"
  output: "{artist}/{album}/{track-number} - {title}.{output-ext}"
  audio_providers:
    - "youtube-music"
    - "youtube"
    - "soundcloud"
  overwrite: "metadata"

  # Cache settings
  cache_max_size: 2000
  cache_ttl: 7200
  audio_search_cache_max_size: 1000
  audio_search_cache_ttl: 86400

  # Rate limiting
  spotify_rate_limit_requests: 8
  download_rate_limit_requests: 4
  download_bandwidth_limit: 2097152  # 2MB/sec

ui:
  history_path: "custom_history"
  history_retention: 100
  snapshot_interval: 5
  log_path: "logs/musicdl.log"

# Music sources
songs:
  - name: "Song 1"
    url: "https://open.spotify.com/track/..."

artists:
  - name: "Artist 1"
    url: "https://open.spotify.com/artist/..."

playlists:
  - name: "My Playlist"
    url: "https://open.spotify.com/playlist/..."
    create_m3u: true

albums:
  - name: "Album 1"
    url: "https://open.spotify.com/album/..."
    create_m3u: true
```

### Mixed Source Formats

```yaml
version: "1.2"

download:
  client_id: "your_client_id"
  client_secret: "your_client_secret"
  format: "mp3"

# Mix of simple and extended formats
songs:
  - name: "Song 1"
    url: "https://open.spotify.com/track/1"
  - "Song 2": "https://open.spotify.com/track/2"  # Simple format

artists:
  - name: "Artist 1"
    url: "https://open.spotify.com/artist/1"

playlists:
  - name: "Playlist 1"
    url: "https://open.spotify.com/playlist/1"
    create_m3u: true
  - "Playlist 2": "https://open.spotify.com/playlist/2"  # Simple format

albums:
  - name: "Album 1"
    url: "https://open.spotify.com/album/1"
    create_m3u: true
```

## Docker

### Basic Usage

```bash
docker run --rm \
  -p 8080:8080 \
  -v /path/to/music:/download \
  -v /path/to/config.yaml:/scripts/config.yaml:ro \
  ghcr.io/sv4u/musicdl:latest
```

### Docker Compose

```yaml
version: '2.4'

services:
  musicdl:
    image: ghcr.io/sv4u/musicdl:latest
    container_name: musicdl
    ports:
      - "8080:8080"
    volumes:
      - /path/to/music/library:/download:rw
      - ./plans:/var/lib/musicdl/plans:rw
      - ./logs:/var/lib/musicdl/logs:rw
      - /path/to/config.yaml:/scripts/config.yaml:ro
    environment:
      - CONFIG_PATH=/scripts/config.yaml
      - HEALTHCHECK_PORT=8080
      - MUSICDL_PLAN_PATH=/var/lib/musicdl/plans
      - MUSICDL_LOG_PATH=/var/lib/musicdl/logs/musicdl.log
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/scripts/healthcheck.sh"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Environment Variables

- `CONFIG_PATH` - Path to configuration file (default: `/scripts/config.yaml`)
- `MUSICDL_PLAN_PATH` - Plan files directory (default: `/var/lib/musicdl/plans`)
- `MUSICDL_LOG_PATH` - Log file path (default: `/var/lib/musicdl/logs/musicdl.log`)
- `HEALTHCHECK_PORT` - Control platform port (default: `8080`)

## Files & Directories

- `config.yaml` - Configuration file (required)
- `/var/lib/musicdl/plans/` - Plan files directory (default, configurable via `--plan-path`)
  - `download_plan.json` - Initial plan after generation
  - `download_plan_optimized.json` - Optimized plan after optimization
  - `download_plan_progress.json` - Progress snapshot during execution
- `/var/lib/musicdl/logs/musicdl.log` - Application log file (default, configurable via `--log-path`)

## Architecture

musicdl uses a decoupled architecture:

1. **Control Platform** (`musicdl serve`) - Web server providing UI and REST API
2. **Download Service** (`musicdl download-service`) - gRPC server handling downloads
3. **Communication** - gRPC between control platform and download service

The control platform automatically spawns and manages the download service as a child process when downloads are started via the web UI or API.

## See Also

For architecture details, CI/CD workflows, TrueNAS deployment, and development documentation, see the [GitHub Wiki](https://github.com/sv4u/musicdl/wiki).

## License

See LICENSE file.
