# `musicdl`

Personal music downloader with native Python implementation (no spotDL dependency).

## Overview

musicdl downloads music from Spotify by sourcing audio from YouTube and other providers, then embedding metadata into the downloaded files. This is a simplified, self-contained implementation that uses spotDL's dependencies (spotipy, yt-dlp, mutagen) directly without requiring the spotDL package.

## Features

- Downloads tracks, albums, playlists, and artist discographies from Spotify
- Sources audio from YouTube Music, YouTube, and SoundCloud
- Embeds metadata (album art, track information) using mutagen
- Supports multiple audio formats (MP3, FLAC, M4A, Opus)
- In-memory caching with LRU eviction and TTL expiration
- Configurable via single YAML file
- Structured logging and error handling

## Architecture

The application uses a simplified, modular architecture:

- **Core Modules**: Spotify client, audio provider, metadata embedder, download orchestrator
- **Caching**: Simple in-memory cache with TTL and LRU support (no external dependencies)
- **Configuration**: Pydantic models for type-safe configuration validation
- **Error Handling**: Custom exceptions with retry logic and exponential backoff

## Installation

1. Clone repository:

    ```bash
    git clone git@gitlab.com/sv4u/musicdl.git
    cd musicdl
    ```

2. Install dependencies:

    ```bash
    pip install -r requirements.txt
    ```

3. Configure `config.yaml` with your Spotify API credentials (see Configuration section)

## Configuration

The tool uses a single `config.yaml` file (version 1.2) for all settings.

### Basic Structure

```yaml
version: 1.2

download:
  # Spotify credentials
  client_id: "your_client_id"
  client_secret: "your_client_secret"
  
  # Download settings
  threads: 4
  max_retries: 3
  format: "mp3"
  bitrate: "128k"
  output: "{artist}/{album}/{track-number} - {title}.{output-ext}"
  
  # Provider settings
  audio_providers: ["youtube-music", "youtube"]
  
  # Cache settings
  cache_max_size: 1000
  cache_ttl: 3600
  
  # File management
  overwrite: "skip"  # skip, overwrite, metadata

songs: []
artists: []
playlists: []
```

### Download Settings

All download settings are configured under the `download` section:

- `client_id` / `client_secret`: Spotify API credentials (required)
- `threads`: Number of parallel downloads (default: 4)
- `max_retries`: Retry attempts for failed downloads (default: 3)
- `format`: Audio format - mp3, flac, m4a, opus (default: mp3)
- `bitrate`: Audio bitrate - e.g., "128k", "320k" (default: "128k")
- `output`: File naming pattern with placeholders (see below)
- `audio_providers`: List of audio sources to try in order (default: ["youtube-music"])
- `cache_max_size`: Maximum cached Spotify API responses (default: 1000)
- `cache_ttl`: Cache expiration time in seconds (default: 3600 = 1 hour)
- `overwrite`: Behavior when file exists - "skip", "overwrite", or "metadata" (default: "skip")

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

- `songs`: List of individual songs `{name: url}`
- `artists`: List of artists to download entire discography
- `playlists`: List of playlists (creates M3U files)

## Usage

Run the download script with your configuration file:

```bash
python3 download.py config.yaml
```

The script will:

1. Load and validate the configuration
2. Process all songs, artists, and playlists
3. Display a summary of successful and failed downloads

## Docker

1. Build image:

    ```bash
    docker build -f musicdl.Dockerfile -t musicdl:latest .
    ```

2. Start container with music library mapped to `/download`:

    ```bash
    docker run -v /path/to/music/library:/download musicdl:latest python3 download.py config.yaml
    ```

## Dependencies

- **spotipy**: Spotify Web API client
- **yt-dlp**: YouTube downloader
- **mutagen**: Audio metadata manipulation
- **pydantic**: Configuration validation
- **PyYAML**: YAML file parsing
- **requests**: HTTP requests for cover art

## Key Differences from spotDL

1. **No spotDL Package**: Direct implementation using spotDL's dependencies
2. **Simplified Architecture**: No singleton patterns, simpler provider abstraction
3. **Single Configuration**: One YAML file (version 1.2) instead of split config
4. **In-Memory Caching**: Simple cache implementation (no file persistence)
5. **Focused Features**: Only core download functionality (no web UI, sync, etc.)

## License

See LICENSE file.
