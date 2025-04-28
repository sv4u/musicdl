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

PLATFORM = platform.system()
PLATFORM_VERSION = platform.version()

username = os.getenv('PLEX_USERNAME')
password = os.getenv('PLEX_PASSWORD')
server = os.getenv('PLEX_SERVER')

# plex.tv auth
auth = ('%s:%s' % (username, password)).replace('\n', '').encode('ascii')
base64_auth = base64.b64encode(auth)
txdata = ''
headers={'Authorization': "Basic %s" % base64_auth.decode('ascii'),
                'X-Plex-Client-Identifier': "Plex Token",
                'X-Plex-Device-Name': "Plex Updater",
                'X-Plex-Product': "Plex Updater",
                'X-Plex-Platform': PLATFORM,
                'X-Plex-Platform-Version': PLATFORM_VERSION,
                'X-Plex-Version': "1.0"}

conn = http.client.HTTPSConnection("plex.tv")
conn.request("POST","/users/sign_in.json",txdata,headers)
response = conn.getresponse()

data = response.read()
json_data = json.loads(data)

PLEX_TOKEN = json_data['user']['authToken']
PLEX_SERVER = server

try:
    plex = PlexServer(PLEX_SERVER, token=PLEX_TOKEN)
except Unauthorized:
    print("Invalid Plex token. Please check your credentials.")
    sys.exit(1)

music_library = plex.library.section('Music')
print(f'Library section: "{music_library.title}" retrieved.')

all_tracks = []
albums = music_library.albums()

def fetch_tracks(album):
    try:
        return album.tracks()
    except Exception as e:
        print(f"Error fetching tracks for album '{album.title}': {e}")
        return []

with ThreadPoolExecutor() as executor:
    futures = { executor.submit(fetch_tracks, album): album for album in albums }

    for future in as_completed(futures):
        album = futures[future]
        try:
            tracks = future.result()
            all_tracks.extend(tracks)
        except Exception as e:
            print(f"Error processing album '{album.title}': {e}")

media_map = {}

for idx, track in enumerate(all_tracks, start=1):
    if hasattr(track, 'media'):
        for media in track.media:
            title = media.title

            for part in media.parts:
                # full_path will be a path rooted at /mnt/Storage/Music/...
                full_path = os.path.normpath(part.file)
                media_map[full_path] = track

    if idx % 100 == 0:
        print(f"Processed {idx} tracks...")


playlist_files = []
for root, _, files in os.walk('.'):
    for file in files:
        if file.endswith('.m3u8'):
            playlist_files.append(os.path.join(root, file))

for playlist in playlist_files:
    playlist_title = os.path.basename(playlist).replace('.m3u8', '')

    with open(playlist, 'r') as f:
        song_paths = [line.strip() for line in f if not line.startswith('#') and line.strip()]

    matched_tracks = []
    for _song_path in song_paths:
        # the playlist path is relative to the root of the music library
        song_path = os.path.join("/mnt/Storage/Music", _song_path)
        if song_path in media_map:
            track = media_map[song_path]
            matched_tracks.append(track)
        else:
            print(f"Track not found in Plex: {song_path}")

    try:
        existing_playlist = next((pl for pl in plex.playlists() if pl.title == playlist_title), None)

        if existing_playlist:
            existing_track_ids = {track.ratingKey for item in existing_playlist.items()}
            new_tracks = [track for track in matched_tracks if track.ratingKey not in existing_track_ids]

            if new_tracks:
                existing_playlist.addItems(new_tracks)
        else:
            plex.createPlaylist(playlist_title, items=matched_tracks)
    except Exception as e:
        print(f"Error creating/updating playlist '{playlist_title}': {e}")

