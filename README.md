# musicdl

![Tests & Coverage](https://github.com/sv4u/musicdl/actions/workflows/test-and-coverage.yml/badge.svg)
![Docker Build & Test](https://github.com/sv4u/musicdl/actions/workflows/docker-build-and-test.yml/badge.svg)
![Security & SBOM](https://github.com/sv4u/musicdl/actions/workflows/security-and-sbom.yml/badge.svg)
![Version](https://img.shields.io/github/v/tag/sv4u/musicdl?label=version&sort=semver)
![License](https://img.shields.io/github/license/sv4u/musicdl)
![Python](https://img.shields.io/badge/python-3.12-blue?logo=python&logoColor=white)

## NAME

musicdl - Download music from Spotify using YouTube and other audio sources

## SYNOPSIS

```bash
python3 download.py [CONFIG]
```

## DESCRIPTION

Downloads music from Spotify by sourcing audio from YouTube, YouTube Music, and SoundCloud, then embedding metadata into downloaded files. Uses plan-based architecture with parallel execution.

## INSTALLATION

```bash
git clone git@github.com:sv4u/musicdl.git
cd musicdl
pip install -r requirements.txt
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

```bash
python3 download.py config.yaml
```

Processes all songs, artists, playlists, and albums in configuration file. Displays summary of successful and failed downloads.

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
- `download.py` - Main entry point
- `/var/lib/musicdl/plans/` - Plan files directory (default, configurable via `MUSICDL_PLAN_PATH`)
  - `download_plan.json` - Initial plan after generation
  - `download_plan_optimized.json` - Optimized plan after optimization
  - `download_plan_progress.json` - Progress snapshot during execution
- `/var/lib/musicdl/logs/musicdl.log` - Application log file (default, configurable via `MUSICDL_LOG_PATH`)
  - Used by `/logs` endpoint for log viewing
  - Contains all application logs with timestamps, log levels, and messages

## ENVIRONMENT

**Required:**

- `SPOTIFY_CLIENT_ID` - Spotify API client ID
- `SPOTIFY_CLIENT_SECRET` - Spotify API client secret

**Optional:**

- `CONFIG_PATH` - Path to configuration file (default: `/scripts/config.yaml`)
- `MUSICDL_PLAN_PATH` - Plan file directory path (default: `/var/lib/musicdl/plans`)
- `MUSICDL_LOG_PATH` - Log file path (default: `/var/lib/musicdl/logs/musicdl.log`)
- `HEALTHCHECK_PORT` - Healthcheck server port (default: `8080`)

**Credential Resolution Order:**

1. Environment variables (`SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET`) - highest priority
2. Configuration file (`download.client_id`, `download.client_secret`) - fallback

## DOCKER

**Build:**

```bash
docker build -f musicdl.Dockerfile -t musicdl:latest .
```

**Run:**

```bash
docker run --rm \
  -e SPOTIFY_CLIENT_ID="your_client_id" \
  -e SPOTIFY_CLIENT_SECRET="your_client_secret" \
  -v /path/to/music:/download \
  musicdl:latest
```

**Custom Config:**

```bash
docker run --rm \
  -e SPOTIFY_CLIENT_ID="your_client_id" \
  -e SPOTIFY_CLIENT_SECRET="your_client_secret" \
  -v /path/to/music:/download \
  -v /path/to/config.yaml:/scripts/config.yaml:ro \
  musicdl:latest
```

**Healthcheck Server:**

Docker image includes HTTP healthcheck server. Publish port 8080:

- `http://localhost:8080/health` - JSON healthcheck endpoint for monitoring systems
- `http://localhost:8080/status` - HTML status dashboard with real-time progress
- `http://localhost:8080/logs` - HTML log viewer with filtering and search

Requires `plan_persistence_enabled: true` or `plan_status_reporting_enabled: true` in configuration.

**Status Dashboard Features:**

- Real-time download progress and statistics
- Plan phase tracking (generating, optimizing, executing)
- Spotify rate limit warnings with countdown timer
- Plan item status (pending, in progress, completed, failed, skipped)
- Auto-refresh capability

**Log Viewer Features:**

- View all application logs in styled console format
- Filter by log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
- Search logs by keyword (case-insensitive)
- Filter by time range (start time and end time)
- Auto-refresh capability
- Search result highlighting

**Docker Compose:**

```yaml
services:
  musicdl:
    image: musicdl:latest
    build:
      context: .
      dockerfile: ./musicdl.Dockerfile
    volumes:
      - /path/to/music/library:/download:rw
    environment:
      - SPOTIFY_CLIENT_ID=${SPOTIFY_CLIENT_ID}
      - SPOTIFY_CLIENT_SECRET=${SPOTIFY_CLIENT_SECRET}
    restart: unless-stopped
```

## SEE ALSO

For architecture details, CI/CD workflows, TrueNAS deployment, and development documentation, see the [GitHub Wiki](https://github.com/sv4u/musicdl/wiki).

## LICENSE

See LICENSE file.
