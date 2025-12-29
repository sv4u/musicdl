"""
Sample Spotify API responses for testing.
"""
SAMPLE_TRACK_RESPONSE = {
    "id": "1RKbVxcm267VdsIzqY7msi",
    "name": "YYZ",
    "artists": [
        {"id": "artist_id", "name": "Rush"}
    ],
    "album": {
        "id": "77CZUF57sYqgtznUe3OikQ",
        "name": "Moving Pictures (40th Anniversary Super Deluxe)",
        "images": [
            {"url": "https://i.scdn.co/image/...", "width": 640, "height": 640}
        ],
        "release_date": "2022-01-01",
        "artists": [{"name": "Rush"}],
        "total_tracks": 10,
    },
    "duration_ms": 266000,
    "track_number": 3,
    "disc_number": 1,
    "external_urls": {
        "spotify": "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"
    },
    "external_ids": {"isrc": "USRC12345678"},
    "explicit": False,
}

# Add more sample responses as needed

