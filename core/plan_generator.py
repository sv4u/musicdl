"""
Plan generator for converting configuration to download plan.

This module generates a comprehensive download plan from the configuration,
creating plan items for all tracks, albums, artists, and playlists.
"""

import logging
import re
from typing import Dict, List, Set

from core.config import MusicDLConfig, MusicSource
from core.plan import DownloadPlan, PlanItem, PlanItemType
from core.spotify_client import SpotifyClient, extract_id_from_url

logger = logging.getLogger(__name__)


class PlanGenerator:
    """
    Generates download plan from configuration.

    Processes songs, artists, and playlists to create a comprehensive plan
    with proper hierarchy and duplicate tracking.
    """

    def __init__(self, config: MusicDLConfig, spotify_client: SpotifyClient):
        """
        Initialize plan generator.

        Args:
            config: MusicDLConfig instance
            spotify_client: SpotifyClient instance for API calls
        """
        self.config = config
        self.spotify = spotify_client
        self.seen_track_ids: Set[str] = set()  # Track duplicates by Spotify ID
        self.seen_album_ids: Set[str] = set()
        self.seen_playlist_ids: Set[str] = set()
        self.seen_artist_ids: Set[str] = set()

    def generate_plan(self) -> DownloadPlan:
        """
        Generate complete download plan from configuration.

        Returns:
            DownloadPlan with all items
        """
        logger.info("Generating download plan from configuration...")
        plan = DownloadPlan(metadata={"config_version": self.config.version})

        # Process songs
        logger.info(f"Processing {len(self.config.songs)} songs...")
        for song in self.config.songs:
            self._process_song(plan, song)

        # Process artists
        logger.info(f"Processing {len(self.config.artists)} artists...")
        for artist in self.config.artists:
            self._process_artist(plan, artist)

        # Process playlists
        logger.info(f"Processing {len(self.config.playlists)} playlists...")
        for playlist in self.config.playlists:
            self._process_playlist(plan, playlist)

        # Process albums
        logger.info(f"Processing {len(self.config.albums)} albums...")
        for album in self.config.albums:
            self._process_album(plan, album)

        stats = plan.get_statistics()
        logger.info(
            f"Plan generation complete: {stats['total_items']} items "
            f"({stats['by_type']['track']} tracks, "
            f"{stats['by_type']['album']} albums, "
            f"{stats['by_type']['artist']} artists, "
            f"{stats['by_type']['playlist']} playlists)"
        )

        return plan

    def _process_song(self, plan: DownloadPlan, song: MusicSource) -> None:
        """
        Process a single song and add to plan.

        Args:
            plan: DownloadPlan to add items to
            song: MusicSource for the song
        """
        try:
            track_id = extract_id_from_url(song.url)
            track_id = self._extract_track_id(track_id)
            
            # Validate track_id is not empty
            if not track_id or not track_id.strip():
                raise ValueError(f"Invalid or empty track ID extracted from URL: {song.url}")

            # Check for duplicates
            if track_id in self.seen_track_ids:
                logger.debug(f"Skipping duplicate track: {song.name} ({track_id})")
                return

            # Fetch track metadata
            track_data = self.spotify.get_track(track_id)
            track_name = track_data.get("name", song.name)

            # Create track item
            item = PlanItem(
                item_id=f"track:{track_id}",
                item_type=PlanItemType.TRACK,
                spotify_id=track_id,
                spotify_url=track_data.get("external_urls", {}).get("spotify", song.url),
                name=track_name,
                metadata={
                    "source_name": song.name,
                    "source_url": song.url,
                    "artist": track_data.get("artists", [{}])[0].get("name", ""),
                },
            )

            plan.items.append(item)
            self.seen_track_ids.add(track_id)
            logger.debug(f"Added track to plan: {track_name}")

        except Exception as e:
            logger.error(f"Error processing song {song.name}: {e}")
            # Create failed item for tracking
            item = PlanItem(
                item_id=f"track:error:{song.name}",
                item_type=PlanItemType.TRACK,
                name=song.name,
                metadata={"source_url": song.url, "error": str(e)},
            )
            item.mark_failed(str(e))
            plan.items.append(item)

    def _process_artist(self, plan: DownloadPlan, artist: MusicSource) -> None:
        """
        Process an artist and add albums/tracks to plan.

        Args:
            plan: DownloadPlan to add items to
            artist: MusicSource for the artist
        """
        try:
            artist_id = extract_id_from_url(artist.url)
            artist_id = self._extract_artist_id(artist_id)

            # Check for duplicates
            if artist_id in self.seen_artist_ids:
                logger.debug(f"Skipping duplicate artist: {artist.name} ({artist_id})")
                return

            # Fetch artist metadata
            artist_data = self.spotify.get_artist(artist_id)
            artist_name = artist_data.get("name", artist.name)

            # Create artist item
            artist_item = PlanItem(
                item_id=f"artist:{artist_id}",
                item_type=PlanItemType.ARTIST,
                spotify_id=artist_id,
                spotify_url=artist_data.get("external_urls", {}).get("spotify", artist.url),
                name=artist_name,
                metadata={
                    "source_name": artist.name,
                    "source_url": artist.url,
                },
            )
            plan.items.append(artist_item)
            self.seen_artist_ids.add(artist_id)

            # Get artist albums (discography only - albums and singles)
            albums = self.spotify.get_artist_albums(artist_id)
            logger.info(f"Found {len(albums)} albums for artist: {artist_name}")

            for album_data in albums:
                album_id = album_data.get("id")
                if not album_id:
                    continue

                # Check for duplicate albums
                if album_id in self.seen_album_ids:
                    logger.debug(f"Skipping duplicate album: {album_data.get('name')}")
                    # Still add reference to parent's child_ids for proper tracking
                    existing_album_item_id = f"album:{album_id}"
                    # Find existing album item
                    existing_album = plan.get_item(existing_album_item_id)
                    if existing_album:
                        artist_item.child_ids.append(existing_album_item_id)
                    continue

                # Create album item
                album_item = PlanItem(
                    item_id=f"album:{album_id}",
                    item_type=PlanItemType.ALBUM,
                    spotify_id=album_id,
                    spotify_url=album_data.get("external_urls", {}).get("spotify"),
                    parent_id=artist_item.item_id,
                    name=album_data.get("name", ""),
                    metadata={
                        "album_type": album_data.get("album_type"),
                        "release_date": album_data.get("release_date"),
                    },
                )
                plan.items.append(album_item)
                artist_item.child_ids.append(album_item.item_id)
                self.seen_album_ids.add(album_id)

                # Process album tracks
                self._process_album_tracks(plan, album_item, album_id)

        except Exception as e:
            logger.error(f"Error processing artist {artist.name}: {e}")
            # Create failed item
            item = PlanItem(
                item_id=f"artist:error:{artist.name}",
                item_type=PlanItemType.ARTIST,
                name=artist.name,
                metadata={"source_url": artist.url, "error": str(e)},
            )
            item.mark_failed(str(e))
            plan.items.append(item)

    def _process_album_tracks(self, plan: DownloadPlan, album_item: PlanItem, album_id: str) -> None:
        """
        Process tracks in an album and add to plan.

        Args:
            plan: DownloadPlan to add items to
            album_item: Parent album item
            album_id: Spotify album ID
        """
        try:
            album_data = self.spotify.get_album(album_id)
            tracks_obj = album_data.get("tracks", {})
            items = tracks_obj.get("items", [])

            # Handle pagination with rate limiting
            while tracks_obj.get("next"):
                tracks_obj = self.spotify._next_with_rate_limit(tracks_obj)
                items.extend(tracks_obj.get("items", []))

            logger.debug(f"Found {len(items)} tracks in album: {album_data.get('name')}")

            for track_item in items:
                track_id = track_item.get("id")
                if not track_id:
                    continue

                # Check for duplicate tracks
                if track_id in self.seen_track_ids:
                    logger.debug(f"Skipping duplicate track: {track_item.get('name')}")
                    # Still add reference to parent's child_ids for M3U generation
                    existing_track_item_id = f"track:{track_id}"
                    # Find existing track item
                    existing_track = plan.get_item(existing_track_item_id)
                    if existing_track:
                        album_item.child_ids.append(existing_track_item_id)
                    continue

                # Create track item
                track_plan_item = PlanItem(
                    item_id=f"track:{track_id}",
                    item_type=PlanItemType.TRACK,
                    spotify_id=track_id,
                    spotify_url=track_item.get("external_urls", {}).get("spotify"),
                    parent_id=album_item.item_id,
                    name=track_item.get("name", ""),
                    metadata={
                        "track_number": track_item.get("track_number"),
                        "disc_number": track_item.get("disc_number"),
                    },
                )
                plan.items.append(track_plan_item)
                album_item.child_ids.append(track_plan_item.item_id)
                self.seen_track_ids.add(track_id)

        except Exception as e:
            logger.error(f"Error processing album tracks for {album_id}: {e}")

    def _process_playlist(self, plan: DownloadPlan, playlist: MusicSource) -> None:
        """
        Process a playlist and add tracks/M3U to plan.

        Args:
            plan: DownloadPlan to add items to
            playlist: MusicSource for the playlist
        """
        try:
            playlist_id = extract_id_from_url(playlist.url)
            playlist_id = self._extract_playlist_id(playlist_id)

            # Check for duplicates
            if playlist_id in self.seen_playlist_ids:
                logger.debug(f"Skipping duplicate playlist: {playlist.name} ({playlist_id})")
                return

            # Fetch playlist metadata
            playlist_data = self.spotify.get_playlist(playlist_id)
            playlist_name = playlist_data.get("name", playlist.name)

            # Create playlist item
            playlist_item = PlanItem(
                item_id=f"playlist:{playlist_id}",
                item_type=PlanItemType.PLAYLIST,
                spotify_id=playlist_id,
                spotify_url=playlist_data.get("external_urls", {}).get("spotify", playlist.url),
                name=playlist_name,
                metadata={
                    "source_name": playlist.name,
                    "source_url": playlist.url,
                    "description": playlist_data.get("description"),
                },
            )
            plan.items.append(playlist_item)
            self.seen_playlist_ids.add(playlist_id)

            # Get playlist tracks
            tracks_obj = playlist_data.get("tracks", {})
            items = tracks_obj.get("items", [])

            # Handle pagination with rate limiting
            while tracks_obj.get("next"):
                tracks_obj = self.spotify._next_with_rate_limit(tracks_obj)
                items.extend(tracks_obj.get("items", []))

            logger.info(f"Found {len(items)} tracks in playlist: {playlist_name}")

            # Process tracks
            for track_item in items:
                track = track_item.get("track")
                if not track or track.get("is_local"):
                    continue

                track_id = track.get("id")
                if not track_id:
                    continue

                # Check for duplicate tracks
                if track_id in self.seen_track_ids:
                    logger.debug(f"Skipping duplicate track: {track.get('name')}")
                    # Still add reference to parent's child_ids for M3U generation
                    existing_track_item_id = f"track:{track_id}"
                    # Find existing track item
                    existing_track = plan.get_item(existing_track_item_id)
                    if existing_track:
                        playlist_item.child_ids.append(existing_track_item_id)
                    continue

                # Create track item
                track_plan_item = PlanItem(
                    item_id=f"track:{track_id}",
                    item_type=PlanItemType.TRACK,
                    spotify_id=track_id,
                    spotify_url=track.get("external_urls", {}).get("spotify"),
                    parent_id=playlist_item.item_id,
                    name=track.get("name", ""),
                    metadata={
                        "added_at": track_item.get("added_at"),
                    },
                )
                plan.items.append(track_plan_item)
                playlist_item.child_ids.append(track_plan_item.item_id)
                self.seen_track_ids.add(track_id)

            # Create M3U item (child of playlist)
            m3u_item = PlanItem(
                item_id=f"m3u:{playlist_id}",
                item_type=PlanItemType.M3U,
                parent_id=playlist_item.item_id,
                name=f"{playlist_name}.m3u",
                metadata={
                    "playlist_name": playlist_name,
                },
            )
            plan.items.append(m3u_item)
            playlist_item.child_ids.append(m3u_item.item_id)

        except Exception as e:
            logger.error(f"Error processing playlist {playlist.name}: {e}")
            # Create failed item
            item = PlanItem(
                item_id=f"playlist:error:{playlist.name}",
                item_type=PlanItemType.PLAYLIST,
                name=playlist.name,
                metadata={"source_url": playlist.url, "error": str(e)},
            )
            item.mark_failed(str(e))
            plan.items.append(item)

    def _process_album(self, plan: DownloadPlan, album: MusicSource) -> None:
        """
        Process an album and add tracks/M3U to plan.

        Args:
            plan: DownloadPlan to add items to
            album: MusicSource for the album
        """
        try:
            album_id = extract_id_from_url(album.url)
            album_id = self._extract_album_id(album_id)

            # Check for duplicates
            if album_id in self.seen_album_ids:
                # Album already exists (likely from artist processing)
                # Check if user requested M3U creation for this explicit album entry
                if album.create_m3u:
                    logger.info(
                        f"Album {album.name} already exists, but honoring create_m3u=True request"
                    )
                    # Find existing album item
                    existing_album_item_id = f"album:{album_id}"
                    existing_album_item = plan.get_item(existing_album_item_id)
                    if existing_album_item:
                        # Update metadata to indicate M3U should be created
                        existing_album_item.metadata["create_m3u"] = True
                        # Check if M3U item already exists
                        m3u_item_id = f"m3u:album:{album_id}"
                        existing_m3u_item = plan.get_item(m3u_item_id)
                        if not existing_m3u_item:
                            # Create M3U item if it doesn't exist
                            album_name = existing_album_item.name
                            m3u_item = PlanItem(
                                item_id=m3u_item_id,
                                item_type=PlanItemType.M3U,
                                parent_id=existing_album_item.item_id,
                                name=f"{album_name}.m3u",
                                metadata={
                                    "album_name": album_name,
                                },
                            )
                            plan.items.append(m3u_item)
                            existing_album_item.child_ids.append(m3u_item.item_id)
                            logger.debug(f"Added M3U item for duplicate album: {album.name}")
                else:
                    logger.debug(f"Skipping duplicate album: {album.name} ({album_id})")
                return

            # Fetch album metadata
            album_data = self.spotify.get_album(album_id)
            album_name = album_data.get("name", album.name)

            # Create album item
            album_item = PlanItem(
                item_id=f"album:{album_id}",
                item_type=PlanItemType.ALBUM,
                spotify_id=album_id,
                spotify_url=album_data.get("external_urls", {}).get("spotify", album.url),
                name=album_name,
                metadata={
                    "source_name": album.name,
                    "source_url": album.url,
                    "create_m3u": album.create_m3u,  # Store M3U flag in metadata
                    "album_type": album_data.get("album_type"),
                    "release_date": album_data.get("release_date"),
                },
            )
            plan.items.append(album_item)
            self.seen_album_ids.add(album_id)

            # Process album tracks
            self._process_album_tracks(plan, album_item, album_id)

            # Create M3U item only if requested (unlike playlists which always create M3U)
            if album.create_m3u:
                m3u_item = PlanItem(
                    item_id=f"m3u:album:{album_id}",
                    item_type=PlanItemType.M3U,
                    parent_id=album_item.item_id,
                    name=f"{album_name}.m3u",
                    metadata={
                        "album_name": album_name,
                    },
                )
                plan.items.append(m3u_item)
                album_item.child_ids.append(m3u_item.item_id)

        except Exception as e:
            logger.error(f"Error processing album {album.name}: {e}")
            # Create failed item
            item = PlanItem(
                item_id=f"album:error:{album.name}",
                item_type=PlanItemType.ALBUM,
                name=album.name,
                metadata={"source_url": album.url, "error": str(e)},
            )
            item.mark_failed(str(e))
            plan.items.append(item)

    def _extract_track_id(self, url_or_id: str) -> str:
        """
        Extract track ID from URL or return as-is if already an ID.

        Args:
            url_or_id: Spotify URL or ID

        Returns:
            Track ID
        """
        match = re.search(r"track/([a-zA-Z0-9]+)", url_or_id)
        return match.group(1) if match else url_or_id

    def _extract_artist_id(self, url_or_id: str) -> str:
        """
        Extract artist ID from URL or return as-is if already an ID.

        Args:
            url_or_id: Spotify URL or ID

        Returns:
            Artist ID
        """
        match = re.search(r"artist/([a-zA-Z0-9]+)", url_or_id)
        return match.group(1) if match else url_or_id

    def _extract_playlist_id(self, url_or_id: str) -> str:
        """
        Extract playlist ID from URL or return as-is if already an ID.

        Args:
            url_or_id: Spotify URL or ID

        Returns:
            Playlist ID
        """
        match = re.search(r"playlist/([a-zA-Z0-9]+)", url_or_id)
        return match.group(1) if match else url_or_id

    def _extract_album_id(self, url_or_id: str) -> str:
        """
        Extract album ID from URL or return as-is if already an ID.

        Args:
            url_or_id: Spotify URL or ID

        Returns:
            Album ID
        """
        match = re.search(r"album/([a-zA-Z0-9]+)", url_or_id)
        return match.group(1) if match else url_or_id

