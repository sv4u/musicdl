# TrueNAS Scale Deployment

This guide provides instructions for deploying musicdl on TrueNAS Scale as a CLI-only tool with scheduled execution via cron.

## Table of Contents

- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Detailed Deployment](#detailed-deployment)
- [Configuration](#configuration)
- [Usage and Scheduling](#usage-and-scheduling)
- [Troubleshooting](#troubleshooting)
- [Advanced Topics](#advanced-topics)
- [Reference](#reference)

## Introduction

musicdl is a CLI-only tool that downloads music from Spotify by sourcing audio from YouTube and other providers, then embedding metadata. It has no web UI or API. You run two commands: `musicdl plan <config-file>` to generate a download plan, then `musicdl download <config-file>` to perform the downloads.

This deployment provides:

- **CLI-only execution**: Run `musicdl plan` and `musicdl download` via Docker (manually or on a schedule).
- **Configurable music library**: Use a host volume (e.g. TrueNAS dataset) as the working directory for downloads.
- **Scheduled runs**: Use TrueNAS Task Scheduler (Cron Jobs) to run plan and download at set times.
- **Cache and plan persistence**: Plan and caches live under `.cache/` in the work directory (or `MUSICDL_CACHE_DIR`).

## Prerequisites

Before deploying, ensure you have:

1. **TrueNAS Scale** (tested on 25.10.1) with Docker/containerd available.
2. **Network access** for GitHub Container Registry, Spotify Web API, and YouTube/YouTube Music.
3. **Music library dataset** (e.g. `/mnt/peace-house-storage-pool/peace-house-storage/Music`).
4. **Config file** with Spotify credentials and sources (songs, artists, playlists). Credentials can be in the config file or via `SPOTIFY_CLIENT_ID` / `SPOTIFY_CLIENT_SECRET`.

## Quick Start

1. **Prepare config** (e.g. `config.yaml`) with `version: "1.2"`, Spotify credentials, and your `songs` / `artists` / `playlists`. Place it on the host (e.g. next to your music dataset or in a configs dataset).

2. **Run once manually** to verify:

   ```bash
   # Create a directory for config and cache (e.g. on your dataset)
   # Mount that directory as the container work dir so .cache and downloads live there.
   docker run --rm \
     -v /mnt/peace-house-storage-pool/peace-house-storage/Music:/download:rw \
     -v /path/on/host/config.yaml:/download/config.yaml:ro \
     -w /download \
     ghcr.io/sv4u/musicdl:latest \
     musicdl plan /download/config.yaml

   docker run --rm \
     -v /mnt/peace-house-storage-pool/peace-house-storage/Music:/download:rw \
     -v /path/on/host/config.yaml:/download/config.yaml:ro \
     -w /download \
     ghcr.io/sv4u/musicdl:latest \
     musicdl download /download/config.yaml
   ```

3. **Schedule with TrueNAS Cron** (see [Usage and Scheduling](#usage-and-scheduling)): add a cron job that runs a script (or two jobs) to execute the two commands above so plan and download run on a schedule.

## Detailed Deployment

### Step 1: Prepare config and paths

1. Create `config.yaml` with your Spotify credentials and download settings (see [Configuration](#configuration)).
2. Choose a **work directory** that the container will use: this will hold both the music output and the `.cache` directory (plans and caches). Typically this is your music library path (e.g. `/mnt/pool/dataset/Music`).
3. Mount the same work directory and config path into every `docker run` so that `musicdl plan` and `musicdl download` see the same `.cache` and config.

### Step 2: Volume layout

Use a single work directory for both output and cache:

- **Work dir** (e.g. `/download` in container): mount your music dataset here. Downloads go here; `.cache` is created here (or under `MUSICDL_CACHE_DIR` if set).
- **Config file**: mount your `config.yaml` into the work dir (e.g. `/download/config.yaml`) or another path and pass that path to `plan` and `download`.

Example:

```yaml
# For cron or scripts, use the same mounts for both plan and download:
# - Host music path -> /download (work dir)
# - Host config file -> /download/config.yaml (or another path)
```

### Step 3: Set up scheduled execution

Use TrueNAS **Tasks → Cron Jobs** to run plan and download. Two options:

**Option A – Single cron job running a script**

Create a script on the host (e.g. `/path/to/musicdl-run.sh`):

```bash
#!/bin/sh
WORK_DIR="/mnt/peace-house-storage-pool/peace-house-storage/Music"
CONFIG="/path/on/host/config.yaml"   # or mount under $WORK_DIR

docker run --rm \
  -v "$WORK_DIR:/download:rw" \
  -v "$CONFIG:/download/config.yaml:ro" \
  -w /download \
  ghcr.io/sv4u/musicdl:latest \
  musicdl plan /download/config.yaml || exit 1

docker run --rm \
  -v "$WORK_DIR:/download:rw" \
  -v "$CONFIG:/download/config.yaml:ro" \
  -w /download \
  ghcr.io/sv4u/musicdl:latest \
  musicdl download /download/config.yaml
```

Make it executable (`chmod +x /path/to/musicdl-run.sh`), then add a Cron Job that runs this script at your desired schedule (e.g. daily at 2 AM: `0 2 * * *`).

**Option B – Two cron jobs**

- First job: run `musicdl plan ...` with the same volume mounts, at e.g. `0 2 * * *`.
- Second job: run `musicdl download ...` with the same mounts, a few minutes later (e.g. `5 2 * * *`).

Ensure both use the same work dir and config path so the download step finds the plan in `.cache/`.

### Step 4: Deploy via TrueNAS Apps (optional)

If you use TrueNAS Apps (Docker Compose):

- Define a service that uses the musicdl image and mounts the work dir and config. The service **command** can be something that keeps the container alive (e.g. `sleep infinity`) so you can use **Tasks → Cron Jobs** to run:

  ```bash
  docker exec musicdl musicdl plan /download/config.yaml
  docker exec musicdl musicdl download /download/config.yaml
  ```

  Alternatively, don’t run the container continuously; have cron run `docker run --rm ...` as in Option A/B above (no Compose service needed).

### Step 5: Post-deployment checks

- Run `musicdl plan` and `musicdl download` once manually (as in Quick Start) and confirm files appear under the work dir and that `.cache/download_plan_<hash>.json` is created.
- Check exit codes: plan returns 0 on success, 1 on config error; download returns 0 on success, 2 if no plan or hash mismatch.
- Check logs: use `docker logs` if running a long-lived container, or capture stdout/stderr from the cron script.

## Configuration

### Config file structure

musicdl supports **spec layout** (top-level `spotify`, `threads`, `rate_limits`) or **legacy layout** (`download.client_id`, `download.client_secret`). Example (legacy style):

```yaml
version: "1.2"

download:
  client_id: "your_spotify_client_id"
  client_secret: "your_spotify_client_secret"
  threads: 4
  output: "{artist}/{album}/{track-number} - {title}.{output-ext}"

songs: []
artists: []
playlists: []
```

Credentials can also be provided via environment variables: `SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET` (and optionally `SPOTIFY_REDIRECT_URI`). Pass them when running the container, e.g.:

```bash
docker run --rm -e SPOTIFY_CLIENT_ID=... -e SPOTIFY_CLIENT_SECRET=... \
  -v "$WORK_DIR:/download:rw" -v "$CONFIG:/download/config.yaml:ro" \
  -w /download ghcr.io/sv4u/musicdl:latest \
  musicdl plan /download/config.yaml
```

### Environment variables

| Variable | Description |
|----------|-------------|
| `MUSICDL_WORK_DIR` | Working directory (default: current dir). Should match the mounted volume used as output/cache root. |
| `MUSICDL_CACHE_DIR` | Override cache directory (default: `<work_dir>/.cache`). Plan and caches live here. |
| `SPOTIFY_CLIENT_ID` | Spotify app client ID (optional if in config). |
| `SPOTIFY_CLIENT_SECRET` | Spotify app client secret (optional if in config). |
| `TZ` | Timezone (e.g. `America/Denver`) for logs. |

Config file path is **not** an environment variable; you pass it as the argument to `musicdl plan <path>` and `musicdl download <path>`.

## Usage and Scheduling

### Manual run

From the host:

```bash
WORK_DIR="/mnt/peace-house-storage-pool/peace-house-storage/Music"
CONFIG="/path/to/config.yaml"

docker run --rm -v "$WORK_DIR:/download:rw" -v "$CONFIG:/download/config.yaml:ro" \
  -w /download ghcr.io/sv4u/musicdl:latest musicdl plan /download/config.yaml

docker run --rm -v "$WORK_DIR:/download:rw" -v "$CONFIG:/download/config.yaml:ro" \
  -w /download ghcr.io/sv4u/musicdl:latest musicdl download /download/config.yaml
```

### Cron schedule examples

| Schedule | Description |
|----------|-------------|
| `0 2 * * *` | Daily at 2:00 AM |
| `0 */6 * * *` | Every 6 hours |
| `0 3 * * 0` | Weekly on Sunday at 3:00 AM |
| `5 2 * * *` | Daily at 2:05 AM (use for download if plan runs at 2:00) |

### Exit codes

- **plan**: 0 success, 1 config error, 2 network error, 3 filesystem error.
- **download**: 0 success, 1 config error, 2 plan not found/hash mismatch, 3 network, 4 filesystem, 5 partial success.

Use these in your script to decide whether to run download or alert (e.g. don’t run download if plan exited non-zero).

## Troubleshooting

### Plan or download exits with code 1

- **Config error.** Check config file path and YAML syntax; ensure `version: "1.2"`, valid Spotify credentials (in config or env), and required fields (e.g. `output` containing `{title}`).

### Download exits with code 2

- **Plan not found or config hash mismatch.** Run `musicdl plan` first with the **same** config file path. Don’t change the config between plan and download, or a new plan (new hash) will be generated and the previous plan won’t be used. Ensure the same work dir and config path are used for both commands so `.cache/download_plan_<hash>.json` is found and matches the current config hash.

### No files in music library

- Confirm the work dir volume is mounted correctly and is writable (`docker run ... -v "$WORK_DIR:/download:rw"`).
- Check that `output` in config uses a path relative to the work dir (e.g. `{artist}/{album}/...`).
- Run with `docker run --rm ...` and watch stdout/stderr for errors.

### Cron job doesn’t run

- In TrueNAS: **Tasks → Cron Jobs** → ensure the job is **Enabled** and the schedule is correct.
- Ensure the cron user has permission to run `docker` (or the script that runs docker).
- Run the script or command manually to verify it works; check system logs for cron execution.

### Permission errors

- Ensure the dataset (work dir) has permissions that allow the container user to write (e.g. `chmod` or dataset ACLs). If the image runs as a specific UID/GID, match dataset ownership or permissions accordingly.

## Advanced Topics

### Multiple configs

Use separate work directories (or separate `MUSICDL_CACHE_DIR`) per config so each has its own `.cache` and plan file. Run separate cron jobs (or script with different args) for each.

### Resource limits

When running via `docker run`, you can add resource limits, e.g.:

```bash
docker run --rm --cpus="1" --memory="1g" ...
```

### Backup

Back up the config file and, if desired, the `.cache` directory (optional; plan can be regenerated with `musicdl plan`). Music files are on your dataset and can be backed up with normal TrueNAS backup/replication.

## Reference

### Cron format

```text
* * * * *
│ │ │ │ │
│ │ │ │ └── Day of week (0–7, 0 or 7 = Sunday)
│ │ │ └──── Month (1–12)
│ │ └────── Day of month (1–31)
│ └──────── Hour (0–23)
└────────── Minute (0–59)
```

### Useful commands

```bash
# Run plan only
docker run --rm -v "$WORK_DIR:/download:rw" -v "$CONFIG:/download/config.yaml:ro" \
  -w /download ghcr.io/sv4u/musicdl:latest musicdl plan /download/config.yaml

# Run download only (after plan)
docker run --rm -v "$WORK_DIR:/download:rw" -v "$CONFIG:/download/config.yaml:ro" \
  -w /download ghcr.io/sv4u/musicdl:latest musicdl download /download/config.yaml

# Print version
docker run --rm ghcr.io/sv4u/musicdl:latest musicdl version

# Print usage
docker run --rm ghcr.io/sv4u/musicdl:latest musicdl
```

### Links

- [musicdl GitHub Repository](https://github.com/sv4u/musicdl)
- [TrueNAS Scale Documentation](https://www.truenas.com/docs/scale/)
- [TrueNAS Cron Jobs](https://www.truenas.com/docs/scale/scaletutorials/tasks/cronjobs/)
- [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
