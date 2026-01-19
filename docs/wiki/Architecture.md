# Architecture

## Overview

musicdl uses a decoupled architecture with a web server (control plane) and a separate download service. The web server provides a REST API and web UI, while the download service handles the actual download operations. Communication between the two is via gRPC. The download service uses a plan-based architecture with parallel execution, processing downloads in three distinct phases: generation, optimization, and execution.

## System Architecture

### Decoupled Design

The system consists of two main components:

1. **Web Server (Control Plane)**: HTTP server providing REST API and web UI
2. **Download Service**: gRPC server handling download operations

**Key Benefits:**
- **Separation of Concerns**: Web server handles user interaction, download service handles downloads
- **Independent Lifecycle**: Download service can be started/stopped independently
- **Process Isolation**: Download service runs as a separate process, improving stability
- **Resource Management**: Each component can have different resource limits
- **Graceful Shutdown**: Coordinated shutdown ensures clean state management

### Architecture Diagram

```mermaid
flowchart TD
    User[User/Browser] --> WebServer[Web Server<br/>HTTP/REST API]
    WebServer --> ServiceManager[Service Manager<br/>Process Management]
    ServiceManager --> DownloadService[Download Service<br/>gRPC Server]
    
    WebServer --> ConfigManager[Config Manager<br/>Config & Queue]
    ConfigManager --> DownloadService
    
    DownloadService --> PlanGenerator[Plan Generator]
    DownloadService --> PlanOptimizer[Plan Optimizer]
    DownloadService --> PlanExecutor[Plan Executor]
    
    PlanGenerator --> SpotifyClient[Spotify Client]
    PlanExecutor --> AudioProvider[Audio Provider<br/>yt-dlp]
    PlanExecutor --> MetadataEmbedder[Metadata Embedder<br/>mutagen]
    
    SpotifyClient --> SpotifyAPI[Spotify Web API]
    AudioProvider --> YouTube[YouTube/YouTube Music]
    AudioProvider --> SoundCloud[SoundCloud]
    MetadataEmbedder --> FileSystem[File System]
    
    WebServer --> LogStreamer[Log Streamer<br/>SSE]
    DownloadService --> Logger[Structured Logger<br/>JSON]
    Logger --> LogFile[Log File]
    LogStreamer --> LogFile
```

### Communication Flow

1. **User Request**: User interacts with web UI or REST API
2. **Web Server**: Handles HTTP request, validates, manages state
3. **Service Manager**: Spawns/manages download service process if needed
4. **gRPC Client**: Web server uses gRPC client to communicate with download service
5. **Download Service**: Receives gRPC requests, executes downloads, streams logs
6. **Response**: Results flow back through gRPC to web server, then to user

## Plan-Based Architecture

### Benefits

- **Better Optimization**: Removes duplicates before downloading, checks existing files
- **Improved Parallelization**: Processes all tracks in parallel regardless of source (song, album, playlist)
- **Detailed Progress Tracking**: Real-time per-item progress with status updates
- **Plan Persistence**: Save and resume download plans (when enabled)
- **Better Error Recovery**: Failed items can be retried without reprocessing entire plan

### Component Details

#### Web Server (Control Plane)

**Location**: `control/` package

**Components:**
- **HTTP Server**: Gorilla Mux router with REST API endpoints
- **Handlers**: Request handlers for API and web UI
- **Service Manager**: Manages download service process lifecycle
- **Config Manager**: Manages configuration loading, validation, and update queue
- **gRPC Client**: Client for communicating with download service

**Key Features:**
- REST API for programmatic control
- Web UI (Dashboard, Status, Config, Logs pages)
- Process management (spawn, monitor, stop download service)
- Configuration management with update queuing
- Log streaming via Server-Sent Events (SSE)
- Graceful shutdown coordination

#### Download Service

**Location**: `download/` package

**Components:**
- **gRPC Server**: Protocol Buffers-based RPC server
- **Service**: Core download orchestration
- **Plan Generator**: Converts config to download plan
- **Plan Optimizer**: Removes duplicates, checks files
- **Plan Executor**: Executes downloads in parallel
- **Structured Logger**: JSON logging to file

**Key Features:**
- gRPC interface for remote control
- Plan-based download architecture
- Parallel execution with thread pool
- Real-time progress tracking
- Structured JSON logging
- Version compatibility checking

### Three-Phase Process

#### Phase 1: Plan Generation

Converts configuration (songs, artists, playlists, albums) into a structured download plan with hierarchy.

**Components:**

- `PlanGenerator`: Converts configuration to download plan
- `SpotifyClient`: Fetches metadata from Spotify API
- Creates hierarchical structure (artists → albums → tracks, playlists → tracks)

**Output:**

- `DownloadPlan` with all items in hierarchical structure
- Saved to `download_plan.json` (if persistence enabled)

#### Phase 2: Plan Optimization

Removes duplicates, checks for existing files, sorts items for optimal processing.

**Components:**

- `PlanOptimizer`: Removes duplicates, checks files, sorts items
- File existence checking with caching
- Duplicate detection across all sources

**Output:**

- Optimized `DownloadPlan` with duplicates removed
- Saved to `download_plan_optimized.json` (if persistence enabled)

#### Phase 3: Plan Execution

Executes downloads in parallel with detailed progress tracking.

**Components:**

- `PlanExecutor`: Executes plan with parallel processing
- `Downloader`: Handles individual track downloads
- `AudioProvider`: Sources audio from YouTube/SoundCloud
- `MetadataEmbedder`: Embeds metadata into files

**Output:**

- Downloaded music files with embedded metadata
- Progress saved to `download_plan_progress.json` (if persistence enabled)

## Core Modules

### Spotify Client

- **Purpose**: Spotify Web API client with rate limiting and caching
- **Features**:
  - Automatic retry with exponential backoff
  - Proactive rate limiting to prevent hitting API limits
  - In-memory caching with TTL and LRU eviction
  - Respects `Retry-After` headers

### Audio Provider

- **Purpose**: yt-dlp wrapper for downloading from YouTube/SoundCloud
- **Features**:
  - Multiple provider support (YouTube Music, YouTube, SoundCloud)
  - Audio format conversion (MP3, FLAC, M4A, Opus)
  - Bitrate control
  - Search result caching

### Metadata Embedder

- **Purpose**: Mutagen-based metadata embedding
- **Features**:
  - Album art embedding
  - Track information (title, artist, album, year, etc.)
  - Format-specific metadata handling

### Plan Generator

- **Purpose**: Converts configuration to download plan
- **Features**:
  - Processes songs, artists, playlists, albums
  - Creates hierarchical structure
  - Handles Spotify API rate limiting

### Plan Optimizer

- **Purpose**: Optimizes download plan
- **Features**:
  - Removes duplicate tracks
  - Checks for existing files
  - Sorts items for optimal processing order

### Plan Executor

- **Purpose**: Executes plan with parallel processing
- **Features**:
  - ThreadPoolExecutor for parallel downloads
  - Thread-safe status updates
  - Graceful shutdown with progress saving
  - Detailed progress tracking

## Supporting Systems

### Caching

- **In-Memory Cache**: Simple LRU cache with TTL expiration
- **Cache Types**:
  - Spotify API responses (default: 1000 entries, 1 hour TTL)
  - Audio search results (default: 500 entries, 24 hour TTL)
  - File existence checks (default: 10000 entries, 1 hour TTL)

### Configuration Management

- **Config Manager**: Centralized config loading, saving, and validation
- **Update Queue**: Config updates are queued and applied after current download completes
- **Version**: Configuration file version 1.2
- **Validation**: YAML validation with schema checking
- **Digest**: SHA256 digest for config change detection

### Process Management

- **Service Manager**: Spawns download service as child process
- **Crash Detection**: Monitors process health, detects crashes
- **State Cleanup**: Automatically cleans up plan files on crash
- **Graceful Shutdown**: Coordinates shutdown between web server and download service
- **Resource Limits**: Container-level limits apply to both processes

### Error Handling

- **Error Types**: Structured error responses via gRPC
- **Retry Logic**: Exponential backoff with jitter
- **Graceful Degradation**: Continues processing on individual item failures
- **Crash Recovery**: Automatic state cleanup on process crash
- **Version Compatibility**: Strict version checking prevents incompatible operations

## Plan Persistence

When `plan_persistence_enabled` is true, plans are saved to:

- `download_plan.json`: Initial plan after generation
- `download_plan_optimized.json`: Optimized plan after optimization
- `download_plan_progress.json`: Progress snapshot (updated during execution, saved on shutdown)

By default, plan files are saved to `/var/lib/musicdl/plans` (configurable via `MUSICDL_PLAN_PATH` environment variable).

## Status Reporting

When `plan_status_reporting_enabled` is true, plans are saved during generation and optimization phases for status display. This enables the healthcheck server to report accurate status during all phases of execution.

**Phase Tracking:**

- `generating`: Plan generation in progress
- `optimizing`: Plan optimization in progress
- `executing`: Plan execution in progress

## Web Server Endpoints

The web server provides HTTP endpoints for monitoring, control, and status display. It runs as the main control plane and provides real-time information about download service execution.

### Endpoints

**`/health` (JSON)**
- Docker HEALTHCHECK endpoint for container health monitoring
- Returns HTTP 200 (healthy) or 503 (unhealthy) status codes
- Response includes:
  - `status`: "healthy" or "unhealthy"
  - `reason`: Human-readable reason for status
  - `timestamp`: Current Unix timestamp
  - `plan_file`: Name of plan file being used
  - `statistics`: Plan statistics (total items, by status, by type)
  - `phase`: Current execution phase (if available)
  - `rate_limit`: Rate limit information (if active)

**`/status` (HTML)**
- Human-readable status dashboard
- Features:
  - Real-time download progress and statistics
  - Plan phase tracking with timestamps
  - Spotify rate limit warnings with countdown timer
  - Detailed plan item status table
  - Auto-refresh capability (configurable via `?refresh=N` query parameter)
  - Link to log viewer

**`/logs` (HTML)**
- Styled log viewer for application logs
- Features:
  - View all application logs in console-style format
  - Filter by log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
  - Search logs by keyword (case-insensitive)
  - Filter by time range (start time and end time)
  - Search result highlighting
  - Auto-refresh capability (configurable via `?refresh=N` query parameter)
  - Color-coded log levels
  - Responsive design for mobile devices

### Configuration

- **Port**: Configurable via `HEALTHCHECK_PORT` environment variable (default: 8080)
- **gRPC Port**: Internal port for download service (default: 30025, configurable)
- **Plan Directory**: Uses `MUSICDL_PLAN_PATH` environment variable (default: `/var/lib/musicdl/plans`)
- **Log File**: Uses `MUSICDL_LOG_PATH` environment variable (default: `/var/lib/musicdl/logs/musicdl.log`)
- **Config Path**: Uses `CONFIG_PATH` environment variable (default: `/scripts/config.yaml`)

### Rate Limit Warning Detection

The healthcheck server automatically detects and displays Spotify rate limit warnings:

- **Automatic Detection**: Custom logging handler intercepts `spotipy.util` WARNING messages
- **Real-time Updates**: Rate limit information is immediately saved to plan metadata
- **Status Display**: Shows on `/status` page with:
  - Active rate limit indicator
  - Retry after time (in seconds)
  - Time remaining countdown
  - Expiration timestamp
  - Detection timestamp
- **Log Visibility**: Rate limit warnings appear in `/logs` viewer with WARNING level highlighting

### Logging

Application logs use structured JSON logging:

- **Format**: JSON lines with timestamp, level, service, operation, message, and optional error
- **Location**: Single log file shared by web server and download service (default: `/var/lib/musicdl/logs/musicdl.log`)
- **Service Tags**: Each log entry includes service name (`web-server` or `download-service`)
- **Streaming**: Real-time log streaming via `/api/logs/stream` endpoint (Server-Sent Events)
- **Filtering**: Filter by level, time range, search terms, and service

**Log Entry Structure:**
```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "level": "INFO",
  "service": "download-service",
  "operation": "StartDownload",
  "message": "Download started successfully"
}
```

The log file is automatically created and managed by the application. Logs are accessible via the `/logs` endpoint for convenient viewing and filtering.

## Rate Limiting

### Spotify API Rate Limiting

- **Proactive Throttling**: Prevents hitting rate limits by throttling requests
- **Reactive Retry**: Automatically retries on HTTP 429 with exponential backoff
- **Retry-After Support**: Respects Spotify's `Retry-After` header
- **Warning Detection**: Automatically intercepts and displays spotipy rate limit warnings
  - Custom logging handler detects `spotipy.util` WARNING messages
  - Updates plan metadata with rate limit information
  - Displays on status page with countdown timer
  - Visible in log viewer for debugging

**Configuration:**

- `spotify_rate_limit_enabled`: Enable/disable (default: true)
- `spotify_rate_limit_requests`: Max requests per window (default: 10)
- `spotify_rate_limit_window`: Window size in seconds (default: 1.0)

### Download Rate Limiting

- **Request Throttling**: Limits concurrent download requests
- **Bandwidth Limiting**: Optional bandwidth cap in bytes per second

**Configuration:**

- `download_rate_limit_enabled`: Enable/disable (default: true)
- `download_rate_limit_requests`: Max requests per window (default: 2)
- `download_rate_limit_window`: Window size in seconds (default: 1.0)
- `download_bandwidth_limit`: Bandwidth limit in bytes/sec (default: 1048576 = 1MB/sec)

## Key Differences from spotDL

1. **No spotDL Package**: Direct implementation using spotDL's dependencies
2. **Simplified Architecture**: No singleton patterns, simpler provider abstraction
3. **Single Configuration**: One YAML file (version 1.2) instead of split config
4. **In-Memory Caching**: Simple cache implementation (no file persistence)
5. **Focused Features**: Only core download functionality (no web UI, sync, etc.)
6. **Plan-Based Architecture**: Better optimization and parallelization
