//go:build e2e

package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/spotify"
	"github.com/sv4u/spotigo"
)

func loadSpotifyCredentials(t *testing.T) (string, string) {
	// Try to load .env file (ignore error if it doesn't exist)
	_ = godotenv.Load()

	// Load credentials from environment (try both SPOTIFY_ and SPOTIGO_ prefixes)
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		clientID = os.Getenv("SPOTIGO_CLIENT_ID")
	}
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("SPOTIGO_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		t.Skip("SPOTIFY_CLIENT_ID/SPOTIGO_CLIENT_ID and SPOTIFY_CLIENT_SECRET/SPOTIGO_CLIENT_SECRET required for E2E tests")
	}

	return clientID, clientSecret
}

func TestE2E_SingleTrackDownload(t *testing.T) {
	clientID, clientSecret := loadSpotifyCredentials(t)

	tmpDir := t.TempDir()
	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Format:       "mp3",
			Bitrate:      "128k",
			Output:       tmpDir + "/{artist}/{album}/{title}.{output-ext}",
			MaxRetries:   2,
			Overwrite:    config.OverwriteSkip,
		},
		Songs: []config.MusicSource{
			{Name: "YYZ", URL: "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"}, // Rush - YYZ
		},
	}

	// Create real Spotify client
	spotifyConfig := &spotify.Config{
		ClientID:            clientID,
		ClientSecret:        clientSecret,
		CacheMaxSize:        100,
		CacheTTL:            3600,
		RateLimitEnabled:    true,
		RateLimitRequests:    10,
		RateLimitWindow:     1.0,
		CacheCleanupInterval: 5 * time.Minute,
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		t.Fatalf("Failed to create Spotify client: %v", err)
	}
	defer spotifyClient.Close()

	// Create audio provider
	audioConfig := &audio.Config{
		OutputFormat: "mp3",
		Bitrate:      "128k",
		AudioProviders: []string{"youtube-music"},
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		t.Fatalf("Failed to create audio provider: %v", err)
	}

	// Create metadata embedder
	metadataEmbedder := metadata.NewEmbedder()

	// Create downloader
	downloader := NewDownloader(&cfg.Download, spotifyClient, audioProvider, metadataEmbedder)

	// Create executor
	executor := plan.NewExecutor(downloader, 1)

	// Create generator
	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return spotifyClient.GetPlaylistTracks(ctx, playlistID, opts)
	}
	generator := plan.NewGenerator(cfg, spotifyClient, playlistTracksFunc, audioProvider)

	// Create optimizer
	optimizer := plan.NewOptimizer(true)

	ctx := context.Background()

	// Step 1: Generate plan
	downloadPlan, err := generator.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("GeneratePlan() failed: %v", err)
	}

	if len(downloadPlan.Items) == 0 {
		t.Fatal("Expected at least 1 item in plan")
	}

	// Step 2: Optimize plan
	optimizer.Optimize(downloadPlan)

	// Step 3: Execute plan
	stats, err := executor.Execute(ctx, downloadPlan, func(item *plan.PlanItem) {
		// Progress callback
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// Verify execution stats
	if stats["completed"] < 1 {
		t.Errorf("Expected at least 1 completed track, got %d", stats["completed"])
	}

	// Verify file was created
	var trackItem *plan.PlanItem
	for _, item := range downloadPlan.Items {
		if item.ItemType == plan.PlanItemTypeTrack && item.Status == plan.PlanItemStatusCompleted {
			trackItem = item
			break
		}
	}

	if trackItem == nil {
		t.Fatal("Expected at least one completed track item")
	}

	if trackItem.FilePath == "" {
		t.Error("Expected track to have file path")
	}

	// Verify file exists
	if _, err := os.Stat(trackItem.FilePath); err != nil {
		t.Errorf("Expected downloaded file to exist: %v", err)
	}

	// Verify file is not empty
	fileInfo, err := os.Stat(trackItem.FilePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if fileInfo.Size() == 0 {
		t.Error("Expected downloaded file to not be empty")
	}
}

func TestE2E_PlanWorkflow_WithRealSpotify(t *testing.T) {
	clientID, clientSecret := loadSpotifyCredentials(t)

	tmpDir := t.TempDir()
	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Format:       "mp3",
			Bitrate:      "128k",
			Output:       tmpDir + "/{artist}/{album}/{title}.{output-ext}",
			MaxRetries:   2,
			Overwrite:    config.OverwriteSkip,
			Threads:      2,
		},
		Songs: []config.MusicSource{
			{Name: "YYZ", URL: "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"}, // Rush - YYZ
		},
	}

	// Create real components
	spotifyConfig := &spotify.Config{
		ClientID:            clientID,
		ClientSecret:        clientSecret,
		CacheMaxSize:        100,
		CacheTTL:            3600,
		RateLimitEnabled:    true,
		RateLimitRequests:    10,
		RateLimitWindow:     1.0,
		CacheCleanupInterval: 5 * time.Minute,
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		t.Fatalf("Failed to create Spotify client: %v", err)
	}
	defer spotifyClient.Close()

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
		Bitrate:      "128k",
		AudioProviders: []string{"youtube-music"},
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		t.Fatalf("Failed to create audio provider: %v", err)
	}

	metadataEmbedder := metadata.NewEmbedder()
	downloader := NewDownloader(&cfg.Download, spotifyClient, audioProvider, metadataEmbedder)
	executor := plan.NewExecutor(downloader, cfg.Download.Threads)

	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return spotifyClient.GetPlaylistTracks(ctx, playlistID, opts)
	}
	generator := plan.NewGenerator(cfg, spotifyClient, playlistTracksFunc, audioProvider)
	optimizer := plan.NewOptimizer(true)

	ctx := context.Background()

	// Full workflow: Generate → Optimize → Execute
	downloadPlan, err := generator.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("GeneratePlan() failed: %v", err)
	}

	optimizer.Optimize(downloadPlan)

	stats, err := executor.Execute(ctx, downloadPlan, func(item *plan.PlanItem) {
		// Progress callback
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// Verify results
	if stats["completed"] < 1 {
		t.Errorf("Expected at least 1 completed track, got %d", stats["completed"])
	}

	// Verify files were created
	filesCreated := 0
	for _, item := range downloadPlan.Items {
		if item.ItemType == plan.PlanItemTypeTrack && item.Status == plan.PlanItemStatusCompleted {
			if item.FilePath != "" {
				if _, err := os.Stat(item.FilePath); err == nil {
					filesCreated++
				}
			}
		}
	}

	if filesCreated < 1 {
		t.Error("Expected at least 1 file to be created")
	}
}

func TestE2E_PlanPersistence_WithRealDownload(t *testing.T) {
	clientID, clientSecret := loadSpotifyCredentials(t)

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")

	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Format:       "mp3",
			Bitrate:      "128k",
			Output:       tmpDir + "/downloads/{artist}/{album}/{title}.{output-ext}",
			MaxRetries:   2,
			Overwrite:    config.OverwriteSkip,
			Threads:      1,
		},
		Songs: []config.MusicSource{
			{Name: "YYZ", URL: "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"}, // Rush - YYZ
		},
	}

	// Create real components
	spotifyConfig := &spotify.Config{
		ClientID:            clientID,
		ClientSecret:        clientSecret,
		CacheMaxSize:        100,
		CacheTTL:            3600,
		RateLimitEnabled:    true,
		RateLimitRequests:    10,
		RateLimitWindow:     1.0,
		CacheCleanupInterval: 5 * time.Minute,
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		t.Fatalf("Failed to create Spotify client: %v", err)
	}
	defer spotifyClient.Close()

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
		Bitrate:      "128k",
		AudioProviders: []string{"youtube-music"},
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		t.Fatalf("Failed to create audio provider: %v", err)
	}

	metadataEmbedder := metadata.NewEmbedder()
	downloader := NewDownloader(&cfg.Download, spotifyClient, audioProvider, metadataEmbedder)

	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return spotifyClient.GetPlaylistTracks(ctx, playlistID, opts)
	}
	generator := plan.NewGenerator(cfg, spotifyClient, playlistTracksFunc, audioProvider)
	optimizer := plan.NewOptimizer(true)

	ctx := context.Background()

	// Generate and optimize plan
	downloadPlan, err := generator.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("GeneratePlan() failed: %v", err)
	}

	optimizer.Optimize(downloadPlan)

	// Save plan
	if err := downloadPlan.Save(planPath); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("Plan file was not created: %v", err)
	}

	// Load plan
	loadedPlan, err := plan.LoadPlan(planPath)
	if err != nil {
		t.Fatalf("LoadPlan() failed: %v", err)
	}

	if len(loadedPlan.Items) != len(downloadPlan.Items) {
		t.Errorf("Expected %d items in loaded plan, got %d", len(downloadPlan.Items), len(loadedPlan.Items))
	}

	// Execute loaded plan
	executor := plan.NewExecutor(downloader, 1)
	stats, err := executor.Execute(ctx, loadedPlan, func(item *plan.PlanItem) {
		// Progress callback
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if stats["completed"] < 1 {
		t.Errorf("Expected at least 1 completed track, got %d", stats["completed"])
	}
}
