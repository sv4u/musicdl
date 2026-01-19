# TrueNAS Scale Deployment

This guide provides comprehensive instructions for deploying musicdl on TrueNAS Scale 25.10.1 as a custom application with scheduled execution.

## Table of Contents

- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Detailed Deployment](#detailed-deployment)
- [Configuration](#configuration)
- [Usage Examples](#usage-examples)
- [Troubleshooting](#troubleshooting)
- [Advanced Topics](#advanced-topics)
- [Reference](#reference)

## Introduction

musicdl is a personal music downloader that downloads music from Spotify by sourcing audio from YouTube and other providers, then embedding metadata into the downloaded files.

This deployment configuration provides:

- **Automated Downloads**: Scheduled execution using TrueNAS Scale's built-in Task Scheduler (Cron Jobs)
- **Configurable Music Library**: Specify your TrueNAS dataset path for downloads
- **Custom Configuration**: Mount your own config.yaml file or use the default
- **Resource Management**: CPU and memory limits to prevent resource exhaustion
- **Comprehensive Logging**: Structured logs with rotation for troubleshooting

## Prerequisites

Before deploying, ensure you have:

1. **TrueNAS Scale 25.10.1** installed and configured
2. **Docker/containerd runtime** available (included with TrueNAS Scale)
3. **Network connectivity** for:
   - Downloading Docker images from GitHub Container Registry
   - Accessing Spotify Web API (`api.spotify.com`)
   - Accessing YouTube/YouTube Music for audio downloads
4. **Music library dataset** created and accessible
   - Default path: `/mnt/peace-house-storage-pool/peace-house-storage/Music`
   - Adjust path in `compose.yaml` if using a different location
5. **Spotify API credentials** (client_id and client_secret)
   - Get credentials from [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
   - Credentials should be provided via environment variables (recommended) or config file

## Quick Start

1. **Prepare your configuration file** (optional):

   ```bash
   # Create config.yaml with your Spotify credentials
   # See Configuration section for structure
   ```

2. **Deploy via TrueNAS UI**:
   - Navigate to **Apps** → **Discover Apps**
   - Click **"Install via YAML"** (or three-dot menu → Custom App)
   - Enter application name: `musicdl`
   - Paste the contents of `compose.yaml` into the **Custom Config** field
   - Review and adjust volume paths if needed (music library, plans, logs)
   - Click **Save** to deploy

3. **Verify deployment**:
   - Check that `musicdl` container exists and is running
   - Access web UI at `http://your-truenas-ip:8080`
   - Check logs for any errors

4. **Test download** (optional):
   - Access web UI and configure your Spotify credentials
   - Add songs/artists/playlists to config
   - Start a download via the Dashboard

## Detailed Deployment

### Step 1: Prepare Configuration File (Optional)

If you want to use a custom configuration file instead of the default one in the image:

1. Create a `config.yaml` file with your Spotify credentials and download settings
2. Place it in an accessible location on your TrueNAS system
3. Uncomment and adjust the config volume mount in `compose.yaml`:

   ```yaml
   volumes:
     - /mnt/peace-house-storage-pool/peace-house-storage/Music:/download:rw
     - /path/to/your/config.yaml:/scripts/config.yaml:ro
   ```

### Step 2: Customize Volume Paths

Edit `compose.yaml` and adjust the volume paths:

```yaml
volumes:
  # Music library - replace with your actual TrueNAS dataset path
  - /mnt/your-pool/your-dataset/Music:/download:rw
  # Plan files directory (shared between web-server and download-service)
  - ${MUSICDL_PLAN_PATH:-./plans}:/var/lib/musicdl/plans:rw
  # Logs directory (shared between web-server and download-service)
  - ${MUSICDL_LOG_PATH_DIR:-./logs}:/var/lib/musicdl/logs:rw
  # Optional: Custom config file
  # - /path/to/your/config.yaml:/scripts/config.yaml:ro
```

**Note:** The web server and download service run in the same container but as separate processes. They share the volumes for plans and logs.

### Step 3: Set Up Scheduled Execution

Scheduled execution is configured using TrueNAS Scale's built-in Task Scheduler (Cron Jobs). See the "Scheduling with TrueNAS Cron Jobs" section below for detailed instructions.

### Step 4: Deploy via TrueNAS UI

1. **Navigate to Apps**:
   - Open TrueNAS web interface
   - Go to **Apps** section
   - Click **Discover Apps**

2. **Install Custom App**:
   - Click **"Install via YAML"** button (or three-dot menu → Custom App)
   - Enter application name: `musicdl`
   - Paste the complete `compose.yaml` content into **Custom Config** field
   - Review all settings

3. **Configure Environment Variables** (if customizing):
   - `TZ`: Timezone (default: `America/Denver`)
   - `CONFIG_PATH`: Config file path (default: `/scripts/config.yaml`)
   - `HEALTHCHECK_PORT`: Web server port (default: `8080`)
   - `MUSICDL_PLAN_PATH`: Plan files directory (default: `/var/lib/musicdl/plans`)
   - `MUSICDL_LOG_PATH`: Log file path (default: `/var/lib/musicdl/logs/musicdl.log`)

4. **Deploy**:
   - Click **Save** to start deployment
   - Monitor deployment status in Apps interface
   - Wait for containers to be created and started

### Step 5: Post-Deployment Verification

1. **Check Container Status**:
   - `musicdl` container should exist and be **running** (web server runs continuously)
   - The download service is spawned as a child process when downloads are started

2. **Verify Volume Mounts**:
   - Check that music library path is correct
   - Verify config file mount (if using custom config)

3. **Check Logs**:
   - View logs via TrueNAS UI: Apps → musicdl → Logs
   - Or via CLI: `docker logs musicdl`

4. **Test Manual Execution** (optional):
   - Start musicdl container manually to test
   - Check that downloads appear in music library
   - Verify logs show successful downloads

## Configuration

### Config.yaml Structure

The configuration file uses YAML format version 1.2:

```yaml
version: "1.2"

download:
  # Spotify API credentials
  # These should be provided via environment variables (recommended):
  #   SPOTIFY_CLIENT_ID
  #   SPOTIFY_CLIENT_SECRET
  # For local development, you can still add them here (not recommended for production)
  # client_id: "your_spotify_client_id"
  # client_secret: "your_spotify_client_secret"
  
  # Download settings
  threads: 4                    # Number of parallel downloads
  max_retries: 3                # Retry attempts for failed downloads
  format: "mp3"                # Audio format: mp3, flac, m4a, opus
  bitrate: "128k"              # Audio bitrate: 128k, 192k, 256k, 320k
  output: "{artist}/{album}/{track-number} - {title}.{output-ext}"
  
  # Provider settings
  audio_providers: ["youtube-music", "youtube"]
  
  # Cache settings
  cache_max_size: 1000          # Maximum cached Spotify API responses
  cache_ttl: 3600               # Cache expiration in seconds (1 hour)
  
  # File management
  overwrite: "skip"             # skip, overwrite, or metadata

# Music sources
songs: []                       # Individual songs: [{name: url}, ...]
artists: []                     # Artists to download: [{name: url}, ...]
playlists: []                   # Playlists: [{name: url}, ...]
```

### Environment Variables

| Variable | Service | Default | Description |
|----------|---------|---------|-------------|
| `CONFIG_PATH` | musicdl | `/scripts/config.yaml` | Path to config file |
| `TZ` | musicdl | `America/Denver` | Timezone for logs |
| `PYTHONUNBUFFERED` | musicdl | `1` | Python output buffering (real-time logs) |

## Usage Examples

### Starting Downloads

With the new architecture, the web server runs continuously. Downloads are started via the web UI or REST API, not via cron jobs.

**Option 1: Web UI (Recommended)**

1. Access the web UI at `http://your-truenas-ip:8080`
2. Navigate to the Dashboard or Config page
3. Configure your Spotify credentials and add songs/artists/playlists
4. Click "Start Download" on the Dashboard

**Option 2: REST API**

Use the REST API to start downloads programmatically:

```bash
# Start a download
curl -X POST http://your-truenas-ip:8080/api/download/start

# Check status
curl http://your-truenas-ip:8080/api/status

# Stop a download
curl -X POST http://your-truenas-ip:8080/api/download/stop
```

**Option 3: Scheduled Downloads (via Cron + API)**

If you want scheduled downloads, create a TrueNAS cron job that calls the API:

1. **Navigate to Task Scheduler**:
   - Go to **Tasks** → **Cron Jobs**
   - Click **Add** to create a new cron job

2. **Configure the Cron Job**:
   - **Description**: "Start musicdl download"
   - **Command**: 
     ```bash
     curl -X POST http://localhost:8080/api/download/start
     ```
   - **Schedule**: Configure using the cron schedule builder
   - **User**: Select **root** or a user with network access
   - **Enabled**: Check this box
   - Click **Save**

**Scheduling Examples:**

- **Daily at 2 AM**: `0 2 * * *`
- **Every 6 hours**: `0 */6 * * *`
- **Weekly on Sunday at 3 AM**: `0 3 * * 0`

### Accessing the Web UI

**Via Browser**:

1. Open your web browser
2. Navigate to `http://your-truenas-ip:8080`
3. You'll see the Dashboard with download status and controls

**Pages Available**:

- **Dashboard** (`/`): Overview with status, config digest, and quick actions
- **Status** (`/status`): Detailed status with per-song progress
- **Config** (`/config`): Configuration editor
- **Logs** (`/logs`): Log viewer with filtering and real-time streaming

**Via CLI**:

```bash
# Check container status
docker ps | grep musicdl

# Follow logs
docker logs -f musicdl

# Access web UI (if port is published)
curl http://localhost:8080/api/health
```

### Custom Configuration File

1. Create your `config.yaml` file
2. Place it in an accessible location (e.g., `/mnt/pool/datasets/configs/musicdl-config.yaml`)
3. Update `compose.yaml`:

   ```yaml
   volumes:
     - /mnt/peace-house-storage-pool/peace-house-storage/Music:/download:rw
     - /mnt/pool/datasets/configs/musicdl-config.yaml:/scripts/config.yaml:ro
   ```

4. Restart containers to apply changes

### Timezone Configuration

Set timezone for the container:

```yaml
environment:
  - TZ=America/Denver    # or Europe/London, Asia/Tokyo, etc.
```

## Troubleshooting

### Container Fails to Start

**Symptoms**: Container shows as "Exited" or fails to start

**Solutions**:

1. Check volume paths exist and are accessible:

   ```bash
   ls -ld /mnt/peace-house-storage-pool/peace-house-storage/Music
   ```

2. Verify Docker image is available:

   ```bash
   docker pull ghcr.io/sv4u/musicdl:latest
   ```

3. Check logs for errors:

   ```bash
   docker logs musicdl
   ```

4. Verify permissions on music library directory

### Downloads Not Appearing

**Symptoms**: Container runs but no files in music library

**Solutions**:

1. Verify volume mount path is correct:

   ```bash
   docker inspect musicdl | grep Mounts
   ```

2. Check file permissions on music library directory:

   ```bash
   ls -ld /mnt/peace-house-storage-pool/peace-house-storage/Music
   ```

3. Verify config.yaml has correct Spotify credentials
4. Check musicdl logs for errors:

   ```bash
   docker logs musicdl
   ```

5. Test manual execution to see real-time output

### Scheduled Jobs Not Running

**Symptoms**: TrueNAS cron job configured but musicdl never executes

**Solutions**:

1. Verify cron job is enabled in TrueNAS:
   - Navigate to **Tasks** → **Cron Jobs**
   - Check that the job is enabled (toggle switch is on)
2. Verify cron job command is correct:
   - Command should be: `docker start musicdl`
   - Or full docker run command if using that approach
3. Check cron job schedule:
   - Verify the schedule is set correctly
   - Use TrueNAS's schedule builder or validate cron syntax
4. Check cron job user has Docker permissions:
   - User should be **root** or have access to Docker socket
5. Test manual execution first:

   ```bash
   docker start musicdl
   ```

6. Check TrueNAS system logs for cron execution:
   - Navigate to **System** → **Logs** → **System Logs**
   - Filter for cron-related entries
7. Verify container name matches:
   - Container name in command must match: `musicdl`

### Permission Errors

**Symptoms**: "Permission denied" errors in logs

**Solutions**:

1. Check container user:

   ```bash
   docker exec musicdl id
   ```

2. Check dataset owner:

   ```bash
   ls -ld /mnt/peace-house-storage-pool/peace-house-storage/Music
   ```

3. Match UID/GID or adjust permissions:

   ```bash
   # Option 1: Adjust dataset permissions
   chmod 755 /mnt/peace-house-storage-pool/peace-house-storage/Music
   
   # Option 2: Use user directive in compose.yaml (if image supports)
   # Uncomment and adjust:
   # user: "1000:1000"
   ```

### Network Issues

**Symptoms**: Downloads fail with network errors

**Solutions**:

1. Verify internet connectivity from container:

   ```bash
   docker exec musicdl ping -c 3 8.8.8.8
   ```

2. Check DNS resolution:

   ```bash
   docker exec musicdl nslookup api.spotify.com
   ```

3. Verify firewall rules allow outbound HTTPS (port 443)
4. Test Spotify API access:

   ```bash
   docker exec musicdl curl -I https://api.spotify.com
   ```

### Container Uses Too Much Resources

**Symptoms**: High CPU or memory usage

**Solutions**:

1. Monitor resource usage:

   ```bash
   docker stats musicdl
   ```

2. Adjust resource limits in `compose.yaml`:

   ```yaml
   # v2.x syntax for standalone Docker Compose
   mem_limit: 1g          # Reduce memory limit
   mem_reservation: 256m  # Reduce memory reservation
   cpus: 0.5              # Reduce CPU limit
   ```

3. Reduce `threads` in config.yaml (fewer parallel downloads)
4. Adjust cache settings in config.yaml

## Advanced Topics

### Scheduling Method

This deployment uses TrueNAS Scale's built-in Task Scheduler (Cron Jobs) for scheduled execution. This approach:

**Advantages**:

- No additional container needed
- Native TrueNAS integration
- Simpler architecture
- Lower resource usage
- Easier to manage via TrueNAS UI

**How it works**:

- TrueNAS cron job executes `docker start musicdl` at scheduled times
- Container runs, downloads music, then exits
- Next scheduled run starts the container again

### Security Hardening

1. **Non-root Execution** (if image supports):

   ```yaml
   user: "1000:1000"  # Match your dataset owner UID/GID
   ```

2. **Read-only Root Filesystem**:

   ```yaml
   read_only: true
   tmpfs:
     - /tmp:rw,noexec,nosuid,size=100m
   ```

3. **Config File Permissions**:

   ```bash
   chmod 600 /path/to/config.yaml
   ```

4. **Docker Socket Access**:
   - Not needed (using TrueNAS cron jobs instead of container-based scheduler)

### Performance Tuning

1. **CPU Optimization**:
   - Monitor usage: `docker stats musicdl`
   - Adjust `threads` in config.yaml
   - Balance download speed vs. system load

2. **Memory Optimization**:
   - Monitor usage: `docker stats musicdl`
   - Adjust memory limits in compose.yaml
   - Tune cache settings in config.yaml

3. **Download Speed**:
   - Increase `threads` for more parallel downloads
   - Ensure adequate network bandwidth
   - Monitor system resources

### Multiple Instances

To run multiple instances with different configurations:

1. Use different container names:

   ```yaml
   container_name: musicdl-instance2
   ```

2. Use different volume paths:

   ```yaml
   volumes:
     - /mnt/pool/dataset2/Music:/download:rw
   ```

3. Use different config files:

   ```yaml
   volumes:
     - /path/to/config2.yaml:/scripts/config.yaml:ro
   ```

### Backup and Recovery

**What to Backup**:

- Config file: `/path/to/config.yaml` (if using custom)
- Compose file: Store in version control
- Music library: Use TrueNAS backup/replication

**Recovery Procedures**:

1. **Container failure**: Restart via TrueNAS UI or `docker start musicdl`
2. **Config corruption**: Restore from backup
3. **Volume issues**: Verify paths and permissions
4. **Complete reinstall**: Restore compose.yaml and config.yaml, redeploy

## Reference

### Cron Schedule Format

```text
* * * * *
│ │ │ │ │
│ │ │ │ └─── Day of week (0-7, 0 or 7 = Sunday)
│ │ │ └───── Month (1-12)
│ │ └─────── Day of month (1-31)
│ └───────── Hour (0-23)
└─────────── Minute (0-59)
```

### Common Cron Examples

| Schedule | Description |
|----------|-------------|
| `0 2 * * *` | Daily at 2:00 AM |
| `0 */6 * * *` | Every 6 hours |
| `0 3 * * 0` | Weekly on Sunday at 3:00 AM |
| `*/30 * * * *` | Every 30 minutes |
| `0 2,14 * * *` | Twice daily (2 AM and 2 PM) |
| `0 1 * * 1-5` | Weekdays only at 1:00 AM |

### Volume Path Format

TrueNAS datasets use the format:

```text
/mnt/{pool}/{dataset}/{subdirectory}
```

Example:

```text
/mnt/peace-house-storage-pool/peace-house-storage/Music
```

### Log Locations

- **TrueNAS UI**: Apps → musicdl → Logs tab
- **Docker CLI**: `docker logs musicdl`
- **Host filesystem**: `/var/lib/docker/containers/{container-id}/{container-id}-json.log`
- **Application Log File**: `/var/lib/musicdl/logs/musicdl.log` (inside container, configurable via `MUSICDL_LOG_PATH`)
- **Web Log Viewer**: `http://your-truenas-ip:30024/logs` (if healthcheck server port is published)
- **Cron Job Logs**: Check TrueNAS system logs (System → Logs → System Logs) for cron execution

### Healthcheck Server

The healthcheck server provides HTTP endpoints for monitoring and status display. To access it, publish port 8080 (or your configured `HEALTHCHECK_PORT`) in your Docker Compose configuration.

**Add to compose.yaml:**

```yaml
ports:
  - "30024:8080"  # Map host port 30024 to container port 8080
```

**Available Endpoints:**

- **`http://your-truenas-ip:30024/health`** - JSON healthcheck endpoint
  - Returns HTTP 200 (healthy) or 503 (unhealthy)
  - Used by monitoring systems and Docker HEALTHCHECK
  - Response includes status, statistics, phase, and rate limit information

- **`http://your-truenas-ip:30024/status`** - HTML status dashboard
  - Real-time download progress and statistics
  - Plan phase tracking (generating, optimizing, executing)
  - Spotify rate limit warnings with countdown timer
  - Detailed plan item status table
  - Auto-refresh capability
  - Link to log viewer

- **`http://your-truenas-ip:30024/logs`** - HTML log viewer
  - View all application logs in styled console format
  - Filter by log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
  - Search logs by keyword (case-insensitive)
  - Filter by time range (start time and end time)
  - Search result highlighting
  - Auto-refresh capability

**Rate Limit Warning Detection:**

The healthcheck server automatically detects and displays Spotify rate limit warnings:

- Warnings are automatically intercepted from spotipy logs
- Displayed on the `/status` page with countdown timer
- Visible in the `/logs` viewer with WARNING level highlighting
- Shows retry after time, expiration timestamp, and detection time

**Configuration:**

- **Port**: Set via `HEALTHCHECK_PORT` environment variable (default: 8080)
- **Plan Directory**: Set via `MUSICDL_PLAN_PATH` environment variable (default: `/var/lib/musicdl/plans`)
- **Log File**: Set via `MUSICDL_LOG_PATH` environment variable (default: `/var/lib/musicdl/logs/musicdl.log`)

**Requirements:**

- `plan_persistence_enabled: true` or `plan_status_reporting_enabled: true` in configuration
- Healthcheck server runs automatically when container starts

### Useful Commands

```bash
# Check container status
docker ps -a | grep musicdl

# View logs
docker logs musicdl

# Follow logs in real-time
docker logs -f musicdl

# Check resource usage
docker stats musicdl

# Inspect container configuration
docker inspect musicdl

# Check volume mounts
docker inspect musicdl | grep Mounts

# Manual execution
docker start musicdl

# Check cron job status (via TrueNAS UI: Tasks → Cron Jobs)
```

### Links and Resources

- [musicdl GitHub Repository](https://github.com/sv4u/musicdl)
- [TrueNAS Scale Documentation](https://www.truenas.com/docs/scale/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [TrueNAS Scale Task Scheduler (Cron Jobs)](https://www.truenas.com/docs/scale/scaletutorials/tasks/cronjobs/)
- [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
- [Cron Expression Validator](https://crontab.guru/)

## FAQ

**Q: Can I run this without scheduled execution?**
A: Yes, simply don't create a TrueNAS cron job. You can manually trigger the container when needed using `docker start musicdl` or via the TrueNAS UI.

**Q: How do I update the Docker image?**
A: Pull the new image: `docker pull ghcr.io/sv4u/musicdl:latest`, then restart the container if it's running.

**Q: Can I use a different music library path?**
A: Yes, update the volume mount path in compose.yaml to match your TrueNAS dataset path, then redeploy the application.

**Q: How do I change the download schedule?**
A: Edit the cron job in TrueNAS (Tasks → Cron Jobs → Edit) and update the schedule. You can use the schedule builder or enter a cron expression manually.

**Q: What if downloads fail?**
A: Check the logs (`docker logs musicdl`) for error messages. Common issues: invalid Spotify credentials, network problems, or permission errors.

**Q: Can I run multiple instances?**
A: Yes, use different container names and volume paths for each instance.

**Q: How do I backup my configuration?**
A: Backup your config.yaml file and compose.yaml. The music library is already on TrueNAS and can be backed up using TrueNAS backup features.

**Q: What resources does this use?**
A: Default limits: 1 CPU core, 2GB RAM. Adjust based on your system capacity and download needs.
