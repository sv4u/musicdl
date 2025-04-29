FROM python:3.12-bookworm

# Add author/maintainer labels
LABEL org.opencontainers.image.authors="sasank@vishnubhatlas.net"
LABEL version="1.1"
LABEL description="This image allows for quick execution of musicdl"

# NOTE: volume mount map music library to /download
# i.e. docker run -v <path-to-music-library>:/download musicdl:latest

RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates curl ffmpeg openssl aria2 g++ \
	git python3-cffi libffi-dev zlib1g-dev

COPY ./requirements.txt /tmp/requirements.txt
RUN python3 -m pip install --upgrade pip
RUN python3 -m pip install -r /tmp/requirements.txt

RUN mkdir -p /scripts

# Copy script and configuration
COPY download.py /scripts/download.py
COPY config.yaml /scripts/config.yaml

# Move into /download
WORKDIR /download

# Run download.py on container start
ENTRYPOINT ["python3"]
CMD ["/scripts/download.py", "/scripts/config.yaml"]

