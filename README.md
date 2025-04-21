# `musicdl`

Personal music downloader using `spotdl`.

## Usage

1. Clone repository:

    ``` bash
    git clone git@gitlab.com/sv4u/musicdl.git
    ```

2. Build image:

    ``` bash
    docker build -f Dockerfile -t musicdl:latest .
    ```

3. Start container with music library mapped to `/download`:

    ``` bash
    docker run -t musicdl:latest -v /path/to/music/library:/download
    ```
