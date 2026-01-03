# Stage 1: Build dependencies and compile Python packages
FROM python:3.12-slim AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
	g++ \
	git \
	python3-cffi \
	libffi-dev \
	zlib1g-dev \
	&& rm -rf /var/lib/apt/lists/*

# Copy requirements and install Python packages
COPY ./requirements.txt /tmp/requirements.txt
RUN python3 -m pip install --upgrade pip && \
	python3 -m pip install --user --no-cache-dir -r /tmp/requirements.txt

# Stage 2: Runtime image
FROM python:3.12-slim

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
RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates \
	curl \
	ffmpeg \
	openssl \
	aria2 \
	&& rm -rf /var/lib/apt/lists/*

# Copy Python packages from builder stage
COPY --from=builder /root/.local /root/.local

# Make sure scripts in .local are usable
ENV PATH=/root/.local/bin:$PATH

RUN mkdir -p /scripts /download && \
	chmod 755 /download

# Copy script, core module, and default configuration
COPY download.py /scripts/download.py
COPY core/ /scripts/core/
COPY config.yaml /scripts/config.yaml

# Create entrypoint script that respects CONFIG_PATH env var
# Set PYTHONPATH to include /scripts so Python can find the core module
RUN printf '#!/bin/sh\nexport PYTHONPATH=/scripts:$PYTHONPATH\npython3 /scripts/download.py "${CONFIG_PATH:-/scripts/config.yaml}"\n' > /scripts/entrypoint.sh && \
	chmod +x /scripts/entrypoint.sh

# Set working directory to download location
WORKDIR /download

# Run download.py on container start
# Config path can be overridden via CONFIG_PATH env var or by mounting a volume
ENTRYPOINT ["/scripts/entrypoint.sh"]

