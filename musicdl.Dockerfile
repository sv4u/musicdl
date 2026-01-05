# Stage 1: Build dependencies and compile Python packages
FROM python:3.14-slim AS builder

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

# Copy Git repository and version script to determine version from tags
# We need .git directory to determine version from tags
COPY ./.git /tmp/repo/.git
COPY ./scripts/get_version.py /tmp/repo/scripts/get_version.py
COPY ./__init__.py /tmp/repo/__init__.py

# Reset Git state to match HEAD commit (ensures clean working tree)
# This is needed because COPY creates files that Git sees as untracked
# We only reset the index, not clean untracked files (we need the copied files)
RUN cd /tmp/repo && git reset --hard HEAD

# Run version script to update __init__.py with version from Git tags
# This will use the latest tag (with v prefix) or commit hash for dirty trees
RUN cd /tmp/repo && \
	python3 scripts/get_version.py > /tmp/version.txt && \
	cat /tmp/version.txt

# Stage 2: Runtime image
FROM python:3.14-slim

# Add author/maintainer labels
LABEL org.opencontainers.image.authors="sasank@vishnubhatlas.net"
LABEL version="1.2"
LABEL description="This image allows for quick execution of musicdl"

# Environment variable for config path override
# Default to built-in config, but allow override via volume mount
ENV CONFIG_PATH=/scripts/config.yaml

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

RUN mkdir -p /scripts /download && \
	chmod 755 /download

# Copy script, core module, default configuration, and version file
# Use the __init__.py that was updated with version from Git tags in builder stage
COPY download.py /scripts/download.py
COPY core/ /scripts/core/
COPY --from=builder /tmp/repo/__init__.py /scripts/__init__.py
COPY config.yaml /scripts/config.yaml

# Create entrypoint script that respects CONFIG_PATH env var
# Set PYTHONPATH to include /scripts so Python can find the core module
# Packages are in standard site-packages location, so no need to add them to PYTHONPATH
# Print container version at startup for verification
# Use proper error handling and exit codes
RUN printf '#!/bin/sh\nset -e\nexport PYTHONPATH=/scripts:$PYTHONPATH\n# Print container version information\npython3 -c "import sys; sys.path.insert(0, \"/scripts\"); from __init__ import __version__; import yt_dlp; print(f\"musicdl container version: {__version__}\"); print(f\"yt-dlp version: {yt_dlp.version.__version__}\"); print(f\"Python version: {sys.version.split()[0]}\")" 2>/dev/null || echo "Warning: Could not determine version information"\nexec python3 /scripts/download.py "${CONFIG_PATH:-/scripts/config.yaml}"\n' > /scripts/entrypoint.sh && \
	chmod +x /scripts/entrypoint.sh

# Set working directory to download location
WORKDIR /download

# Run download.py on container start
# Config path can be overridden via CONFIG_PATH env var or by mounting a volume
ENTRYPOINT ["/scripts/entrypoint.sh"]

