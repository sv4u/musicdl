# musicdl MCP Server

musicdl includes a built-in [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that gives AI assistants deep visibility into every aspect of musicdl's operation: download plans, live progress, logs, configuration, run history, health status, cache state, and your music library.

## Access Modes

The MCP server supports three access modes depending on your setup.

### 1. Standalone stdio (local Cursor/Claude Desktop)

The simplest mode. Cursor or Claude Desktop spawns `musicdl mcp` as a subprocess and communicates over stdin/stdout.

```bash
musicdl mcp
```

**Cursor configuration** (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "musicdl": {
      "command": "/path/to/musicdl",
      "args": ["mcp"],
      "env": {
        "MUSICDL_WORK_DIR": "/path/to/your/music/directory",
        "MUSICDL_CACHE_DIR": "/path/to/your/music/directory/.cache",
        "MUSICDL_LOG_DIR": "/path/to/your/music/directory/.logs"
      }
    }
  }
}
```

**Claude Desktop configuration** (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "musicdl": {
      "command": "/path/to/musicdl",
      "args": ["mcp"],
      "env": {
        "MUSICDL_WORK_DIR": "/path/to/your/music/directory"
      }
    }
  }
}
```

### 2. Standalone SSE (remote access)

Run the MCP server as an HTTP endpoint, accessible from another machine. This is useful when musicdl runs on a NAS, server, or Docker container.

```bash
musicdl mcp --sse
musicdl mcp --sse --port 9000
```

Default port is `8090`. Connect your MCP client to `http://<host>:<port>`.

**Cursor configuration** (remote machine):

```json
{
  "mcpServers": {
    "musicdl": {
      "url": "http://your-nas-ip:8090"
    }
  }
}
```

**Claude Code** (CLI):

```bash
claude mcp add -t http musicdl http://your-nas-ip:8090
```

### 3. Embedded in API server

When running `musicdl api`, the MCP server is automatically available at `/mcp` on the same port as the API. This mode has the richest data because it accesses both disk-persisted state and live in-process data (real-time download progress, active rate limits, in-memory logs).

```bash
musicdl api
musicdl api --port 5000
```

The MCP endpoint is at `http://localhost:5000/mcp`.

**Cursor configuration** (embedded mode):

```json
{
  "mcpServers": {
    "musicdl": {
      "url": "http://localhost:5000/mcp"
    }
  }
}
```

## Available Tools

The MCP server exposes 14 tools across 8 categories.

### Plan Inspection

| Tool | Description |
|------|-------------|
| `get_plan` | View the current download plan with aggregate statistics and filtered item listing. Supports filtering by status and source. |
| `search_plan` | Search plan items by name, artist, album, Spotify URL, or source URL. |
| `list_plan_files` | List all saved plan files on disk with hash, modification time, size, and track counts. |

### Download Monitoring

| Tool | Description |
|------|-------------|
| `get_download_status` | Get current download/plan operation status: running state, progress, and errors. |
| `get_stats` | Get download statistics: cumulative lifetime totals and current run metrics. |

### Log Analysis

| Tool | Description |
|------|-------------|
| `search_logs` | Search and filter structured logs by level (DEBUG/INFO/WARN/ERROR), keyword, and run directory. |
| `get_recent_logs` | Get the N most recent log entries across all runs. |
| `list_log_dirs` | List available log directories (one per run) with creation dates. |

### Configuration

| Tool | Description |
|------|-------------|
| `get_config` | Get the current musicdl configuration (raw YAML). |

### History & Analytics

| Tool | Description |
|------|-------------|
| `list_runs` | List past download runs with timing, state, and statistics. |
| `get_run_details` | Get full details for a specific run including progress snapshots. |
| `get_activity` | Get the activity timeline showing all system events. |

### Health & Recovery

| Tool | Description |
|------|-------------|
| `get_health` | Get system health status, version information (musicdl, spotigo, Go), and API state. |
| `get_recovery_status` | Get circuit breaker state and download resume progress (failed items, counts). |

### Cache Inspection

| Tool | Description |
|------|-------------|
| `get_cache_info` | Get cache directory overview: total size, plan files, stats file, resume state, history. |

### Library Browsing

| Tool | Description |
|------|-------------|
| `browse_library` | Browse downloaded music files and directories. |
| `search_library` | Search downloaded music files by filename or path. |
| `get_library_stats` | Get music library statistics: file counts by format, total size, artist/album counts. |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MUSICDL_WORK_DIR` | `.` | Base directory for config and music files |
| `MUSICDL_CACHE_DIR` | `.cache` | Directory for plans, stats, history, resume state |
| `MUSICDL_LOG_DIR` | `.logs` | Directory for structured log files |
| `MUSICDL_API_PORT` | `5000` | API server port (embedded mode) |

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  MCP Client                      │
│          (Cursor / Claude Desktop)               │
└──────────┬──────────────────┬───────────────────┘
           │ stdio            │ HTTP (SSE)
           ▼                  ▼
┌──────────────────┐  ┌──────────────────────────┐
│  musicdl mcp     │  │  musicdl api             │
│  (standalone)    │  │  (embedded at /mcp)      │
└──────┬───────────┘  └──────┬───────────────────┘
       │                     │
       ▼                     ▼
┌──────────────────────────────────────────────────┐
│              MCP Server (mcp/ package)           │
│  14 tools across 8 categories                    │
└──────┬───────────────────────┬───────────────────┘
       │                       │
       ▼                       ▼
┌──────────────┐      ┌────────────────────┐
│ File Provider│      │ Runtime Provider   │
│ (disk reads) │      │ (live API state)   │
└──────────────┘      └────────────────────┘
       │                       │
       ▼                       ▼
  .cache/                 In-memory state
  .logs/                  (download progress,
  config.yaml             rate limits, logs)
```

**Standalone mode** reads all data from disk (plans, logs, config, history, stats, resume state, library files). It works without the API server running.

**Embedded mode** layers live runtime data on top: real-time download progress, in-memory log buffer, and active Spotify rate limit information.

## Building

The MCP server is built as part of the standard musicdl binary:

```bash
go build -o musicdl ./control
```

It uses the official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk` v1.4.1).
