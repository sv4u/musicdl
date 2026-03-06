package plan

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/spotify"
	"github.com/sv4u/spotigo/v2"
)

const rateLimitRetryMaxAttempts = 3

// SetRateLimitNotifier sets an optional callback used during Spotify rate limit wait.
// It is called with (totalSec, remainingSec) at the start and each second until remainingSec is 0.
func (g *Generator) SetRateLimitNotifier(fn func(totalSec, remainingSec int)) {
	g.rateLimitNotifier = fn
}

// runWithRateLimitRetry runs fn. If fn returns a spotify.RateLimitError, it sleeps for RetryAfter+10 seconds and retries.
func (g *Generator) runWithRateLimitRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 1; attempt <= rateLimitRetryMaxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		var rateLimitErr *spotify.RateLimitError
		if !errors.As(lastErr, &rateLimitErr) || attempt == rateLimitRetryMaxAttempts {
			return lastErr
		}
		waitSec := rateLimitErr.RetryAfter + 10
		if rateLimitErr.RetryAfter <= 0 {
			waitSec = 10
		}
		log.Printf("INFO: rate_limit_retry phase=plan attempt=%d max_retries=%d retry_after_seconds=%d sleeping_seconds=%d", attempt, rateLimitRetryMaxAttempts, rateLimitErr.RetryAfter, waitSec)
		remaining := waitSec
		for remaining > 0 {
			if g.rateLimitNotifier != nil {
				g.rateLimitNotifier(waitSec, remaining)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
				remaining--
			}
		}
		if g.rateLimitNotifier != nil {
			g.rateLimitNotifier(waitSec, 0)
		}
	}
	return lastErr
}

// YouTubeMetadataProvider defines the interface for extracting YouTube metadata.
// This allows for easier testing with mocks.
type YouTubeMetadataProvider interface {
	GetVideoMetadata(ctx context.Context, videoURL string) (*audio.YouTubeVideoMetadata, error)
	GetPlaylistInfo(ctx context.Context, playlistURL string) (*audio.YouTubePlaylistInfo, error)
}

// SpotifyClientInterface defines the interface for Spotify client operations.
type SpotifyClientInterface interface {
	GetTrack(ctx context.Context, trackIDOrURL string) (*spotigo.Track, error)
	GetAlbum(ctx context.Context, albumIDOrURL string) (*spotigo.Album, error)
	GetArtist(ctx context.Context, artistIDOrURL string) (*spotigo.Artist, error)
	GetPlaylist(ctx context.Context, playlistIDOrURL string) (*spotigo.Playlist, error)
	AllArtistAlbums(ctx context.Context, artistIDOrURL string, progressFn func(spotigo.PaginationProgress)) ([]spotigo.SimplifiedAlbum, error)
	AllAlbumTracks(ctx context.Context, albumIDOrURL string, progressFn func(spotigo.PaginationProgress)) ([]spotigo.SimplifiedTrack, error)
	AllPlaylistTracks(ctx context.Context, playlistIDOrURL string, progressFn func(spotigo.PaginationProgress)) ([]spotigo.PlaylistTrack, error)
	Search(ctx context.Context, query, searchType string, opts *spotigo.SearchOptions) (*spotigo.SearchResponse, error)
}

// PlanProgressCallback is called during plan generation to report progress.
type PlanProgressCallback func(message string, itemsFound int)

// Generator generates download plans from configuration.
type Generator struct {
	config                 *config.MusicDLConfig
	spotifyClient          SpotifyClientInterface
	audioProvider          YouTubeMetadataProvider
	seenTrackIDs           map[string]bool
	seenAlbumIDs           map[string]bool
	seenPlaylistIDs        map[string]bool
	seenArtistIDs          map[string]bool
	seenYouTubeVideoIDs    map[string]bool
	seenYouTubePlaylistIDs map[string]bool
	// rateLimitNotifier is optional; when set, called during Spotify rate limit wait with (totalSec, remainingSec).
	rateLimitNotifier func(totalSec, remainingSec int)
	// planProgressCallback is optional; when set, called during plan generation to report progress.
	planProgressCallback PlanProgressCallback
}

// NewGenerator creates a new plan generator.
func NewGenerator(cfg *config.MusicDLConfig, spotifyClient SpotifyClientInterface, audioProvider YouTubeMetadataProvider) *Generator {
	return &Generator{
		config:                 cfg,
		spotifyClient:          spotifyClient,
		audioProvider:          audioProvider,
		seenTrackIDs:           make(map[string]bool),
		seenAlbumIDs:           make(map[string]bool),
		seenPlaylistIDs:        make(map[string]bool),
		seenArtistIDs:          make(map[string]bool),
		seenYouTubeVideoIDs:    make(map[string]bool),
		seenYouTubePlaylistIDs: make(map[string]bool),
	}
}

// SetPlanProgressCallback sets the callback for plan generation progress.
func (g *Generator) SetPlanProgressCallback(cb PlanProgressCallback) {
	g.planProgressCallback = cb
}

// notifyPlanProgress calls the progress callback if set.
func (g *Generator) notifyPlanProgress(message string, itemsFound int) {
	if g.planProgressCallback != nil {
		g.planProgressCallback(message, itemsFound)
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
		if err := ctx.Err(); err != nil {
			return plan, err
		}
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
		if err := ctx.Err(); err != nil {
			return plan, err
		}
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
		if err := ctx.Err(); err != nil {
			return plan, err
		}
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
		if err := ctx.Err(); err != nil {
			return plan, err
		}
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
	// Check if this is a YouTube URL
	if IsYouTubeVideo(song.URL) {
		return g.processYouTubeVideo(ctx, plan, song)
	}

	// Process as Spotify track
	trackID := spotigo.ExtractID(song.URL, "track")
	if trackID == "" {
		return fmt.Errorf("invalid or empty track ID extracted from URL: %s", song.URL)
	}

	// Check for duplicates
	if g.seenTrackIDs[trackID] {
		log.Printf("INFO: duplicate_detected type=track spotify_id=%s url=%s", trackID, song.URL)
		return nil // Skip duplicate
	}

	// Fetch track metadata (with rate limit retry)
	var track *spotigo.Track
	err := g.runWithRateLimitRetry(ctx, func() error {
		var err2 error
		track, err2 = g.spotifyClient.GetTrack(ctx, trackID)
		return err2
	})
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

	metadata := map[string]interface{}{
		"source_name":   song.Name,
		"source_url":    song.URL,
		"title":         trackName,
		"track_number":  track.TrackNumber,
		"disc_number":   track.DiscNumber,
		"duration_ms":   track.DurationMs,
	}
	if len(track.Artists) > 0 {
		metadata["artist"] = track.Artists[0].Name
	}
	if track.Album != nil {
		metadata["album"] = track.Album.Name
	}
	item := &PlanItem{
		ItemID:     fmt.Sprintf("track:%s", trackID),
		ItemType:   PlanItemTypeTrack,
		SpotifyID:  trackID,
		SpotifyURL: spotifyURL,
		Name:       trackName,
		Status:     PlanItemStatusPending,
		Metadata:   metadata,
	}
	plan.AddItem(item)
	g.seenTrackIDs[trackID] = true
	return nil
}

// processYouTubeVideo processes a YouTube video URL and adds it to the plan.
func (g *Generator) processYouTubeVideo(ctx context.Context, plan *DownloadPlan, song config.MusicSource) error {
	if g.audioProvider == nil {
		return fmt.Errorf("audioProvider is required for YouTube video processing")
	}

	videoID := ExtractYouTubeVideoID(song.URL)
	if videoID == "" {
		return fmt.Errorf("invalid or empty YouTube video ID extracted from URL: %s", song.URL)
	}

	// Check for duplicates
	if g.seenYouTubeVideoIDs[videoID] {
		log.Printf("INFO: duplicate_detected type=youtube_video video_id=%s url=%s", videoID, song.URL)
		return nil // Skip duplicate
	}

	// Extract metadata using audioProvider
	videoMetadata, err := g.audioProvider.GetVideoMetadata(ctx, song.URL)
	if err != nil {
		log.Printf("ERROR: youtube_metadata_extraction_failed video_id=%s error=%v", videoID, err)
		// Create failed item
		item := &PlanItem{
			ItemID:   fmt.Sprintf("track:youtube:error:%s", song.Name),
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
	trackName := videoMetadata.Title
	if trackName == "" {
		trackName = song.Name
	}

	metadata := map[string]interface{}{
		"source_name":      song.Name,
		"source_url":       song.URL,
		"title":            trackName,
		"album":            "YouTube",
		"youtube_metadata": videoMetadata,
	}
	if videoMetadata.Uploader != "" {
		metadata["artist"] = videoMetadata.Uploader
	}
	if videoMetadata.Duration > 0 {
		metadata["duration"] = videoMetadata.Duration
	}
	item := &PlanItem{
		ItemID:     fmt.Sprintf("track:youtube:%s", videoID),
		ItemType:   PlanItemTypeTrack,
		YouTubeURL: song.URL,
		Name:       trackName,
		Status:     PlanItemStatusPending,
		Metadata:   metadata,
	}

	// Attempt Spotify enhancement (non-blocking)
	g.enhanceYouTubeWithSpotify(ctx, item, videoMetadata)

	plan.AddItem(item)
	g.seenYouTubeVideoIDs[videoID] = true
	return nil
}

// enhanceYouTubeWithSpotify attempts to enhance YouTube metadata with Spotify data.
// This is non-blocking - if enhancement fails, the item still proceeds with YouTube metadata.
func (g *Generator) enhanceYouTubeWithSpotify(ctx context.Context, item *PlanItem, ytMetadata *audio.YouTubeVideoMetadata) {
	if g.spotifyClient == nil {
		return // No Spotify client available
	}

	// Build search query using Spotify query syntax
	title := ytMetadata.Title
	if title == "" {
		title = item.Name
	}
	if title == "" {
		return // Can't search without a title
	}

	artist := ytMetadata.Uploader
	if artist == "" {
		if artistFromMeta, ok := item.Metadata["artist"].(string); ok && artistFromMeta != "" {
			artist = artistFromMeta
		}
	}
	if artist == "" {
		// Try search with just title
		searchQuery := fmt.Sprintf("track:%s", title)
		g.performSpotifySearch(ctx, item, searchQuery, "")
		return
	}

	// Search with both title and artist
	searchQuery := fmt.Sprintf("track:%s artist:%s", title, artist)
	g.performSpotifySearch(ctx, item, searchQuery, artist)
}

// performSpotifySearch performs a Spotify search and enhances the item if a match is found.
func (g *Generator) performSpotifySearch(ctx context.Context, item *PlanItem, searchQuery, expectedArtist string) {
	// Search for tracks (with rate limit retry)
	opts := &spotigo.SearchOptions{
		Limit: 10, // Get up to 10 results to find best match
	}
	var response *spotigo.SearchResponse
	err := g.runWithRateLimitRetry(ctx, func() error {
		var err2 error
		response, err2 = g.spotifyClient.Search(ctx, searchQuery, "track", opts)
		return err2
	})
	if err != nil {
		log.Printf("WARN: spotify_enhancement_search_failed youtube_id=%s query=%s error=%v", ExtractYouTubeVideoID(item.YouTubeURL), searchQuery, err)
		return
	}

	if response == nil || response.Tracks == nil || len(response.Tracks.Items) == 0 {
		log.Printf("INFO: spotify_enhancement_no_results youtube_id=%s query=%s", ExtractYouTubeVideoID(item.YouTubeURL), searchQuery)
		return
	}

	// Find first track with matching artist (if expectedArtist provided)
	var bestTrack *spotigo.Track
	for _, track := range response.Tracks.Items {
		if expectedArtist == "" {
			// No expected artist, use first result
			bestTrack = &track
			break
		}

		// Check if any artist matches (case-insensitive)
		for _, trackArtist := range track.Artists {
			if strings.EqualFold(trackArtist.Name, expectedArtist) {
				bestTrack = &track
				break
			}
		}
		if bestTrack != nil {
			break
		}
	}

	// If no artist match found but we have results, use first result anyway
	if bestTrack == nil && len(response.Tracks.Items) > 0 {
		bestTrack = &response.Tracks.Items[0]
	}

	if bestTrack == nil {
		log.Printf("INFO: spotify_enhancement_no_match youtube_id=%s query=%s", ExtractYouTubeVideoID(item.YouTubeURL), searchQuery)
		return
	}

	// Get album metadata for the track (with rate limit retry)
	var album *spotigo.Album
	if bestTrack.Album != nil && bestTrack.Album.ID != "" {
		err := g.runWithRateLimitRetry(ctx, func() error {
			var err2 error
			album, err2 = g.spotifyClient.GetAlbum(ctx, bestTrack.Album.ID)
			return err2
		})
		if err != nil {
			log.Printf("WARN: spotify_enhancement_album_fetch_failed track_id=%s album_id=%s error=%v", bestTrack.ID, bestTrack.Album.ID, err)
			// Continue without album metadata
		}
	}

	// Build enhancement metadata
	enhancement := make(map[string]interface{})

	// Track metadata
	if bestTrack.Name != "" {
		enhancement["title"] = bestTrack.Name
	}
	if len(bestTrack.Artists) > 0 {
		enhancement["artist"] = bestTrack.Artists[0].Name
	}
	if bestTrack.TrackNumber > 0 {
		enhancement["track_number"] = bestTrack.TrackNumber
	}
	if bestTrack.DiscNumber > 0 {
		enhancement["disc_number"] = bestTrack.DiscNumber
	}
	if bestTrack.Explicit {
		enhancement["explicit"] = true
	}

	// Album metadata
	if album != nil {
		enhancement["album"] = album.Name
		if len(album.Artists) > 0 {
			enhancement["album_artist"] = album.Artists[0].Name
		}
		if album.ReleaseDate != "" {
			enhancement["date"] = album.ReleaseDate
			// Extract year
			parts := strings.Split(album.ReleaseDate, "-")
			if len(parts) > 0 {
				var year int
				if _, err := fmt.Sscanf(parts[0], "%d", &year); err == nil {
					enhancement["year"] = year
				}
			}
		}
		enhancement["tracks_count"] = album.TotalTracks
	} else if bestTrack.Album != nil {
		enhancement["album"] = bestTrack.Album.Name
		if len(bestTrack.Album.Artists) > 0 {
			enhancement["album_artist"] = bestTrack.Album.Artists[0].Name
		}
	}

	// External URLs
	if bestTrack.ExternalURLs != nil && bestTrack.ExternalURLs.Spotify != "" {
		enhancement["spotify_url"] = bestTrack.ExternalURLs.Spotify
	}

	// Cover art
	if album != nil && len(album.Images) > 0 {
		enhancement["cover_url"] = album.Images[0].URL
	} else if bestTrack.Album != nil && len(bestTrack.Album.Images) > 0 {
		enhancement["cover_url"] = bestTrack.Album.Images[0].URL
	}

	// ISRC
	if bestTrack.ExternalIDs != nil && bestTrack.ExternalIDs.ISRC != nil && *bestTrack.ExternalIDs.ISRC != "" {
		enhancement["isrc"] = *bestTrack.ExternalIDs.ISRC
	}

	// Store enhancement in item metadata
	item.Metadata["spotify_enhancement"] = enhancement

	log.Printf("INFO: spotify_enhancement_applied youtube_id=%s spotify_id=%s track=%s", ExtractYouTubeVideoID(item.YouTubeURL), bestTrack.ID, bestTrack.Name)
}

// processArtist processes an artist and adds albums/tracks to plan.
func (g *Generator) processArtist(ctx context.Context, plan *DownloadPlan, artist config.MusicSource) error {
	// Explicitly reject YouTube URLs
	if IsYouTubeURL(artist.URL) {
		errMsg := fmt.Sprintf("YouTube URLs are not supported for artists. Use songs or playlists instead. URL: %s", artist.URL)
		// Create failed item
		item := &PlanItem{
			ItemID:   fmt.Sprintf("artist:error:%s", artist.Name),
			ItemType: PlanItemTypeArtist,
			Name:     artist.Name,
			Status:   PlanItemStatusFailed,
			Error:    errMsg,
			Metadata: map[string]interface{}{
				"source_url": artist.URL,
				"error":      errMsg,
			},
		}
		plan.AddItem(item)
		return fmt.Errorf("%s", errMsg)
	}

	artistID := spotigo.ExtractID(artist.URL, "artist")
	if artistID == "" {
		return fmt.Errorf("invalid or empty artist ID extracted from URL: %s", artist.URL)
	}

	// Check for duplicates
	if g.seenArtistIDs[artistID] {
		log.Printf("INFO: duplicate_detected type=artist spotify_id=%s url=%s", artistID, artist.URL)
		return nil // Skip duplicate
	}

	// Fetch artist metadata (with rate limit retry)
	var artistData *spotigo.Artist
	err := g.runWithRateLimitRetry(ctx, func() error {
		var err2 error
		artistData, err2 = g.spotifyClient.GetArtist(ctx, artistID)
		return err2
	})
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

	// Get artist albums (with rate limit retry)
	var albums []spotigo.SimplifiedAlbum
	err = g.runWithRateLimitRetry(ctx, func() error {
		var err2 error
		albums, err2 = g.spotifyClient.AllArtistAlbums(ctx, artistID, func(p spotigo.PaginationProgress) {
			g.notifyPlanProgress(fmt.Sprintf("Fetching albums for artist '%s': %d/%d", artistName, p.FetchedItems, p.TotalItems), p.FetchedItems)
		})
		return err2
	})
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
		if err := g.processAlbumTracks(ctx, plan, artistItem, albumID); err != nil {
			log.Printf("ERROR: process_album_tracks_failed album_id=%s album_name=%s error=%v", albumID, albumData.Name, err)
			continue
		}
	}

	return nil
}

// processAlbumTracks processes tracks in an album and adds to plan.
func (g *Generator) processAlbumTracks(ctx context.Context, plan *DownloadPlan, parentItem *PlanItem, albumID string) error {
	// Fetch full album data (with rate limit retry)
	var album *spotigo.Album
	err := g.runWithRateLimitRetry(ctx, func() error {
		var err2 error
		album, err2 = g.spotifyClient.GetAlbum(ctx, albumID)
		return err2
	})
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

	// Fetch all tracks via spotigo's auto-pagination (with rate limit retry)
	var allTracks []spotigo.SimplifiedTrack
	err = g.runWithRateLimitRetry(ctx, func() error {
		var err2 error
		allTracks, err2 = g.spotifyClient.AllAlbumTracks(ctx, albumID, func(p spotigo.PaginationProgress) {
			g.notifyPlanProgress(fmt.Sprintf("Fetching tracks for album '%s': %d/%d", album.Name, p.FetchedItems, p.TotalItems), p.FetchedItems)
		})
		return err2
	})
	if err != nil {
		log.Printf("ERROR: album_tracks_fetch_failed album_id=%s error=%v", albumID, err)
		return fmt.Errorf("failed to fetch album tracks: %w", err)
	}

	for _, track := range allTracks {
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

		metadata := map[string]interface{}{
			"track_number": track.TrackNumber,
			"disc_number":  track.DiscNumber,
			"title":        track.Name,
			"album":        album.Name,
			"duration_ms":  track.DurationMs,
		}
		if len(track.Artists) > 0 {
			metadata["artist"] = track.Artists[0].Name
		}
		trackItem := &PlanItem{
			ItemID:     fmt.Sprintf("track:%s", trackID),
			ItemType:   PlanItemTypeTrack,
			SpotifyID:  trackID,
			SpotifyURL: trackSpotifyURL,
			ParentID:   albumItem.ItemID,
			Name:       track.Name,
			Status:     PlanItemStatusPending,
			Metadata:   metadata,
		}
		plan.AddItem(trackItem)
		albumItem.ChildIDs = append(albumItem.ChildIDs, trackItem.ItemID)
		g.seenTrackIDs[trackID] = true
	}

	return nil
}

// processPlaylist processes a playlist and adds tracks/M3U to plan.
func (g *Generator) processPlaylist(ctx context.Context, plan *DownloadPlan, playlist config.MusicSource) error {
	// Check if this is a YouTube playlist
	if IsYouTubePlaylist(playlist.URL) {
		return g.processYouTubePlaylist(ctx, plan, playlist)
	}

	playlistID := spotigo.ExtractID(playlist.URL, "playlist")
	if playlistID == "" {
		return fmt.Errorf("invalid or empty playlist ID extracted from URL: %s", playlist.URL)
	}

	// Check for duplicates
	if g.seenPlaylistIDs[playlistID] {
		log.Printf("INFO: duplicate_detected type=playlist spotify_id=%s url=%s", playlistID, playlist.URL)
		return nil // Skip duplicate
	}

	// Fetch playlist metadata (with rate limit retry)
	var playlistData *spotigo.Playlist
	err := g.runWithRateLimitRetry(ctx, func() error {
		var err2 error
		playlistData, err2 = g.spotifyClient.GetPlaylist(ctx, playlistID)
		return err2
	})
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
			"source_name":  playlist.Name,
			"source_url":   playlist.URL,
			"create_m3u":   playlist.CreateM3U,
		},
	}
	if playlistData.Description != nil && *playlistData.Description != "" {
		playlistItem.Metadata["description"] = *playlistData.Description
	}
	plan.AddItem(playlistItem)
	g.seenPlaylistIDs[playlistID] = true

	// Fetch all playlist tracks via spotigo's auto-pagination (with rate limit retry)
	var allTracks []spotigo.PlaylistTrack
	err = g.runWithRateLimitRetry(ctx, func() error {
		var err2 error
		allTracks, err2 = g.spotifyClient.AllPlaylistTracks(ctx, playlistID, func(p spotigo.PaginationProgress) {
			g.notifyPlanProgress(fmt.Sprintf("Fetching playlist '%s': %d/%d tracks", playlistName, p.FetchedItems, p.TotalItems), p.FetchedItems)
		})
		return err2
	})
	if err != nil {
		return fmt.Errorf("failed to get playlist tracks: %w", err)
	}

	for _, trackItem := range allTracks {
		t := trackItem.Track
		if t == nil || trackItem.IsLocal || t.IsLocal || t.ID == "" {
			continue
		}

		trackID := t.ID
		if g.seenTrackIDs[trackID] {
			log.Printf("INFO: duplicate_detected type=track spotify_id=%s track_name=%s context=playlist", trackID, t.Name)
			existingTrackItemID := fmt.Sprintf("track:%s", trackID)
			existingTrack := plan.GetItem(existingTrackItemID)
			if existingTrack != nil {
				playlistItem.ChildIDs = append(playlistItem.ChildIDs, existingTrackItemID)
			}
			continue
		}

		trackSpotifyURL := ""
		if t.ExternalURLs != nil {
			trackSpotifyURL = t.ExternalURLs.Spotify
		}

		artist := ""
		if len(t.Artists) > 0 {
			artist = t.Artists[0].Name
		}

		albumName := ""
		if t.Album != nil {
			albumName = t.Album.Name
		}

		metadata := map[string]interface{}{
			"added_at":     trackItem.AddedAt,
			"title":        t.Name,
			"track_number": t.TrackNumber,
			"disc_number":  t.DiscNumber,
			"duration_ms":  t.DurationMs,
		}
		if artist != "" {
			metadata["artist"] = artist
		}
		if albumName != "" {
			metadata["album"] = albumName
		}
		trackPlanItem := &PlanItem{
			ItemID:     fmt.Sprintf("track:%s", trackID),
			ItemType:   PlanItemTypeTrack,
			SpotifyID:  trackID,
			SpotifyURL: trackSpotifyURL,
			ParentID:   playlistItem.ItemID,
			Name:       t.Name,
			Status:     PlanItemStatusPending,
			Metadata:   metadata,
		}
		plan.AddItem(trackPlanItem)
		playlistItem.ChildIDs = append(playlistItem.ChildIDs, trackPlanItem.ItemID)
		g.seenTrackIDs[trackID] = true
	}

	if playlist.CreateM3U {
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
	}

	return nil
}

// processYouTubePlaylist processes a YouTube playlist URL and adds it to the plan.
func (g *Generator) processYouTubePlaylist(ctx context.Context, plan *DownloadPlan, playlist config.MusicSource) error {
	if g.audioProvider == nil {
		return fmt.Errorf("audioProvider is required for YouTube playlist processing")
	}

	playlistID := ExtractYouTubePlaylistID(playlist.URL)
	if playlistID == "" {
		return fmt.Errorf("invalid or empty YouTube playlist ID extracted from URL: %s", playlist.URL)
	}

	// Check for duplicates
	if g.seenYouTubePlaylistIDs[playlistID] {
		log.Printf("INFO: duplicate_detected type=youtube_playlist playlist_id=%s url=%s", playlistID, playlist.URL)
		return nil // Skip duplicate
	}

	// Extract playlist metadata using audioProvider
	playlistInfo, err := g.audioProvider.GetPlaylistInfo(ctx, playlist.URL)
	if err != nil {
		log.Printf("ERROR: youtube_playlist_metadata_extraction_failed playlist_id=%s error=%v", playlistID, err)
		// Create failed item
		item := &PlanItem{
			ItemID:   fmt.Sprintf("playlist:youtube:error:%s", playlist.Name),
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

	// Create playlist item
	playlistName := playlistInfo.Title
	if playlistName == "" {
		playlistName = playlist.Name
	}

	playlistItem := &PlanItem{
		ItemID:     fmt.Sprintf("playlist:youtube:%s", playlistID),
		ItemType:   PlanItemTypePlaylist,
		YouTubeURL: playlist.URL,
		Name:       playlistName,
		Status:     PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"source_name":           playlist.Name,
			"source_url":            playlist.URL,
			"create_m3u":            playlist.CreateM3U,
			"youtube_playlist_info": playlistInfo,
		},
	}

	if playlistInfo.Description != "" {
		playlistItem.Metadata["description"] = playlistInfo.Description
	}

	plan.AddItem(playlistItem)
	g.seenYouTubePlaylistIDs[playlistID] = true

	// Process each video in the playlist
	for _, videoMeta := range playlistInfo.Entries {
		videoID := videoMeta.VideoID
		if videoID == "" {
			continue
		}

		// Check for duplicate videos
		if g.seenYouTubeVideoIDs[videoID] {
			log.Printf("INFO: duplicate_detected type=youtube_video video_id=%s context=playlist", videoID)
			existingTrackItemID := fmt.Sprintf("track:youtube:%s", videoID)
			existingTrack := plan.GetItem(existingTrackItemID)
			if existingTrack != nil {
				playlistItem.ChildIDs = append(playlistItem.ChildIDs, existingTrackItemID)
			}
			continue
		}

		// Create track item for video
		videoURL := videoMeta.WebpageURL
		if videoURL == "" {
			videoURL = fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
		}

		metadata := map[string]interface{}{
			"youtube_metadata": videoMeta,
			"title":            videoMeta.Title,
			"album":            "YouTube",
		}
		if videoMeta.Uploader != "" {
			metadata["artist"] = videoMeta.Uploader
		}
		if videoMeta.Duration > 0 {
			metadata["duration"] = videoMeta.Duration
		}
		trackItem := &PlanItem{
			ItemID:     fmt.Sprintf("track:youtube:%s", videoID),
			ItemType:   PlanItemTypeTrack,
			YouTubeURL: videoURL,
			ParentID:   playlistItem.ItemID,
			Name:       videoMeta.Title,
			Status:     PlanItemStatusPending,
			Metadata:   metadata,
		}

		// Attempt Spotify enhancement (non-blocking)
		g.enhanceYouTubeWithSpotify(ctx, trackItem, &videoMeta)

		plan.AddItem(trackItem)
		playlistItem.ChildIDs = append(playlistItem.ChildIDs, trackItem.ItemID)
		g.seenYouTubeVideoIDs[videoID] = true
	}

	if playlist.CreateM3U {
		m3uItem := &PlanItem{
			ItemID:   fmt.Sprintf("m3u:youtube:%s", playlistID),
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
	}

	return nil
}

// processAlbum processes an album and adds tracks/M3U to plan.
func (g *Generator) processAlbum(ctx context.Context, plan *DownloadPlan, album config.MusicSource) error {
	// Explicitly reject YouTube URLs
	if IsYouTubeURL(album.URL) {
		errMsg := fmt.Sprintf("YouTube URLs are not supported for albums. Use songs or playlists instead. URL: %s", album.URL)
		// Create failed item
		item := &PlanItem{
			ItemID:   fmt.Sprintf("album:error:%s", album.Name),
			ItemType: PlanItemTypeAlbum,
			Name:     album.Name,
			Status:   PlanItemStatusFailed,
			Error:    errMsg,
			Metadata: map[string]interface{}{
				"source_url": album.URL,
				"error":      errMsg,
			},
		}
		plan.AddItem(item)
		return fmt.Errorf("%s", errMsg)
	}

	albumID := spotigo.ExtractID(album.URL, "album")
	if albumID == "" {
		return fmt.Errorf("invalid or empty album ID extracted from URL: %s", album.URL)
	}

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

	// Fetch album metadata (with rate limit retry)
	var albumData *spotigo.Album
	err := g.runWithRateLimitRetry(ctx, func() error {
		var err2 error
		albumData, err2 = g.spotifyClient.GetAlbum(ctx, albumID)
		return err2
	})
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

	// Use processAlbumTracks with a dummy parent
	dummyParent := &PlanItem{
		ItemID:   fmt.Sprintf("album_parent:%s", albumID),
		ItemType: PlanItemTypeAlbum,
	}
	if err := g.processAlbumTracks(ctx, plan, dummyParent, albumID); err != nil {
		return fmt.Errorf("failed to process album tracks: %w", err)
	}

	// Fix up the album item created by processAlbumTracks
	albumItem := plan.GetItem(fmt.Sprintf("album:%s", albumID))
	if albumItem == nil {
		return fmt.Errorf("processAlbumTracks did not create album item for %s", albumID)
	}
	albumItem.ParentID = ""
	albumItem.SpotifyURL = ""
	if albumData.ExternalURLs != nil {
		albumItem.SpotifyURL = albumData.ExternalURLs.Spotify
	}
	if albumItem.SpotifyURL == "" {
		albumItem.SpotifyURL = album.URL
	}
	albumItem.Metadata["source_name"] = album.Name
	albumItem.Metadata["source_url"] = album.URL
	albumItem.Metadata["create_m3u"] = album.CreateM3U

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
