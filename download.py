#! /usr/bin/env python3
# download.py
#
# USAGE
#
# python3 download.py [CONFIG]
#
# SYNOPSIS
#
# The goal of this script is to read a YAML configuration file for information
# on Spotify links and corresponding spotdl file names, create spotdl files,
# and download from the spotdl files. All downloads are done in the directory
# where this script is invoked.
#
# The `artist` and `playlists` objects are sequences of maps. Each map within
# the sequence maps spotdl file name to Spotify link. So, the Spotify link is
# passed to spotdl which creates the spotdl file. Then, the spotdl file is used
# to download the music accordingly. For the `playlist` object, the spotdl file
# name also is used as the file name for the m3u playlist file created. So, the
# mapping is `{filename}: {spotify-link}` wth a `{filename}.spotdl` file always
# being created and a `{filename}.m3u` file being created for maps in the
# `playlist` object. The `threads` and `retires` items are optional and must be
# integers. The `version` object is required and must be `1.0` currently.
#
# COMMAND LINE ARGUMENT
#
# [CONFIG]      musicdl YAML configuration file containing information on
#               artists and playlists to download
#
# NOTES
#
# - generating spotdl file for artist:
#
#   ``` bash
#   spotdl save $url --config --max-retires $retires --threads $threads \
#       --format mp3 --save-file "$artist.spotdl"
#   ```
#
# - generating spotdl file for playlist:
#
#   ``` bash
#   spotdl save $url --config --max-retires $retries --threads $threads \
#       --format mp3 --save-file "$playlist.spotdl" --m3u "$playlist.m3u"
#   ```
#
# - downloading from spotdl file
#
#   ``` bash
#   spotdl download $spotdl_file --config --bitrate 128k --format mp3 \
#       --max-retires $retries --threads $threads --overwrite metadata \
#       --restrict ascii --scan-for-songs --preload
#   ```
#
# - using `--bitrate 128k` is required for mp3 files

import argparse
import os
import subprocess
import yaml

# global variables
MAX_RETRIES = 5
THREADS = 4


def read_config(config_path):
    """
    Parse and validate the YAML configuration file

    Args:
        config_path (str): Path to the YAML configuration file

    Returns:
        dict: Parsed and validated configuration data

    Raises:
        FileNotFoundError: If the configuration file does not exist.
        ValueError: If the configuration is invalid or missing required fields.
    """
    if not os.path.exists(config_path):
        raise FileNotFoundError(
            f"Configuration file '{config_path}' not found.")

    with open(config_path, 'r') as file:
        try:
            config = yaml.safe_load(file)
        except yaml.YAMLError as e:
            raise ValueError(f"Error parsing YAML file: {e}")

    # validate required fields
    if 'version' not in config or config['version'] not in [1.0, "1.0"]:
        raise ValueError(
            "Invalid or missing 'version'. Expected 1.0 or \"1.0\".")

    if 'artists' not in config or not isinstance(config['artists'], list):
        raise ValueError("Invalid or missing 'artists'.")

    if 'playlists' not in config or not isinstance(config['playlists'], list):
        raise ValueError("Invalid or missing 'artists'.")

    # validate optional fields
    if 'threads' in config and not isinstance(config['threads'], int):
        raise ValueError("'threads' must be an integer.")

    if 'threads' in config:
        THREADS = config['threads']

    if 'retries' in config and not isinstance(config['retries'], int):
        raise ValueError("'retries' must be an integer.")

    if 'retries' in config:
        MAX_RETRIES = config['retries']

    # return config now that validation is done
    return config


def create_spotdl(url, name, make_m3u):
    """
    Create spotdl file with filename 'name' from 'url'.

    Args:
        url (string): Spotify url
        name (string): Filename to use. File written will be "name.spotdl"
        make_m3u (bool): Toggle if spotdl should make an m3u file. File
            written will be "name.m3u"

    Raises:
        RuntimeError: If the shell subprocess encounters an error.
    """
    # create output filenames
    spotdl_filename = f"{name}.spotdl"
    m3u_filename = f"{name}.m3u"

    # case on if we need to make an m3u
    try:
        if make_m3u:
            # toggle on m3u flag on and use m3u_filename
            subprocess.run(
                ["spotdl", "save", f"{url}", "--config", "--max-retries", f"{MAX_RETRIES}", "--threads", "f{THREADS}", "--format", "mp3", "--save-file", f"{spotdl_filename}", "--m3u", f"{m3u_filename}"], capture_output=True)
        else:
            # normal spotdl usage
            subprocess.run(
                ["spotdl", "save", f"{url}", "--config", "--max-retries", f"{MAX_RETRIES}", "--threads", "f{THREADS}", "--format", "mp3", "--save-file", f"{spotdl_filename}"], capture_output=True)
    except subprocess.CalledProcessError as e:
        raise RuntimeError(
            f"Error executing 'spotdl save {spotdl_filename} ...' command: {e}")

    # done so return
    return


def scan_for_spotdl(path):
    """
    Scan for all .spotdl files in specified directory.

    Args:
        path (str): Path to scan.

    Returns:
        list: A list of paths to all .spotdl files.

    Raises:
        FileNotFoundError: If the specified directory does not exist.
    """
    if not os.path.exists(path):
        raise FileNotFoundError(f"Directory '{path}' does not exist.")

    # storage
    matched = []

    # os walk on path
    for root, _, files in os.walk(path):
        for file in files:
            if file.endswith(".spotdl"):
                matched.append(os.path.join(root, file))

    return matched


def mapper(spotdl_files, config):
    """
    Map scanned spotdl files to the configuration file.

    Args:
        spotdl_files (list): List of paths to .spotdl files.
        config (dict): Parsed configuration data.

    Returns:
        dict: A mapping of .spotdl file paths to their corresponding type:
            ('artist' or 'playlist').

    Raises:
        ValueError: If a spotdl file does not match any configuration entry.
    """
    _mapping = {}
    mapping = {"artists": [], "playlists": []}

    # create lookup tables
    artist_table = {f"{artist}.spotdl": "artist" for artist,
                    _ in config["artists"]}
    playlist_table = {
        f"{playlist}.spotdl": "playlist" for playlist, _ in config["playlists"]}

    # combine tables
    valid_files = {**artist_table, **playlist_table}

    # map each spotdl file to its type
    for file_path in spotdl_files:
        file_name = os.path.basename(file_path)

        if file_name in valid_files:
            _mapping[file_path] = valid_files[file_name]
        else:
            raise ValueError(f"Unrecognized .spotdl file: {file_path}")

    for (file_path, key) in _mapping:
        if key == "artist":
            mapping["artists"].append(file_path)

        if key == "playlist":
            mapping["playlists"].append(file_name)

    return mapping


def download(spotdl_file):
    """
    Download using spotdl

    Args:
        spotdl_file (str): Path to spotdl file

    Raises:
        RuntimeError: If the shell subprocess encounters an error.
    """
    try:
        subprocess.run(["spotdl", "download", f"{spotdl_file}", "--config", "--bitrate", "128k", "--format", "mp3", "--max-retries",
                       f"{MAX_RETRIES}", "--threads", f"{THREADS}", "--overwrite", "metadata", "--restrict", "ascii", "--scan-for-songs", "--preload"], capture_output=True)
    except subprocess.CalledProcessError as e:
        raise RuntimeError(
            f"Error executing 'spotdl download {spotdl_file} ...': {e}")

    return


def main():
    """
    Main function to orchestrate the music download process.
    """
    # create argument parser
    parser = argparse.ArgumentParser(
        prog="download.py", description="Download music using a YAML configuration file.")

    # add config argument
    parser.add_argument("config", type=str,
                        help="Path to the YAML configuration file.")

    # parse arguments
    args = parser.parse_args()

    print("parsed arguments")

    # read and validate configuration file
    config_path = args.config
    try:
        config = read_config(config_path)
    except (FileNotFoundError, ValueError) as e:
        print(f"Error: {e}")

        return

    print("read and validated config file")

    # iterate through artists and create spotdl
    for dict in config['artists']:
        for item in dict:
            artist = item
            url = dict[item]

        print(f"create_spotdl(url={url}, name={artist}, make_m3u=False)")
        create_spotdl(url=url, name=artist, make_m3u=False)

    # iterate through playlists and create spotdl
    for dict in config['playlists']:
        for item in dict:
            playlist = item
            url = dict[item]

        print(f"create_spotdl(url={url}, name={playlist}, make_m3u=True)")
        create_spotdl(url=url, name=playlist, make_m3u=True)

    print("made all spotdl files from configuration")

    # scan current working directory for spotdl files
    cwd = os.getcwd()
    spotdl_files = scan_for_spotdl(path=cwd)

    print(f"scanned {cwd} for spotdl files")

    # run mapper to run download on found spotdl files
    spotdl_mapping = mapper(spotdl_files, config)

    print("mapped spotdl files to configuration")

    # download
    for file_path in spotdl_mapping["artists"]:
        print(f"download(spotdl_file={file_path})")
        download(spotdl_file=file_path)

    for file_path in spotdl_mapping["playlists"]:
        print(f"download(spotdl_file={file_path})")
        download(spotdl_file=file_path)

    print(":)")

    return


if __name__ == "__main__":
    main()
