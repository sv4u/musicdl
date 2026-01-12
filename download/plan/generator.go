package plan

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/spotigo"
)

// Generator generates download plans from configuration.
type Generator struct {
	config        *config.MusicDLConfig
	spotifyClient interface {
		GetTrack(ctx context.Context, trackIDOrURL string) (*spotigo.Track, error)
		GetAlbum(ctx context.Context, albumIDOrURL string) (*spotigo.Album, error)
		GetArtist(ctx context.Context, artistIDOrURL string) (*spotigo.Artist, error)
		GetPlaylist(ctx context.Context, playlistIDOrURL string) (*spotigo.Playlist, error)
		GetArtistAlbums(ctx context.Context, artistIDOrURL string) ([]spotigo.SimplifiedAlbum, error)
		NextWithRateLimit(ctx context.Context, paging interface{ GetNext() *string }) (*spotigo.Paging[spotigo.SimplifiedAlbum], error)
		NextAlbumTracks(ctx context.Context, paging interface{ GetNext() *string }) (*spotigo.Paging[spotigo.SimplifiedTrack], error)
		NextPlaylistTracks(ctx context.Context, paging interface{ GetNext() *string }) (*spotigo.Paging[spotigo.PlaylistTrack], error)
	}
	// For playlist tracks, we need direct access to the spotigo client
	playlistTracksFunc func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error)
	seenTrackIDs    map[string]bool
	seenAlbumIDs    map[string]bool
	seenPlaylistIDs map[string]bool
	seenArtistIDs   map[string]bool
}

// SpotifyClientInterface defines the interface for Spotify client operations.
type SpotifyClientInterface interface {
	GetTrack(ctx context.Context, trackIDOrURL string) (*spotigo.Track, error)
	GetAlbum(ctx context.Context, albumIDOrURL string) (*spotigo.Album, error)
	GetArtist(ctx context.Context, artistIDOrURL string) (*spotigo.Artist, error)
	GetPlaylist(ctx context.Context, playlistIDOrURL string) (*spotigo.Playlist, error)
	GetArtistAlbums(ctx context.Context, artistIDOrURL string) ([]spotigo.SimplifiedAlbum, error)
	NextWithRateLimit(ctx context.Context, paging interface{ GetNext() *string }) (*spotigo.Paging[spotigo.SimplifiedAlbum], error)
	NextAlbumTracks(ctx context.Context, paging interface{ GetNext() *string }) (*spotigo.Paging[spotigo.SimplifiedTrack], error)
	NextPlaylistTracks(ctx context.Context, paging interface{ GetNext() *string }) (*spotigo.Paging[spotigo.PlaylistTrack], error)
}

// NewGenerator creates a new plan generator.
func NewGenerator(cfg *config.MusicDLConfig, spotifyClient SpotifyClientInterface, playlistTracksFunc func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error)) *Generator {
	return &Generator{
		config:            cfg,
		spotifyClient:     spotifyClient,
		playlistTracksFunc: playlistTracksFunc,
		seenTrackIDs:      make(map[string]bool),
		seenAlbumIDs:      make(map[string]bool),
		seenPlaylistIDs:   make(map[string]bool),
		seenArtistIDs:     make(map[string]bool),
	}
}

// GeneratePlan generates a complete download plan from configuration.
func (g *Generator) GeneratePlan(ctx context.Context) (*DownloadPlan, error) {
	plan := NewDownloadPlan(map[string]interface{}{
		"config_version": g.config.Version,
	})

	// Process songs
	songCount := 0
	for _, song := range g.config.Songs {
		if err := g.processSong(ctx, plan, song); err != nil {
			log.Printf("ERROR: process_song_failed url=%s name=%s error=%v", song.URL, song.Name, err)
		} else {
			songCount++
		}
	}
	if len(g.config.Songs) > 0 {
		log.Printf("INFO: processed_songs total=%d successful=%d", len(g.config.Songs), songCount)
	}

	// Process artists
	artistCount := 0
	for _, artist := range g.config.Artists {
		if err := g.processArtist(ctx, plan, artist); err != nil {
			log.Printf("ERROR: process_artist_failed url=%s name=%s error=%v", artist.URL, artist.Name, err)
		} else {
			artistCount++
		}
	}
	if len(g.config.Artists) > 0 {
		log.Printf("INFO: processed_artists total=%d successful=%d", len(g.config.Artists), artistCount)
	}

	// Process playlists
	playlistCount := 0
	for _, playlist := range g.config.Playlists {
		if err := g.processPlaylist(ctx, plan, playlist); err != nil {
			log.Printf("ERROR: process_playlist_failed url=%s name=%s error=%v", playlist.URL, playlist.Name, err)
		} else {
			playlistCount++
		}
	}
	if len(g.config.Playlists) > 0 {
		log.Printf("INFO: processed_playlists total=%d successful=%d", len(g.config.Playlists), playlistCount)
	}

	// Process albums
	albumCount := 0
	for _, album := range g.config.Albums {
		if err := g.processAlbum(ctx, plan, album); err != nil {
			log.Printf("ERROR: process_album_failed url=%s name=%s error=%v", album.URL, album.Name, err)
		} else {
			albumCount++
		}
	}
	if len(g.config.Albums) > 0 {
		log.Printf("INFO: processed_albums total=%d successful=%d", len(g.config.Albums), albumCount)
	}

	log.Printf("INFO: plan_generation_progress items=%d", len(plan.Items))

	return plan, nil
}

// processSong processes a single song and adds it to the plan.
func (g *Generator) processSong(ctx context.Context, plan *DownloadPlan, song config.MusicSource) error {
	trackID := extractTrackID(song.URL)
	if trackID == "" {
		return fmt.Errorf("invalid or empty track ID extracted from URL: %s", song.URL)
	}

	// Check for duplicates
	if g.seenTrackIDs[trackID] {
		log.Printf("INFO: duplicate_detected type=track spotify_id=%s url=%s", trackID, song.URL)
		return nil // Skip duplicate
	}

	// Fetch track metadata
	track, err := g.spotifyClient.GetTrack(ctx, trackID)
	if err != nil {
		log.Printf("ERROR: api_call_failed type=track spotify_id=%s error=%v", trackID, err)
		// Create failed item
		item := &PlanItem{
			ItemID:   fmt.Sprintf("track:error:%s", song.Name),
			ItemType: PlanItemTypeTrack,
			Name:     song.Name,
			Status:   PlanItemStatusFailed,
			Error:    err.Error(),
			Metadata: map[string]interface{}{
				"source_url": song.URL,
				"error":      err.Error(),
			},
		}
		plan.AddItem(item)
		return err
	}

	// Create track item
	trackName := track.Name
	if trackName == "" {
		trackName = song.Name
	}

	spotifyURL := ""
	if track.ExternalURLs != nil {
		spotifyURL = track.ExternalURLs.Spotify
	}
	if spotifyURL == "" {
		spotifyURL = song.URL
	}

	item := &PlanItem{
		ItemID:     fmt.Sprintf("track:%s", trackID),
		ItemType:   PlanItemTypeTrack,
		SpotifyID:  trackID,
		SpotifyURL: spotifyURL,
		Name:       trackName,
		Status:     PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"source_name": song.Name,
			"source_url":  song.URL,
		},
	}

	if len(track.Artists) > 0 {
		item.Metadata["artist"] = track.Artists[0].Name
	}

	plan.AddItem(item)
	g.seenTrackIDs[trackID] = true
	return nil
}

// extractTrackID extracts track ID from URL or returns as-is if already an ID.
func extractTrackID(urlOrID string) string {
	re := regexp.MustCompile(`track/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(urlOrID)
	if len(matches) > 1 {
		return matches[1]
	}
	return urlOrID
}

// extractArtistID extracts artist ID from URL or returns as-is if already an ID.
func extractArtistID(urlOrID string) string {
	re := regexp.MustCompile(`artist/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(urlOrID)
	if len(matches) > 1 {
		return matches[1]
	}
	return urlOrID
}

// extractPlaylistID extracts playlist ID from URL or returns as-is if already an ID.
func extractPlaylistID(urlOrID string) string {
	re := regexp.MustCompile(`playlist/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(urlOrID)
	if len(matches) > 1 {
		return matches[1]
	}
	return urlOrID
}

// extractAlbumID extracts album ID from URL or returns as-is if already an ID.
func extractAlbumID(urlOrID string) string {
	re := regexp.MustCompile(`album/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(urlOrID)
	if len(matches) > 1 {
		return matches[1]
	}
	return urlOrID
}

// processArtist processes an artist and adds albums/tracks to plan.
func (g *Generator) processArtist(ctx context.Context, plan *DownloadPlan, artist config.MusicSource) error {
	artistID := extractArtistID(artist.URL)

	// Check for duplicates
	if g.seenArtistIDs[artistID] {
		log.Printf("INFO: duplicate_detected type=artist spotify_id=%s url=%s", artistID, artist.URL)
		return nil // Skip duplicate
	}

	// Fetch artist metadata
	artistData, err := g.spotifyClient.GetArtist(ctx, artistID)
	if err != nil {
		// Create failed item
		item := &PlanItem{
			ItemID:   fmt.Sprintf("artist:error:%s", artist.Name),
			ItemType: PlanItemTypeArtist,
			Name:     artist.Name,
			Status:   PlanItemStatusFailed,
			Error:    err.Error(),
			Metadata: map[string]interface{}{
				"source_url": artist.URL,
				"error":      err.Error(),
			},
		}
		plan.AddItem(item)
		return err
	}

	artistName := artistData.Name
	if artistName == "" {
		artistName = artist.Name
	}

	spotifyURL := ""
	if artistData.ExternalURLs != nil {
		spotifyURL = artistData.ExternalURLs.Spotify
	}
	if spotifyURL == "" {
		spotifyURL = artist.URL
	}

	// Create artist item
	artistItem := &PlanItem{
		ItemID:     fmt.Sprintf("artist:%s", artistID),
		ItemType:   PlanItemTypeArtist,
		SpotifyID:  artistID,
		SpotifyURL: spotifyURL,
		Name:       artistName,
		Status:     PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"source_name": artist.Name,
			"source_url":  artist.URL,
		},
	}
	plan.AddItem(artistItem)
	g.seenArtistIDs[artistID] = true

	// Get artist albums
	albums, err := g.spotifyClient.GetArtistAlbums(ctx, artistID)
	if err != nil {
		return fmt.Errorf("failed to get artist albums: %w", err)
	}

	// Process each album
	for _, albumData := range albums {
		albumID := albumData.ID
		if albumID == "" {
			continue
		}

		// Check for duplicate albums
		if g.seenAlbumIDs[albumID] {
			log.Printf("INFO: duplicate_detected type=album spotify_id=%s album_name=%s", albumID, albumData.Name)
			// Still add reference to parent's child_ids
			existingAlbumItemID := fmt.Sprintf("album:%s", albumID)
			existingAlbum := plan.GetItem(existingAlbumItemID)
			if existingAlbum != nil {
				artistItem.ChildIDs = append(artistItem.ChildIDs, existingAlbumItemID)
			}
			continue
		}

		// Process album tracks
		if err := g.processAlbumTracks(ctx, plan, artistItem, albumID, albumData); err != nil {
			log.Printf("ERROR: process_album_tracks_failed album_id=%s album_name=%s error=%v", albumID, albumData.Name, err)
			continue
		}
	}

	return nil
}

// processAlbumTracks processes tracks in an album and adds to plan.
func (g *Generator) processAlbumTracks(ctx context.Context, plan *DownloadPlan, parentItem *PlanItem, albumID string, albumData spotigo.SimplifiedAlbum) error {
	// Fetch full album data to get tracks
	album, err := g.spotifyClient.GetAlbum(ctx, albumID)
	if err != nil {
		log.Printf("ERROR: api_call_failed type=album spotify_id=%s error=%v", albumID, err)
		return fmt.Errorf("failed to get album: %w", err)
	}

	// Create album item
	albumSpotifyURL := ""
	if album.ExternalURLs != nil {
		albumSpotifyURL = album.ExternalURLs.Spotify
	}

	albumItem := &PlanItem{
		ItemID:     fmt.Sprintf("album:%s", albumID),
		ItemType:   PlanItemTypeAlbum,
		SpotifyID:  albumID,
		SpotifyURL: albumSpotifyURL,
		ParentID:   parentItem.ItemID,
		Name:       album.Name,
		Status:     PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"album_type":   album.AlbumType,
			"release_date": album.ReleaseDate,
		},
	}
	plan.AddItem(albumItem)
	parentItem.ChildIDs = append(parentItem.ChildIDs, albumItem.ItemID)
	g.seenAlbumIDs[albumID] = true

	// Process tracks
	// Note: spotigo.Album.Tracks is a Paging object
	// We need to paginate through all tracks
	tracks := album.Tracks
	if tracks == nil {
		return nil
	}

	// Process first page
	for _, track := range tracks.Items {
		trackID := track.ID
		if trackID == "" {
			continue
		}

		// Check for duplicate tracks
		if g.seenTrackIDs[trackID] {
			log.Printf("INFO: duplicate_detected type=track spotify_id=%s track_name=%s context=album", trackID, track.Name)
			existingTrackItemID := fmt.Sprintf("track:%s", trackID)
			existingTrack := plan.GetItem(existingTrackItemID)
			if existingTrack != nil {
				albumItem.ChildIDs = append(albumItem.ChildIDs, existingTrackItemID)
			}
			continue
		}

		// Create track item
		trackSpotifyURL := ""
		if track.ExternalURLs != nil {
			trackSpotifyURL = track.ExternalURLs.Spotify
		}

		trackItem := &PlanItem{
			ItemID:     fmt.Sprintf("track:%s", trackID),
			ItemType:   PlanItemTypeTrack,
			SpotifyID:  trackID,
			SpotifyURL: trackSpotifyURL,
			ParentID:   albumItem.ItemID,
			Name:       track.Name,
			Status:     PlanItemStatusPending,
			Metadata: map[string]interface{}{
				"track_number": track.TrackNumber,
				"disc_number":  track.DiscNumber,
			},
		}
		plan.AddItem(trackItem)
		albumItem.ChildIDs = append(albumItem.ChildIDs, trackItem.ItemID)
		g.seenTrackIDs[trackID] = true
	}

	// Paginate through remaining tracks
	for tracks.GetNext() != nil {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		// Get next page with rate limiting
		nextTracks, err := g.spotifyClient.NextAlbumTracks(ctx, tracks)
		if err != nil {
			log.Printf("ERROR: album_tracks_pagination_failed album_id=%s error=%v", albumID, err)
			return fmt.Errorf("failed to paginate album tracks: %w", err)
		}
		if nextTracks == nil {
			break
		}

		// Process tracks from next page
		for _, track := range nextTracks.Items {
			trackID := track.ID
			if trackID == "" {
				continue
			}

			// Check for duplicate tracks
			if g.seenTrackIDs[trackID] {
				log.Printf("INFO: duplicate_detected type=track spotify_id=%s track_name=%s context=album", trackID, track.Name)
				existingTrackItemID := fmt.Sprintf("track:%s", trackID)
				existingTrack := plan.GetItem(existingTrackItemID)
				if existingTrack != nil {
					albumItem.ChildIDs = append(albumItem.ChildIDs, existingTrackItemID)
				}
				continue
			}

			// Create track item
			trackSpotifyURL := ""
			if track.ExternalURLs != nil {
				trackSpotifyURL = track.ExternalURLs.Spotify
			}

			trackItem := &PlanItem{
				ItemID:     fmt.Sprintf("track:%s", trackID),
				ItemType:   PlanItemTypeTrack,
				SpotifyID:  trackID,
				SpotifyURL: trackSpotifyURL,
				ParentID:   albumItem.ItemID,
				Name:       track.Name,
				Status:     PlanItemStatusPending,
				Metadata: map[string]interface{}{
					"track_number": track.TrackNumber,
					"disc_number":  track.DiscNumber,
				},
			}
			plan.AddItem(trackItem)
			albumItem.ChildIDs = append(albumItem.ChildIDs, trackItem.ItemID)
			g.seenTrackIDs[trackID] = true
		}

		// Update tracks to next page for next iteration
		tracks = nextTracks
	}

	return nil
}

// processPlaylist processes a playlist and adds tracks/M3U to plan.
func (g *Generator) processPlaylist(ctx context.Context, plan *DownloadPlan, playlist config.MusicSource) error {
	playlistID := extractPlaylistID(playlist.URL)

	// Check for duplicates
	if g.seenPlaylistIDs[playlistID] {
		log.Printf("INFO: duplicate_detected type=playlist spotify_id=%s url=%s", playlistID, playlist.URL)
		return nil // Skip duplicate
	}

	// Fetch playlist metadata
	playlistData, err := g.spotifyClient.GetPlaylist(ctx, playlistID)
	if err != nil {
		log.Printf("ERROR: api_call_failed type=playlist spotify_id=%s error=%v", playlistID, err)
		// Create failed item
		item := &PlanItem{
			ItemID:   fmt.Sprintf("playlist:error:%s", playlist.Name),
			ItemType: PlanItemTypePlaylist,
			Name:     playlist.Name,
			Status:   PlanItemStatusFailed,
			Error:    err.Error(),
			Metadata: map[string]interface{}{
				"source_url": playlist.URL,
				"error":      err.Error(),
			},
		}
		plan.AddItem(item)
		return err
	}

	playlistName := playlistData.Name
	if playlistName == "" {
		playlistName = playlist.Name
	}

	playlistSpotifyURL := ""
	if playlistData.ExternalURLs != nil {
		playlistSpotifyURL = playlistData.ExternalURLs.Spotify
	}
	if playlistSpotifyURL == "" {
		playlistSpotifyURL = playlist.URL
	}

	// Create playlist item
	playlistItem := &PlanItem{
		ItemID:     fmt.Sprintf("playlist:%s", playlistID),
		ItemType:   PlanItemTypePlaylist,
		SpotifyID:  playlistID,
		SpotifyURL: playlistSpotifyURL,
		Name:       playlistName,
		Status:     PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"source_name": playlist.Name,
			"source_url":  playlist.URL,
		},
	}
	if playlistData.Description != nil && *playlistData.Description != "" {
		playlistItem.Metadata["description"] = *playlistData.Description
	}
	plan.AddItem(playlistItem)
	g.seenPlaylistIDs[playlistID] = true

	// Process playlist tracks using PlaylistTracks method
	if g.playlistTracksFunc == nil {
		return fmt.Errorf("playlistTracksFunc not provided")
	}

	tracks, err := g.playlistTracksFunc(ctx, playlistID, nil)
	if err != nil {
		return fmt.Errorf("failed to get playlist tracks: %w", err)
	}

	// Process first page
	for _, trackItem := range tracks.Items {
		// trackItem.Track can be a Track or SimplifiedTrack
		// We need to handle both cases
		var trackID, trackName, trackSpotifyURL string
		var isLocal bool

		// Type assert to get the actual track
		switch t := trackItem.Track.(type) {
		case *spotigo.Track:
			if t == nil {
				continue
			}
			isLocal = t.IsLocal
			trackID = t.ID
			trackName = t.Name
			if t.ExternalURLs != nil {
				trackSpotifyURL = t.ExternalURLs.Spotify
			}
		case spotigo.Track:
			isLocal = t.IsLocal
			trackID = t.ID
			trackName = t.Name
			if t.ExternalURLs != nil {
				trackSpotifyURL = t.ExternalURLs.Spotify
			}
		case *spotigo.SimplifiedTrack:
			if t == nil {
				continue
			}
			isLocal = t.IsLocal
			trackID = t.ID
			trackName = t.Name
			if t.ExternalURLs != nil {
				trackSpotifyURL = t.ExternalURLs.Spotify
			}
		case spotigo.SimplifiedTrack:
			isLocal = t.IsLocal
			trackID = t.ID
			trackName = t.Name
			if t.ExternalURLs != nil {
				trackSpotifyURL = t.ExternalURLs.Spotify
			}
		default:
			// Unknown track type, skip
			continue
		}

		// Check if track is local (not downloadable)
		if isLocal {
			continue
		}

		if trackID == "" {
			continue
		}

		// Check for duplicate tracks
		if g.seenTrackIDs[trackID] {
			log.Printf("INFO: duplicate_detected type=track spotify_id=%s track_name=%s context=playlist", trackID, trackName)
			existingTrackItemID := fmt.Sprintf("track:%s", trackID)
			existingTrack := plan.GetItem(existingTrackItemID)
			if existingTrack != nil {
				playlistItem.ChildIDs = append(playlistItem.ChildIDs, existingTrackItemID)
			}
			continue
		}

		// Create track item
		trackPlanItem := &PlanItem{
			ItemID:     fmt.Sprintf("track:%s", trackID),
			ItemType:   PlanItemTypeTrack,
			SpotifyID:  trackID,
			SpotifyURL: trackSpotifyURL,
			ParentID:   playlistItem.ItemID,
			Name:       trackName,
			Status:     PlanItemStatusPending,
			Metadata: map[string]interface{}{
				"added_at": trackItem.AddedAt,
			},
		}
		plan.AddItem(trackPlanItem)
		playlistItem.ChildIDs = append(playlistItem.ChildIDs, trackPlanItem.ItemID)
		g.seenTrackIDs[trackID] = true
	}

	// Paginate through remaining tracks
	for tracks.GetNext() != nil {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		// Get next page with rate limiting
		nextTracks, err := g.spotifyClient.NextPlaylistTracks(ctx, tracks)
		if err != nil {
			log.Printf("ERROR: playlist_tracks_pagination_failed playlist_id=%s error=%v", playlistID, err)
			return fmt.Errorf("failed to paginate playlist tracks: %w", err)
		}
		if nextTracks == nil {
			break
		}

		// Process tracks from next page
		for _, trackItem := range nextTracks.Items {
			// trackItem.Track can be a Track or SimplifiedTrack
			// We need to handle both cases
			var trackID, trackName, trackSpotifyURL string
			var isLocal bool

			// Type assert to get the actual track
			switch t := trackItem.Track.(type) {
			case *spotigo.Track:
				if t == nil {
					continue
				}
				isLocal = t.IsLocal
				trackID = t.ID
				trackName = t.Name
				if t.ExternalURLs != nil {
					trackSpotifyURL = t.ExternalURLs.Spotify
				}
			case spotigo.Track:
				isLocal = t.IsLocal
				trackID = t.ID
				trackName = t.Name
				if t.ExternalURLs != nil {
					trackSpotifyURL = t.ExternalURLs.Spotify
				}
			case *spotigo.SimplifiedTrack:
				if t == nil {
					continue
				}
				isLocal = t.IsLocal
				trackID = t.ID
				trackName = t.Name
				if t.ExternalURLs != nil {
					trackSpotifyURL = t.ExternalURLs.Spotify
				}
			case spotigo.SimplifiedTrack:
				isLocal = t.IsLocal
				trackID = t.ID
				trackName = t.Name
				if t.ExternalURLs != nil {
					trackSpotifyURL = t.ExternalURLs.Spotify
				}
			default:
				// Unknown track type, skip
				continue
			}

			// Check if track is local (not downloadable)
			if isLocal {
				continue
			}

			if trackID == "" {
				continue
			}

			// Check for duplicate tracks
			if g.seenTrackIDs[trackID] {
				log.Printf("INFO: duplicate_detected type=track spotify_id=%s track_name=%s context=playlist", trackID, trackName)
				existingTrackItemID := fmt.Sprintf("track:%s", trackID)
				existingTrack := plan.GetItem(existingTrackItemID)
				if existingTrack != nil {
					playlistItem.ChildIDs = append(playlistItem.ChildIDs, existingTrackItemID)
				}
				continue
			}

			// Create track item
			trackPlanItem := &PlanItem{
				ItemID:     fmt.Sprintf("track:%s", trackID),
				ItemType:   PlanItemTypeTrack,
				SpotifyID:  trackID,
				SpotifyURL: trackSpotifyURL,
				ParentID:   playlistItem.ItemID,
				Name:       trackName,
				Status:     PlanItemStatusPending,
				Metadata: map[string]interface{}{
					"added_at": trackItem.AddedAt,
				},
			}
			plan.AddItem(trackPlanItem)
			playlistItem.ChildIDs = append(playlistItem.ChildIDs, trackPlanItem.ItemID)
			g.seenTrackIDs[trackID] = true
		}

		// Update tracks to next page for next iteration
		tracks = nextTracks
	}

	// Create M3U item (child of playlist)
	m3uItem := &PlanItem{
		ItemID:   fmt.Sprintf("m3u:%s", playlistID),
		ItemType: PlanItemTypeM3U,
		ParentID: playlistItem.ItemID,
		Name:     fmt.Sprintf("%s.m3u", playlistName),
		Status:   PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"playlist_name": playlistName,
		},
	}
	plan.AddItem(m3uItem)
	playlistItem.ChildIDs = append(playlistItem.ChildIDs, m3uItem.ItemID)

	return nil
}

// processAlbum processes an album and adds tracks/M3U to plan.
func (g *Generator) processAlbum(ctx context.Context, plan *DownloadPlan, album config.MusicSource) error {
	albumID := extractAlbumID(album.URL)

	// Check for duplicates
	if g.seenAlbumIDs[albumID] {
		log.Printf("INFO: duplicate_detected type=album spotify_id=%s url=%s", albumID, album.URL)
		// Album already exists - check if M3U should be created
		if album.CreateM3U {
			existingAlbumItemID := fmt.Sprintf("album:%s", albumID)
			existingAlbumItem := plan.GetItem(existingAlbumItemID)
			if existingAlbumItem != nil {
				existingAlbumItem.Metadata["create_m3u"] = true
				// Check if M3U item already exists
				m3uItemID := fmt.Sprintf("m3u:album:%s", albumID)
				existingM3UItem := plan.GetItem(m3uItemID)
				if existingM3UItem == nil {
					albumName := existingAlbumItem.Name
					m3uItem := &PlanItem{
						ItemID:   m3uItemID,
						ItemType: PlanItemTypeM3U,
						ParentID: existingAlbumItem.ItemID,
						Name:     fmt.Sprintf("%s.m3u", albumName),
						Status:   PlanItemStatusPending,
						Metadata: map[string]interface{}{
							"album_name": albumName,
						},
					}
					plan.AddItem(m3uItem)
					existingAlbumItem.ChildIDs = append(existingAlbumItem.ChildIDs, m3uItem.ItemID)
				}
			}
		}
		return nil // Skip duplicate
	}

	// Fetch album metadata
	albumData, err := g.spotifyClient.GetAlbum(ctx, albumID)
	if err != nil {
		log.Printf("ERROR: api_call_failed type=album spotify_id=%s error=%v", albumID, err)
		// Create failed item
		item := &PlanItem{
			ItemID:   fmt.Sprintf("album:error:%s", album.Name),
			ItemType: PlanItemTypeAlbum,
			Name:     album.Name,
			Status:   PlanItemStatusFailed,
			Error:    err.Error(),
			Metadata: map[string]interface{}{
				"source_url": album.URL,
				"error":      err.Error(),
			},
		}
		plan.AddItem(item)
		return err
	}

	albumName := albumData.Name
	if albumName == "" {
		albumName = album.Name
	}

	albumSpotifyURL := ""
	if albumData.ExternalURLs != nil {
		albumSpotifyURL = albumData.ExternalURLs.Spotify
	}
	if albumSpotifyURL == "" {
		albumSpotifyURL = album.URL
	}

	// Create album item
	albumItem := &PlanItem{
		ItemID:     fmt.Sprintf("album:%s", albumID),
		ItemType:   PlanItemTypeAlbum,
		SpotifyID:  albumID,
		SpotifyURL: albumSpotifyURL,
		Name:       albumName,
		Status:     PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"source_name":  album.Name,
			"source_url":   album.URL,
			"create_m3u":   album.CreateM3U,
			"album_type":   albumData.AlbumType,
			"release_date": albumData.ReleaseDate,
		},
	}
	plan.AddItem(albumItem)
	g.seenAlbumIDs[albumID] = true

	// Process album tracks
	// Create a simplified album for processAlbumTracks
	simplifiedAlbum := spotigo.SimplifiedAlbum{
		ID:          albumID,
		Name:        albumName,
		AlbumType:   albumData.AlbumType,
		ReleaseDate: albumData.ReleaseDate,
		ExternalURLs: albumData.ExternalURLs,
	}

	// Use a dummy parent item for albums processed directly (not from artist)
	dummyParent := &PlanItem{
		ItemID:   fmt.Sprintf("album_parent:%s", albumID),
		ItemType: PlanItemTypeAlbum,
	}
	if err := g.processAlbumTracks(ctx, plan, dummyParent, albumID, simplifiedAlbum); err != nil {
		return fmt.Errorf("failed to process album tracks: %w", err)
	}

	// Update album item with child IDs from dummy parent
	albumItem.ChildIDs = dummyParent.ChildIDs

	// Create M3U item only if requested
	if album.CreateM3U {
		m3uItem := &PlanItem{
			ItemID:   fmt.Sprintf("m3u:album:%s", albumID),
			ItemType: PlanItemTypeM3U,
			ParentID: albumItem.ItemID,
			Name:     fmt.Sprintf("%s.m3u", albumName),
			Status:   PlanItemStatusPending,
			Metadata: map[string]interface{}{
				"album_name": albumName,
			},
		}
		plan.AddItem(m3uItem)
		albumItem.ChildIDs = append(albumItem.ChildIDs, m3uItem.ItemID)
	}

	return nil
}
