# Stage 1: Build Go binary (CLI-only, no proto)
FROM golang:1.24-alpine AS builder

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

# Stage 2: Runtime image (CLI-only)
FROM alpine:latest

LABEL org.opencontainers.image.authors="sasank@vishnubhatlas.net"
LABEL version="1.2"
LABEL description="musicdl - Music download tool (CLI)"

ENV MUSICDL_WORK_DIR=/download
ENV MUSICDL_CACHE_DIR=.cache
ENV MUSICDL_LOG_LEVEL=info

RUN apk add --no-cache \
	ca-certificates \
	ffmpeg \
	python3 \
	py3-pip \
	&& pip3 install --break-system-packages --no-cache-dir yt-dlp mutagen \
	&& rm -rf /var/cache/apk/*

RUN python3 -c "import yt_dlp; print(f'yt-dlp: {yt_dlp.version.__version__}')" || (echo "ERROR: yt-dlp not found" && exit 1)
RUN python3 -c "import mutagen; print(f'mutagen: {mutagen.version_string}')" || (echo "ERROR: mutagen not found" && exit 1)

RUN mkdir -p /download && chmod 755 /download

COPY --from=builder /usr/local/bin/musicdl /usr/local/bin/musicdl

WORKDIR /download

# No default entrypoint - run ad-hoc: docker run ... musicdl plan config.yml
# Example: docker run -v $(pwd):/download musicdl musicdl plan musicdl-config.yml
