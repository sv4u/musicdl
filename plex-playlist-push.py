#! /usr/bin/env python3
# plex-playlist-push.py

import base64
import http.client
import json
import os
import platform
import sys

from concurrent.futures import ThreadPoolExecutor, as_completed

from plexapi.exceptions import Unauthorized
from plexapi.server import PlexServer

# Get platform details for logging and headers
PLATFORM = platform.system()
PLATFORM_VERSION = platform.version()

# Fetch Plex credentials and server details from environment variables
username = os.getenv('PLEX_USERNAME')
password = os.getenv('PLEX_PASSWORD')
server = os.getenv('PLEX_SERVER')

# Log the start of the authentication process
print("Starting Plex.tv authentication...")

# Encode credentials for Plex.tv authentication
auth = ('%s:%s' % (username, password)).replace('\n', '').encode('ascii')
base64_auth = base64.b64encode(auth)
txdata = ''  # No additional data for the POST request
headers = {
    'Authorization': "Basic %s" % base64_auth.decode('ascii'),
    'X-Plex-Client-Identifier': "Plex Token",
    'X-Plex-Device-Name': "Plex Updater",
    'X-Plex-Product': "Plex Updater",
    'X-Plex-Platform': PLATFORM,
    'X-Plex-Platform-Version': PLATFORM_VERSION,
    'X-Plex-Version': "1.0"
}

# Connect to Plex.tv and authenticate
conn = http.client.HTTPSConnection("plex.tv")
conn.request("POST", "/users/sign_in.json", txdata, headers)
response = conn.getresponse()

# Parse the response to extract the authentication token
data = response.read()
json_data = json.loads(data)

# Extract the Plex token and server details
PLEX_TOKEN = json_data['user']['authToken']
PLEX_SERVER = server

# Log the successful authentication
print("Plex.tv authentication successful. Token retrieved.")

# Attempt to connect to the Plex server
try:
    plex = PlexServer(PLEX_SERVER, token=PLEX_TOKEN)
    print(f"Connected to Plex server: {PLEX_SERVER}")
except Unauthorized:
    print("Invalid Plex token. Please check your credentials.")
    sys.exit(1)

# Access the 'Music' library section
music_library = plex.library.section('Music')
print(f'Library section: "{music_library.title}" retrieved.')

# Initialize a list to store all tracks
all_tracks = []
albums = music_library.albums()

# Function to fetch tracks from an album
def fetch_tracks(album):
    try:
        print(f"Fetching tracks for album: {album.title}")
        return album.tracks()
    except Exception as e:
        print(f"Error fetching tracks for album '{album.title}': {e}")
        return []

# Use a ThreadPoolExecutor to fetch tracks concurrently
print("Fetching tracks from all albums...")
with ThreadPoolExecutor() as executor:
    futures = {executor.submit(fetch_tracks, album): album for album in albums}

    for future in as_completed(futures):
        album = futures[future]
        try:
            tracks = future.result()
            all_tracks.extend(tracks)
            print(f"Tracks fetched for album: {album.title}")
        except Exception as e:
            print(f"Error processing album '{album.title}': {e}")

# Map to store media file paths and their corresponding tracks
media_map = {}

# Process all tracks and build the media map
print("Processing tracks and building media map...")
for idx, track in enumerate(all_tracks, start=1):
    if hasattr(track, 'media'):
        for media in track.media:
            title = media.title

            for part in media.parts:
                # Normalize the file path
                full_path = os.path.normpath(part.file)
                media_map[full_path] = track

    # Log progress every 100 tracks
    if idx % 100 == 0:
        print(f"Processed {idx} tracks...")

# Scan the current directory for .m3u8 playlist files
print("Scanning for .m3u8 playlist files...")
playlist_files = []
for root, _, files in os.walk('.'):
    for file in files:
        if file.endswith('.m3u8'):
            playlist_files.append(os.path.join(root, file))

print(f"Found {len(playlist_files)} playlist files.")

# Process each playlist file
for playlist in playlist_files:
    playlist_title = os.path.basename(playlist).replace('.m3u8', '')
    print(f"Processing playlist: {playlist_title}")

    # Read the playlist file and extract song paths
    with open(playlist, 'r') as f:
        song_paths = [line.strip() for line in f if not line.startswith('#') and line.strip()]

    matched_tracks = []
    for _song_path in song_paths:
        # Convert the relative playlist path to an absolute path
        song_path = os.path.join("/mnt/Storage/Music", _song_path)
        if song_path in media_map:
            track = media_map[song_path]
            matched_tracks.append(track)
        else:
            print(f"Track not found in Plex: {song_path}")

    # Create or update the playlist in Plex
    try:
        existing_playlist = next((pl for pl in plex.playlists() if pl.title == playlist_title), None)

        if existing_playlist:
            print(f"Updating existing playlist: {playlist_title}")
            existing_track_ids = {track.ratingKey for track in existing_playlist.items()}
            new_tracks = [track for track in matched_tracks if track.ratingKey not in existing_track_ids]

            if new_tracks:
                existing_playlist.addItems(new_tracks)
                print(f"Added {len(new_tracks)} new tracks to playlist: {playlist_title}")
        else:
            print(f"Creating new playlist: {playlist_title}")
            plex.createPlaylist(playlist_title, items=matched_tracks)
            print(f"Playlist '{playlist_title}' created successfully.")
    except Exception as e:
        print(f"Error creating/updating playlist '{playlist_title}': {e}")

