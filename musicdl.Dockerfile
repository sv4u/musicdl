# Stage 1: Build dependencies and compile Python packages
FROM python:3.12-slim AS builder

# Accept VERSION as build argument (optional - if not provided, will use Git tags)
ARG VERSION

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
	g++ \
	git \
	python3-cffi \
	libffi-dev \
	zlib1g-dev \
	&& rm -rf /var/lib/apt/lists/*

# Copy requirements and install Python packages
# Install system-wide so packages are in standard site-packages location
COPY ./requirements.txt /tmp/requirements.txt
RUN python3 -m pip install --upgrade pip && \
	python3 -m pip install --no-cache-dir -r /tmp/requirements.txt

# Copy Git repository, version script, and __init__.py
# We need .git directory for Git tag detection (fallback when VERSION not provided)
COPY ./.git /tmp/repo/.git
COPY ./scripts/get_version.py /tmp/repo/scripts/get_version.py
COPY ./__init__.py /tmp/repo/__init__.py

# Reset Git state to match HEAD commit (ensures clean working tree)
# This is needed because COPY creates files that Git sees as untracked
# We only reset the index, not clean untracked files (we need the copied files)
RUN cd /tmp/repo && git reset --hard HEAD

# Set version in __init__.py
# If VERSION build arg is provided, pass it to version script (for CI builds)
# Otherwise, version script will determine from Git tags (for local builds)
RUN cd /tmp/repo && \
	if [ -n "$VERSION" ]; then \
	echo "[i] Using provided VERSION build arg: $VERSION"; \
	python3 scripts/get_version.py "$VERSION" > /tmp/version.txt; \
	else \
	echo "[i] VERSION build arg not provided, using Git tags"; \
	python3 scripts/get_version.py > /tmp/version.txt; \
	fi && \
	cat /tmp/version.txt

# Stage 2: Runtime image
FROM python:3.12-slim

# Add author/maintainer labels
LABEL org.opencontainers.image.authors="sasank@vishnubhatlas.net"
LABEL version="1.2"
LABEL description="This image allows for quick execution of musicdl"

# Environment variable for config path override
# Default to built-in config, but allow override via volume mount
ENV CONFIG_PATH=/scripts/config.yaml

# Environment variables for plan path and healthcheck port
ENV MUSICDL_PLAN_PATH=/var/lib/musicdl/plans
ENV HEALTHCHECK_PORT=8080

# Required environment variables for Spotify API credentials:
#   SPOTIFY_CLIENT_ID - Spotify API client ID (required at runtime)
#   SPOTIFY_CLIENT_SECRET - Spotify API client secret (required at runtime)
# These should be provided at runtime via:
#   - docker run -e SPOTIFY_CLIENT_ID=... -e SPOTIFY_CLIENT_SECRET=...
#   - Docker secrets
#   - Environment file (.env)
#   - Docker Compose environment section
# Do NOT set these at build time - they should be injected at runtime for security

# Install runtime dependencies only (no build tools)
# Include runtime libraries for compiled Python packages (libffi8 for CFFI, zlib1g for compression)
RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates \
	curl \
	ffmpeg \
	openssl \
	aria2 \
	libffi8 \
	zlib1g \
	&& rm -rf /var/lib/apt/lists/*

# Copy Python packages from builder stage
# Copy the entire site-packages directory to ensure all dependencies are available
# This includes all packages and their metadata, ensuring proper Python import resolution
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Verify yt-dlp is available (helps catch issues early)
# This ensures the package is properly installed and accessible
RUN python3 -c "import yt_dlp; print(f'yt-dlp version: {yt_dlp.version.__version__}')" || \
	(echo "ERROR: yt-dlp not found in container" && exit 1)

RUN mkdir -p /scripts /download /var/lib/musicdl/plans && \
	chmod 755 /download && \
	chmod 755 /var/lib/musicdl/plans

# Copy script, core module, default configuration, and version file
# Use the __init__.py that was updated with version from Git tags in builder stage
COPY download.py /scripts/download.py
COPY core/ /scripts/core/
COPY --from=builder /tmp/repo/__init__.py /scripts/__init__.py
COPY config.yaml /scripts/config.yaml
COPY scripts/healthcheck_server.py /scripts/healthcheck_server.py

# Create entrypoint script that respects CONFIG_PATH env var
# Set PYTHONPATH to include /scripts so Python can find the core module
# Packages are in standard site-packages location, so no need to add them to PYTHONPATH
# Print container version at startup for verification
# Start healthcheck server in background with monitoring, then run download.py in foreground
# Use proper error handling and exit codes
RUN printf '#!/bin/sh\nset -e\nexport PYTHONPATH=/scripts:$PYTHONPATH\n# Print container version information\npython3 -c "import sys; sys.path.insert(0, \"/scripts\"); from __init__ import __version__; import yt_dlp; print(f\"musicdl container version: {__version__}\"); print(f\"yt-dlp version: {yt_dlp.version.__version__}\"); print(f\"Python version: {sys.version.split()[0]}\")" 2>/dev/null || echo "Warning: Could not determine version information"\n# Start healthcheck server in background\necho "Starting healthcheck server on port ${HEALTHCHECK_PORT:-8080}..."\npython3 /scripts/healthcheck_server.py &\nHEALTHCHECK_PID=$!\n# Function to check if healthcheck server is ready\ncheck_healthcheck_ready() {\n\tPORT="${HEALTHCHECK_PORT:-8080}"\n\tfor i in $(seq 1 20); do\n\t\tif curl -f -s "http://localhost:${PORT}/health" > /dev/null 2>&1; then\n\t\t\techo "Healthcheck server is ready"\n\t\t\treturn 0\n\t\tfi\n\t\tsleep 0.5\n\tdone\n\techo "Warning: Healthcheck server did not become ready within 10 seconds"\n\treturn 1\n}\n# Wait for healthcheck server to be ready\ncheck_healthcheck_ready || true\n# Monitor healthcheck server in background (restart if it dies)\n# Use PID file to track the current healthcheck server process\nHEALTHCHECK_PIDFILE="/tmp/healthcheck.pid"\necho "$HEALTHCHECK_PID" > "$HEALTHCHECK_PIDFILE"\nmonitor_healthcheck() {\n\tPORT="${HEALTHCHECK_PORT:-8080}"\n\twhile true; do\n\t\tsleep 5\n\t\tif [ -f "$HEALTHCHECK_PIDFILE" ]; then\n\t\t\tCURRENT_PID=$(cat "$HEALTHCHECK_PIDFILE")\n\t\t\tif ! kill -0 "$CURRENT_PID" 2>/dev/null; then\n\t\t\t\techo "Healthcheck server died (PID $CURRENT_PID), restarting..."\n\t\t\t\tpython3 /scripts/healthcheck_server.py &\n\t\t\tNEW_PID=$!\n\t\t\techo "$NEW_PID" > "$HEALTHCHECK_PIDFILE"\n\t\t\t# Wait briefly for server to start\n\t\t\tsleep 1\n\t\t\tfi\n\t\tfi\n\t\t# Also check if server is responding (in case PID is stale)\n\t\tif ! curl -f -s "http://localhost:${PORT}/health" > /dev/null 2>&1; then\n\t\t\t# Server not responding, check if process is still running\n\t\t\tif [ -f "$HEALTHCHECK_PIDFILE" ]; then\n\t\t\t\tCURRENT_PID=$(cat "$HEALTHCHECK_PIDFILE")\n\t\t\t\tif ! kill -0 "$CURRENT_PID" 2>/dev/null; then\n\t\t\t\t\techo "Healthcheck server not responding, restarting..."\n\t\t\t\t\tpython3 /scripts/healthcheck_server.py &\n\t\t\t\tNEW_PID=$!\n\t\t\t\techo "$NEW_PID" > "$HEALTHCHECK_PIDFILE"\n\t\t\t\tsleep 1\n\t\t\t\tfi\n\t\t\tfi\n\t\tfi\n\tdone\n}\nmonitor_healthcheck &\nMONITOR_PID=$!\n# Cleanup function to kill healthcheck server and monitor on exit\n# This function reads the PID file when called, not when trap is set\ncleanup() {\n\tif [ -f "$HEALTHCHECK_PIDFILE" ]; then\n\t\tCURRENT_PID=$(cat "$HEALTHCHECK_PIDFILE" 2>/dev/null)\n\t\tif [ -n "$CURRENT_PID" ]; then\n\t\t\tkill "$CURRENT_PID" 2>/dev/null || true\n\t\tfi\n\t\trm -f "$HEALTHCHECK_PIDFILE"\n\tfi\n\tkill "$MONITOR_PID" 2>/dev/null || true\n}\n# Set up trap to call cleanup function on exit\ntrap cleanup EXIT TERM INT\n# Run download.py in foreground\nexec python3 /scripts/download.py "${CONFIG_PATH:-/scripts/config.yaml}"\n' > /scripts/entrypoint.sh && \
	chmod +x /scripts/entrypoint.sh

# Create healthcheck wrapper script that reads HEALTHCHECK_PORT environment variable
RUN printf '#!/bin/sh\n# Healthcheck wrapper that reads HEALTHCHECK_PORT environment variable\nPORT="${HEALTHCHECK_PORT:-8080}"\ncurl -f "http://localhost:${PORT}/health" || exit 1\n' > /scripts/healthcheck.sh && \
	chmod +x /scripts/healthcheck.sh

# Set working directory to download location
WORKDIR /download

# Run download.py on container start
# Config path can be overridden via CONFIG_PATH env var or by mounting a volume
ENTRYPOINT ["/scripts/entrypoint.sh"]

# Healthcheck instruction for Docker
# Checks /health endpoint every 30 seconds with 10 second timeout
# Allows 10 seconds start period for server and plan generation
# Retries 3 times before marking unhealthy
# Uses wrapper script to read HEALTHCHECK_PORT environment variable (defaults to 8080)
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
	CMD /scripts/healthcheck.sh

