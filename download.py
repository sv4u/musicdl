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
# passed to spotdl which downloads the music.
#
# The `threads` and `retires` items are optional and must be integers. The
# `version` object is required and must be `1.0` currently.
#
# COMMAND LINE ARGUMENT
#
# [CONFIG]      musicdl YAML configuration file containing information on
#               artists and playlists to download
#
# NOTES
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

    if 'retries' in config and not isinstance(config['retries'], int):
        raise ValueError("'retries' must be an integer.")

    # return config now that validation is done
    return config


def download(url, make_m3u=False, name="", threads=THREADS, retries=MAX_RETRIES):
    """
    Download using spotdl

    Args:
        url (str): Spotify url
        make_m3u (bool): Toggle for creating playlist [default: False]
        name (str): Name of playlist [default: ""]

    Raises:
        RuntimeError: If the shell subprocess encounters an error.
    """
    if threads is None:
        threads = THREADS

    if retries is None:
        retries = MAX_RETRIES

    try:
        if make_m3u:
            result = subprocess.run(["spotdl", "--simple-tui", "--log-level", "INFO", "--config", "--bitrate", "128k", "--format", "mp3",  "--m3u", name, "--max-retries", str(retries), "--threads", str(
                threads), "--overwrite", "metadata", "--restrict", "ascii", "--scan-for-songs", "--create-skip-file", "--respect-skip-file", "download", url], capture_output=True)
        else:
            result = subprocess.run(["spotdl", "--simple-tui", "--log-level", "INFO", "--config", "--bitrate", "128k", "--format", "mp3", "--max-retries", str(retries), "--threads", str(
                threads), "--overwrite", "metadata", "--restrict", "ascii", "--scan-for-songs", "--create-skip-file", "--respect-skip-file", "download", url], capture_output=True)

        if result.returncode != 0:
            print(f"return code: {result.returncode}")
            print(f"stdout: {result.stdout.decode()}")
            print(f"stderr: {result.stderr.decode()}")

    except subprocess.CalledProcessError as e:
        raise RuntimeError(
            f"Error executing 'spotdl download {url} {make_m3u} {name} ...': {e}")

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

    print(f"threads: {config['threads']}")
    print(f"retries: {config['retries']}")

    # iterate through artists and create spotdl
    for dict in config['artists']:
        for item in dict:
            artist = item
            url = dict[item]

        print(f"download(url={url}, make_m3u=False, name={artist})")
        download(url, False, name=artist,
                 threads=config['threads'], retries=config['retries'])

    # iterate through playlists and create spotdl
    for dict in config['playlists']:
        for item in dict:
            playlist = item
            url = dict[item]

        print(f"download(url={url}, make_m3u=True, playlist={playlist})")
        download(url, True, name=playlist,
                 threads=config['threads'], retries=config['retries'])

    print("downloaded all items from configuration file")

    print(":)")

    return


if __name__ == "__main__":
    main()
