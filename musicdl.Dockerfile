# Stage 1: Build Go binary
FROM golang:1.25-alpine AS go-builder

ARG VERSION

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN if [ -n "$VERSION" ]; then \
		BUILD_VERSION="$VERSION"; \
	else \
	git fetch --tags --force >/dev/null 2>&1 || true; \
		if git describe --exact-match --tags HEAD >/dev/null 2>&1; then \
	BUILD_VERSION=$(git describe --exact-match --tags HEAD); \
		elif git describe --tags HEAD >/dev/null 2>&1; then \
	BUILD_VERSION=$(git describe --tags HEAD); \
		elif git rev-parse --short HEAD >/dev/null 2>&1; then \
	BUILD_VERSION=$(git rev-parse --short HEAD); \
		else \
			BUILD_VERSION="dev"; \
		fi; \
	fi && \
	echo "Building musicdl with version: $BUILD_VERSION" && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-w -s -X main.Version=$BUILD_VERSION" \
		-o /usr/local/bin/musicdl \
		./control

# Stage 2: Build Node.js frontend and backend
FROM node:20-alpine AS web-builder

WORKDIR /web

# Copy backend and frontend
COPY webserver/backend ./backend
COPY webserver/frontend ./frontend

# Build backend
WORKDIR /web/backend
RUN npm ci && npm run build

# Build frontend
WORKDIR /web/frontend
RUN npm ci && npm run build

# syntax=docker/dockerfile:1

# Stage 3: Runtime image
FROM alpine:latest

LABEL org.opencontainers.image.authors="sasank@vishnubhatlas.net"
LABEL version="1.2"
LABEL description="musicdl - Music download tool (CLI + Web Interface)"

ENV MUSICDL_WORK_DIR=/download
ENV MUSICDL_CACHE_DIR=.cache
ENV MUSICDL_LOG_LEVEL=info
ENV MUSICDL_API_PORT=5000
ENV PORT=3000

RUN apk add --no-cache \
	ca-certificates \
	ffmpeg \
	python3 \
	py3-pip \
	nodejs \
	npm \
	&& pip3 install --break-system-packages --no-cache-dir yt-dlp mutagen \
	&& rm -rf /var/cache/apk/*

RUN python3 -c "import yt_dlp; print(f'yt-dlp: {yt_dlp.version.__version__}')" || (echo "ERROR: yt-dlp not found" && exit 1)
RUN python3 -c "import mutagen; print(f'mutagen: {mutagen.version_string}')" || (echo "ERROR: mutagen not found" && exit 1)

RUN mkdir -p /download && chmod 755 /download

# Optional: install YouTube cookies at build time (does not embed the secret in git).
# Build: DOCKER_BUILDKIT=1 docker build --secret id=youtube_cookies,src=./cookies.txt ...
# Then set download.cookies: "/etc/musicdl/youtube-cookies.txt" in config (re-build when cookies expire).
RUN mkdir -p /etc/musicdl \
	&& chmod 755 /etc/musicdl
RUN --mount=type=secret,id=youtube_cookies,required=false \
	sh -c 'if [ -f /run/secrets/youtube_cookies ]; then \
		install -m 600 /run/secrets/youtube_cookies /etc/musicdl/youtube-cookies.txt; \
		echo "musicdl: installed /etc/musicdl/youtube-cookies.txt from build secret youtube_cookies"; \
	else \
		echo "musicdl: no youtube_cookies build secret (optional); use download.cookies or a volume at runtime"; \
	fi'

COPY --from=go-builder /usr/local/bin/musicdl /usr/local/bin/musicdl
COPY --from=web-builder /web/backend/dist /opt/musicdl/backend/dist
COPY --from=web-builder /web/backend/public /opt/musicdl/backend/public
COPY --from=web-builder /web/backend/package*.json /opt/musicdl/backend/

# Install production dependencies for backend
WORKDIR /opt/musicdl/backend
RUN npm ci --omit=dev

WORKDIR /download

# Create entrypoint script (use printf for proper Unix line endings; echo \n is literal in sh)
# Node must run from /opt/musicdl/backend so express.static('public') resolves correctly
RUN printf '%s\n' \
    '#!/bin/sh' \
    'if [ "$1" = "web" ]; then' \
    '  echo "Starting musicdl services..."' \
    '  musicdl api &' \
    '  sleep 2' \
    '  cd /opt/musicdl/backend && exec node dist/index.js' \
    'else' \
    '  exec "$@"' \
    'fi' \
    > /entrypoint.sh && chmod +x /entrypoint.sh

EXPOSE 80 3000 5000

ENTRYPOINT ["/entrypoint.sh"]
CMD ["web"]
