# Stage 1: Build Go binary
FROM golang:1.24-alpine AS builder

# Accept VERSION as build argument (optional - if not provided, will use Git tags)
ARG VERSION

# Install build dependencies (including protoc for Protocol Buffers)
RUN apk add --no-cache \
	git \
	ca-certificates \
	protobuf \
	protobuf-dev

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Install Go protobuf plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Protocol Buffers code
RUN protoc --go_out=. --go_opt=paths=source_relative \
	--go-grpc_out=. --go-grpc_opt=paths=source_relative \
	download/proto/musicdl.proto || (echo "Proto generation failed" && exit 1)

# Determine version: use Git tag if on a tag, otherwise use Git commit
# If VERSION build arg is provided, use it; otherwise calculate from Git
RUN if [ -n "$VERSION" ]; then \
		BUILD_VERSION="$VERSION"; \
	else \
		# Fetch tags in case of shallow clone
		git fetch --tags --force >/dev/null 2>&1 || true; \
		# Check if we're on a tag (exact match)
		if git describe --exact-match --tags HEAD >/dev/null 2>&1; then \
			BUILD_VERSION=$(git describe --exact-match --tags HEAD); \
		# Check if we can describe HEAD with a tag (includes commits after tag)
		elif git describe --tags HEAD >/dev/null 2>&1; then \
			BUILD_VERSION=$(git describe --tags HEAD); \
		# Fallback to commit hash (short, 7 chars)
		elif git rev-parse --short HEAD >/dev/null 2>&1; then \
			BUILD_VERSION=$(git rev-parse --short HEAD); \
		# Final fallback
		else \
			BUILD_VERSION="dev"; \
		fi; \
	fi && \
	echo "Building musicdl with version: $BUILD_VERSION" && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-w -s -X main.Version=$BUILD_VERSION" \
		-o /usr/local/bin/musicdl \
		./control

# Stage 2: Runtime image
FROM alpine:latest

# Add author/maintainer labels
LABEL org.opencontainers.image.authors="sasank@vishnubhatlas.net"
LABEL version="1.2"
LABEL description="This image allows for quick execution of musicdl"

# Environment variables
ENV CONFIG_PATH=/scripts/config.yaml
ENV MUSICDL_PLAN_PATH=/var/lib/musicdl/plans
ENV MUSICDL_LOG_PATH=/var/lib/musicdl/logs/musicdl.log
ENV HEALTHCHECK_PORT=8080

# Install runtime dependencies
# - ca-certificates: for HTTPS requests
# - curl: for healthcheck
# - ffmpeg: for audio conversion
# - python3: for yt-dlp and mutagen (Python tools, still needed as subprocess)
# - py3-pip: to install yt-dlp and mutagen
# Note: --break-system-packages is required for Alpine Linux (PEP 668)
RUN apk add --no-cache \
	ca-certificates \
	curl \
	ffmpeg \
	python3 \
	py3-pip \
	&& pip3 install --break-system-packages --no-cache-dir yt-dlp mutagen \
	&& rm -rf /var/cache/apk/*

# Verify yt-dlp and mutagen are available
RUN python3 -c "import yt_dlp; print(f'yt-dlp version: {yt_dlp.version.__version__}')" || \
	(echo "ERROR: yt-dlp not found in container" && exit 1)
RUN python3 -c "import mutagen; print(f'mutagen version: {mutagen.version_string}')" || \
	(echo "ERROR: mutagen not found in container" && exit 1)

# Create necessary directories
RUN mkdir -p /scripts /download /var/lib/musicdl/plans /var/lib/musicdl/logs && \
	chmod 755 /download && \
	chmod 755 /var/lib/musicdl/plans && \
	chmod 755 /var/lib/musicdl/logs

# Copy Go binary from builder stage
COPY --from=builder /usr/local/bin/musicdl /usr/local/bin/musicdl

# Copy default configuration
COPY config.yaml /scripts/config.yaml

# Create entrypoint script
# The entrypoint runs the Go control platform with 'serve' command
# Version will be printed by the Go application itself
RUN printf '#!/bin/sh\nset -e\n# Run control platform server\nexec /usr/local/bin/musicdl serve --port "${HEALTHCHECK_PORT:-8080}" --config "${CONFIG_PATH:-/scripts/config.yaml}" --plan-path "${MUSICDL_PLAN_PATH:-/var/lib/musicdl/plans}" --log-path "${MUSICDL_LOG_PATH:-/var/lib/musicdl/logs/musicdl.log}"\n' > /scripts/entrypoint.sh && \
	chmod +x /scripts/entrypoint.sh

# Create healthcheck script
# Uses the control platform /api/health endpoint
RUN printf '#!/bin/sh\n# Healthcheck wrapper that reads HEALTHCHECK_PORT environment variable\nPORT="${HEALTHCHECK_PORT:-8080}"\ncurl -f "http://localhost:${PORT}/api/health" || exit 1\n' > /scripts/healthcheck.sh && \
	chmod +x /scripts/healthcheck.sh

# Set working directory to download location
WORKDIR /download

# Run control platform on container start
ENTRYPOINT ["/scripts/entrypoint.sh"]

# Healthcheck instruction for Docker
# Checks /api/health endpoint every 30 seconds with 10 second timeout
# Allows 10 seconds start period for server
# Retries 3 times before marking unhealthy
# Uses wrapper script to read HEALTHCHECK_PORT environment variable (defaults to 8080)
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
	CMD /scripts/healthcheck.sh
