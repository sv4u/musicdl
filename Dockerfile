# SECURITY find a safter image
FROM python:3-bullseye

# Add author/maintainer labels
LABEL org.opencontainers.image.authors="sasank@vishnubhatlas.net"
LABEL version="1.0"
LABEL description="This image allows for quick execution of musicdl"

# NOTE: volume mount map music library to /download
# i.e. docker run -v <path-to-music-library>:/download musicdl:latest

RUN apt-get update && \
	apt-get install -y ca-certificates \
	ffmpeg openssl aria2 g++ git python3-cffi \
	libffi-dev zlib1g-dev

RUN pip install --upgrade pip pyyaml spotdl

RUN mkdir -p /scripts

# Copy script and configuration
COPY download.py /scripts/download.py
COPY config.yaml /scripts/config.yaml

# Move into /download
WORKDIR /download

# Run download.py on container start
ENTRYPOINT ["python3"]
CMD ["/scripts/download.py", "/scripts/config.yaml"]

