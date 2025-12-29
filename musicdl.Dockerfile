FROM python:3.12-bookworm

# Add author/maintainer labels
LABEL org.opencontainers.image.authors="sasank@vishnubhatlas.net"
LABEL version="1.2"
LABEL description="This image allows for quick execution of musicdl"

# Environment variable for config path override
# Default to built-in config, but allow override via volume mount
ENV CONFIG_PATH=/scripts/config.yaml

RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates curl ffmpeg openssl aria2 g++ \
	git python3-cffi libffi-dev zlib1g-dev && \
	rm -rf /var/lib/apt/lists/*

COPY ./requirements.txt /tmp/requirements.txt
RUN python3 -m pip install --upgrade pip && \
	python3 -m pip install -r /tmp/requirements.txt

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

