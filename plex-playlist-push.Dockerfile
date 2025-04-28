FROM python:3-alpine

# Add author/maintainer labels
LABEL org.opencontainers.image.authors="sasank@vishnubhatlas.net"
LABEL version="1.0"
LABEL description="This image allows for quick execution of plex-playlist-push after musicdl"

# NOTE: volume mount map music library to /download
# i.e. docker run -v <path-to-music-library>:/download plex-playlist-push:latest

# These environment variables are set via build args
ENV PLEX_USERNAME=""
ENV PLEX_PASSWORD=""
ENV PLEX_SERVER=""

RUN apk add --no-cache \
	ca-certificates ffmpeg openssl aria2 g++ \
	git py3-cffi libffi-dev zlib-dev

RUN pip install --upgrade pip plexapi

RUN mkdir -p /scripts

# Copy script and configuration
COPY plex-playlist-push.py /scripts/plex-playlist-push.py

# Move into /download
WORKDIR /download

# Run download.py on container start
ENTRYPOINT ["python3"]
CMD ["/scripts/plex-playlist-push.py"]

