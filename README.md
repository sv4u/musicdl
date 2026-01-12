# musicdl

![Tests & Coverage](https://github.com/sv4u/musicdl/actions/workflows/test-and-coverage.yml/badge.svg)
![Docker Build & Test](https://github.com/sv4u/musicdl/actions/workflows/docker-build-and-test.yml/badge.svg)
![Security & SBOM](https://github.com/sv4u/musicdl/actions/workflows/security-and-sbom.yml/badge.svg)
![Version](https://img.shields.io/github/v/tag/sv4u/musicdl?label=version&sort=semver)
![License](https://img.shields.io/github/license/sv4u/musicdl)
![Go](https://img.shields.io/badge/go-1.23-blue?logo=go&logoColor=white)

## NAME

musicdl - Control platform for downloading music from Spotify using YouTube and other audio sources

## SYNOPSIS

```bash
# Control platform (web UI and API)
musicdl serve [flags]

# Download service (long-running)
musicdl download [flags]
```

## DESCRIPTION

musicdl is a Go-based control platform for downloading music from Spotify by sourcing audio from YouTube, YouTube Music, and SoundCloud, then embedding metadata into downloaded files. It provides a web-based UI and REST API for managing downloads, configuration, and monitoring.

**Key Features:**
- Web-based control platform with real-time status dashboard
- REST API for programmatic control
- Plan-based architecture with parallel execution
- Config-only credentials (no environment variables)
- Real-time progress tracking and log viewing
- Configuration editor with validation

## INSTALLATION

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

## CONFIGURATION

Configuration file is YAML format, version 1.2. See `config.yaml` for example.

### Basic Structure

```yaml
version: "1.2"

download:
  threads: 4
  max_retries: 3
  format: "mp3"
  bitrate: "128k"
  output: "{artist}/{album}/{track-number} - {title}.{output-ext}"
  audio_providers: ["youtube-music", "youtube"]
  overwrite: "skip"

songs: []
artists: []
playlists: []
albums: []
```

### Download Settings

**Basic Options:**

- `threads` (int, default: 4): Parallel downloads
- `max_retries` (int, default: 3): Retry attempts for failed downloads
- `format` (str, default: "mp3"): Audio format - mp3, flac, m4a, opus
- `bitrate` (str, default: "128k"): Audio bitrate (e.g., "128k", "320k")
- `output` (str): File naming pattern with placeholders (see Output Template)
- `audio_providers` (list, default: ["youtube-music"]): Audio sources in order - youtube-music, youtube, soundcloud
- `overwrite` (str, default: "skip"): Behavior when file exists - "skip", "overwrite", or "metadata"

**Cache Settings:**

- `cache_max_size` (int, default: 1000): Maximum cached Spotify API responses
- `cache_ttl` (int, default: 3600): Cache expiration in seconds
- `audio_search_cache_max_size` (int, default: 500): Maximum cached audio search results
- `audio_search_cache_ttl` (int, default: 86400): Audio search cache TTL in seconds
- `file_existence_cache_max_size` (int, default: 10000): Maximum cached file existence checks
- `file_existence_cache_ttl` (int, default: 3600): File existence cache TTL in seconds

**Spotify API Rate Limiting:**

- `spotify_rate_limit_enabled` (bool, default: true): Enable proactive rate limiting
- `spotify_rate_limit_requests` (int, default: 10): Maximum requests per time window
- `spotify_rate_limit_window` (float, default: 1.0): Time window size in seconds
- `spotify_max_retries` (int, default: 3): Maximum retry attempts for rate-limited requests
- `spotify_retry_base_delay` (float, default: 1.0): Base delay in seconds for exponential backoff
- `spotify_retry_max_delay` (float, default: 120.0): Maximum delay in seconds for exponential backoff

**Download Rate Limiting:**

- `download_rate_limit_enabled` (bool, default: true): Enable download rate limiting
- `download_rate_limit_requests` (int, default: 2): Maximum requests per time window
- `download_rate_limit_window` (float, default: 1.0): Time window size in seconds
- `download_bandwidth_limit` (int, default: 1048576): Bandwidth limit in bytes per second (1MB/sec), null for unlimited

**Plan Architecture:**

- `plan_generation_enabled` (bool, default: true): Enable plan generation
- `plan_optimization_enabled` (bool, default: true): Enable plan optimization
- `plan_execution_enabled` (bool, default: true): Enable plan execution
- `plan_persistence_enabled` (bool, default: true): Enable plan persistence (save/load to disk)
- `plan_status_reporting_enabled` (bool, default: true): Enable plan status reporting (saves plans during generation/optimization for status display)

### Output Template Placeholders

- `{artist}` - Artist name
- `{title}` - Track title
- `{album}` - Album name
- `{track-number}` - Track number (zero-padded)
- `{disc-number}` - Disc number
- `{album-artist}` - Album artist name
- `{year}` - Release year
- `{date}` - Release date
- `{output-ext}` - File extension based on format

### Music Sources

**Songs, Artists, Playlists:**

```yaml
songs:
  - "Song Name": https://open.spotify.com/track/...
  - name: "Song Name"
    url: https://open.spotify.com/track/...

artists:
  - "Artist Name": https://open.spotify.com/artist/...

playlists:
  - "Playlist Name": https://open.spotify.com/playlist/...
```

**Albums (Simple Format):**

```yaml
albums:
  - "Album Name": https://open.spotify.com/album/...
```

**Albums (Extended Format with M3U):**

```yaml
albums:
  - name: "Album Name"
    url: https://open.spotify.com/album/...
    create_m3u: true  # Optional, defaults to false
```

## USAGE

### Control Platform (Web UI)

Start the control platform web server:

```bash
musicdl serve --port 8080 --config config.yaml
```

Access the web UI at `http://localhost:8080`:
- **Dashboard** (`/`) - Overview with status and quick actions
- **Status** (`/status`) - Detailed download status and plan progress
- **Config** (`/config`) - YAML configuration editor
- **Logs** (`/logs`) - Log viewer with filtering and search

### Download Service

Start the download service (long-running):

```bash
musicdl download --config config.yaml
```

### API Endpoints

The control platform provides REST API endpoints:

- `GET /api/health` - Health check
- `GET /api/status` - Download service status
- `POST /api/download/start` - Start download
- `POST /api/download/stop` - Stop download
- `GET /api/config` - Get configuration
- `PUT /api/config` - Update configuration
- `POST /api/config/validate` - Validate configuration
- `GET /api/logs` - Get logs (with filtering)

## EXAMPLES

**Minimal Configuration:**

```yaml
version: "1.2"

download:
  format: "mp3"
  bitrate: "320k"

songs:
  - "Example Song": https://open.spotify.com/track/...
```

**Full Configuration:**

```yaml
version: "1.2"

download:
  threads: 8
  max_retries: 5
  format: "flac"
  bitrate: "320k"
  output: "{artist}/{album}/{disc-number}{track-number} - {title}.{output-ext}"
  audio_providers: ["youtube-music", "youtube", "soundcloud"]
  overwrite: "metadata"
  cache_max_size: 2000
  cache_ttl: 7200
  spotify_rate_limit_requests: 8
  download_bandwidth_limit: 2097152  # 2MB/sec

songs:
  - "Song 1": https://open.spotify.com/track/...

artists:
  - "Artist 1": https://open.spotify.com/artist/...

playlists:
  - "Playlist 1": https://open.spotify.com/playlist/...

albums:
  - name: "Album 1"
    url: https://open.spotify.com/album/...
    create_m3u: true
```

## FILES

- `config.yaml` - Configuration file (required)
- `musicdl` - Go binary (control platform and download service)
- `/var/lib/musicdl/plans/` - Plan files directory (default, configurable via `--plan-path` or `MUSICDL_PLAN_PATH`)
  - `download_plan.json` - Initial plan after generation
  - `download_plan_optimized.json` - Optimized plan after optimization
  - `download_plan_progress.json` - Progress snapshot during execution
- `/var/lib/musicdl/logs/musicdl.log` - Application log file (default, configurable via `--log-path` or `MUSICDL_LOG_PATH`)
  - Used by `/logs` endpoint for log viewing
  - Contains all application logs with timestamps, log levels, and messages

## CONFIGURATION

**Credentials:**

Spotify API credentials are configured **only** in the configuration file. No environment variables are needed:

```yaml
download:
  client_id: "your_spotify_client_id"
  client_secret: "your_spotify_client_secret"
```

**Environment Variables (Optional):**

- `CONFIG_PATH` - Path to configuration file (default: `/scripts/config.yaml`)
- `MUSICDL_PLAN_PATH` - Plan file directory path (default: `/var/lib/musicdl/plans`)
- `MUSICDL_LOG_PATH` - Log file path (default: `/var/lib/musicdl/logs/musicdl.log`)
- `HEALTHCHECK_PORT` - Control platform port (default: `8080`)

## DOCKER

**Build:**

```bash
docker build -f musicdl.Dockerfile -t musicdl:latest .
```

**Run:**

```bash
docker run --rm \
  -p 8080:8080 \
  -v /path/to/music:/download \
  -v /path/to/config.yaml:/scripts/config.yaml:ro \
  musicdl:latest
```

**Note:** Spotify credentials are configured in `config.yaml` only. No environment variables needed.

**Custom Config:**

```bash
docker run --rm \
  -p 8080:8080 \
  -v /path/to/music:/download \
  -v /path/to/config.yaml:/scripts/config.yaml:ro \
  musicdl:latest
```

**Web UI:**

The Docker image runs the control platform by default. Access the web UI at:

- `http://localhost:8080/` - Dashboard with status and quick actions
- `http://localhost:8080/status` - Detailed status page
- `http://localhost:8080/config` - Configuration editor
- `http://localhost:8080/logs` - Log viewer

**API Endpoints:**

- `http://localhost:8080/api/health` - JSON healthcheck endpoint
- `http://localhost:8080/api/status` - Download service status
- `http://localhost:8080/api/download/start` - Start download
- `http://localhost:8080/api/download/stop` - Stop download
- `http://localhost:8080/api/config` - Get/update configuration

**Docker Compose:**

```yaml
services:
  musicdl:
    image: ghcr.io/sv4u/musicdl:latest
    ports:
      - "8080:8080"
    volumes:
      - /path/to/music/library:/download:rw
      - /path/to/config.yaml:/scripts/config.yaml:ro
    environment:
      - CONFIG_PATH=/scripts/config.yaml
      - HEALTHCHECK_PORT=8080
    restart: unless-stopped
```

## SEE ALSO

For architecture details, CI/CD workflows, TrueNAS deployment, and development documentation, see the [GitHub Wiki](https://github.com/sv4u/musicdl/wiki).

## LICENSE

See LICENSE file.
