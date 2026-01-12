//go:build integration

package download

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/spotify"
	"github.com/sv4u/spotigo"
)

// checkYtDlpAvailable checks if yt-dlp is available for integration tests.
func checkYtDlpAvailable(t *testing.T) {
	// Try to run yt-dlp --version
	cmd := exec.Command("yt-dlp", "--version")
	if err := cmd.Run(); err != nil {
		t.Skip("yt-dlp not available, skipping YouTube integration tests")
	}
}

func TestIntegration_YouTubeVideoDownload(t *testing.T) {
	checkYtDlpAvailable(t)

	tmpDir := t.TempDir()
	cfg := &config.DownloadSettings{
		Format:    "mp3",
		Bitrate:   "128k",
		Output:    filepath.Join(tmpDir, "{artist}/{album}/{title}.{output-ext}"),
		Overwrite: config.OverwriteSkip,
		MaxRetries: 1,
	}

	// Create real components
	spotifyConfig := &spotify.Config{
		ClientID:     "dummy", // Not used for YouTube downloads
		ClientSecret: "dummy",
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		t.Fatalf("Failed to create Spotify client: %v", err)
	}
	defer spotifyClient.Close()

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
		Bitrate:      "128k",
		AudioProviders: []string{"youtube"},
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		t.Fatalf("Failed to create audio provider: %v", err)
	}

	metadataEmbedder := metadata.NewEmbedder()
	downloader := NewDownloader(cfg, spotifyClient, audioProvider, metadataEmbedder)

	// Create PlanItem with YouTube URL
	// Using a short, well-known test video
	item := &plan.PlanItem{
		ItemID:     "track:youtube:test",
		ItemType:   plan.PlanItemTypeTrack,
		YouTubeURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", // Short test video
		Name:       "Test Video",
		Status:     plan.PlanItemStatusPending,
		Metadata:   make(map[string]interface{}),
	}

	ctx := context.Background()
	success, filePath, err := downloader.DownloadTrack(ctx, item)

	if err != nil {
		t.Fatalf("DownloadTrack() failed: %v", err)
	}
	if !success {
		t.Error("Expected download to succeed")
	}
	if filePath == "" {
		t.Error("Expected file path to be returned")
	}

	// Verify file was created
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("Expected downloaded file to exist: %v", err)
	}

	// Verify file is not empty
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if fileInfo.Size() == 0 {
		t.Error("Expected downloaded file to not be empty")
	}
}

func TestIntegration_YouTubeVideoDownload_WithSpotifyEnhancement(t *testing.T) {
	checkYtDlpAvailable(t)

	// Load Spotify credentials for enhancement
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		clientID = os.Getenv("SPOTIGO_CLIENT_ID")
	}
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("SPOTIGO_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		t.Skip("Spotify credentials required for enhancement test")
	}

	tmpDir := t.TempDir()
	cfg := &config.DownloadSettings{
		Format:    "mp3",
		Bitrate:   "128k",
		Output:    filepath.Join(tmpDir, "{artist}/{album}/{title}.{output-ext}"),
		Overwrite: config.OverwriteSkip,
		MaxRetries: 1,
	}

	// Create real components
	spotifyConfig := &spotify.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		t.Fatalf("Failed to create Spotify client: %v", err)
	}
	defer spotifyClient.Close()

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
		Bitrate:      "128k",
		AudioProviders: []string{"youtube"},
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		t.Fatalf("Failed to create audio provider: %v", err)
	}

	metadataEmbedder := metadata.NewEmbedder()
	downloader := NewDownloader(cfg, spotifyClient, audioProvider, metadataEmbedder)

	// Create PlanItem - the generator would have added enhancement, but for this test
	// we'll simulate it by adding enhancement metadata manually
	item := &plan.PlanItem{
		ItemID:     "track:youtube:test",
		ItemType:   plan.PlanItemTypeTrack,
		YouTubeURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Name:       "Never Gonna Give You Up",
		Status:     plan.PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"youtube_metadata": &audio.YouTubeVideoMetadata{
				VideoID:  "dQw4w9WgXcQ",
				Title:    "Never Gonna Give You Up",
				Uploader: "Rick Astley",
				Duration: 213,
			},
			// Simulate Spotify enhancement (normally added by generator)
			"spotify_enhancement": map[string]interface{}{
				"album":        "Whenever You Need Somebody",
				"artist":       "Rick Astley",
				"track_number": 1,
				"year":         1987,
			},
		},
	}

	ctx := context.Background()
	success, filePath, err := downloader.DownloadTrack(ctx, item)

	if err != nil {
		t.Fatalf("DownloadTrack() failed: %v", err)
	}
	if !success {
		t.Error("Expected download to succeed")
	}
	if filePath == "" {
		t.Error("Expected file path to be returned")
	}

	// Verify file path uses enhanced metadata
	if !strings.Contains(filePath, "Rick Astley") {
		t.Errorf("Expected file path to contain artist name, got: %s", filePath)
	}
	if !strings.Contains(filePath, "Whenever You Need Somebody") {
		t.Errorf("Expected file path to contain album name, got: %s", filePath)
	}
}

func TestIntegration_YouTubePlaylistDownload(t *testing.T) {
	checkYtDlpAvailable(t)

	tmpDir := t.TempDir()
	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:     "dummy",
			ClientSecret: "dummy",
			Format:       "mp3",
			Bitrate:      "128k",
			Output:       filepath.Join(tmpDir, "{artist}/{album}/{title}.{output-ext}"),
			Threads:      2,
			MaxRetries:   1,
		},
		Playlists: []config.MusicSource{
			// Use a small test playlist
			{Name: "Test Playlist", URL: "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf"},
		},
	}

	// Create real components
	spotifyConfig := &spotify.Config{
		ClientID:     "dummy",
		ClientSecret: "dummy",
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		t.Fatalf("Failed to create Spotify client: %v", err)
	}
	defer spotifyClient.Close()

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
		Bitrate:      "128k",
		AudioProviders: []string{"youtube"},
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		t.Fatalf("Failed to create audio provider: %v", err)
	}

	metadataEmbedder := metadata.NewEmbedder()
	downloader := NewDownloader(&cfg.Download, spotifyClient, audioProvider, metadataEmbedder)

	// Create generator
	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return nil, nil
	}
	generator := plan.NewGenerator(cfg, spotifyClient, playlistTracksFunc, audioProvider)

	// Generate plan
	ctx := context.Background()
	downloadPlan, err := generator.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("GeneratePlan() failed: %v", err)
	}

	// Verify playlist and tracks were created
	playlistCount := 0
	trackCount := 0
	for _, item := range downloadPlan.Items {
		if item.ItemType == plan.PlanItemTypePlaylist {
			playlistCount++
		}
		if item.ItemType == plan.PlanItemTypeTrack {
			trackCount++
		}
	}

	if playlistCount != 1 {
		t.Errorf("Expected 1 playlist, got %d", playlistCount)
	}
	if trackCount == 0 {
		t.Error("Expected at least 1 track in playlist")
	}

	// Execute plan (download tracks)
	executor := plan.NewExecutor(downloader, 2)
	stats, err := executor.Execute(ctx, downloadPlan, func(item *plan.PlanItem) {
		// Progress callback
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// Verify some tracks were downloaded
	if stats["completed"] == 0 {
		t.Error("Expected at least 1 completed track")
	}
}
