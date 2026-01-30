package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sv4u/musicdl/download"
	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/spotify"
	"github.com/sv4u/spotigo"
)

// Exit codes for plan command (spec-aligned).
const (
	PlanExitSuccess     = 0
	PlanExitConfigError = 1
	PlanExitNetwork     = 2
	PlanExitFilesystem  = 3
)

// Exit codes for download command (spec-aligned).
const (
	DownloadExitSuccess     = 0
	DownloadExitConfigError = 1
	DownloadExitPlanMissing = 2
	DownloadExitNetwork     = 3
	DownloadExitFilesystem  = 4
	DownloadExitPartial     = 5
)

// getCacheDir returns MUSICDL_CACHE_DIR or ".cache" under current dir.
func getCacheDir() string {
	if d := os.Getenv("MUSICDL_CACHE_DIR"); d != "" {
		return d
	}
	return ".cache"
}

// planCommand runs the plan subcommand: load config, generate plan, save to .cache/download_plan_<hash>.json.
// Returns exit code.
func planCommand(configPath string) int {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if _, ok := err.(*config.ConfigError); ok {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			return PlanExitConfigError
		}
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return PlanExitConfigError
	}

	hash, err := config.HashFromPath(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error computing config hash: %v\n", err)
		return PlanExitFilesystem
	}

	cacheDir := getCacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating cache directory: %v\n", err)
		return PlanExitFilesystem
	}

	spotifyConfig := &spotify.Config{
		ClientID:          cfg.Download.ClientID,
		ClientSecret:      cfg.Download.ClientSecret,
		CacheMaxSize:      cfg.Download.CacheMaxSize,
		CacheTTL:          cfg.Download.CacheTTL,
		RateLimitEnabled:  cfg.Download.SpotifyRateLimitEnabled,
		RateLimitRequests: cfg.Download.SpotifyRateLimitRequests,
		RateLimitWindow:   cfg.Download.SpotifyRateLimitWindow,
		MaxRetries:        cfg.Download.SpotifyMaxRetries,
		RetryBaseDelay:    cfg.Download.SpotifyRetryBaseDelay,
		RetryMaxDelay:     cfg.Download.SpotifyRetryMaxDelay,
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Spotify client error: %v\n", err)
		return PlanExitNetwork
	}

	audioConfig := &audio.Config{
		OutputFormat:   cfg.Download.Format,
		Bitrate:        cfg.Download.Bitrate,
		AudioProviders: cfg.Download.AudioProviders,
		CacheMaxSize:   cfg.Download.AudioSearchCacheMaxSize,
		CacheTTL:       cfg.Download.AudioSearchCacheTTL,
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audio provider error: %v\n", err)
		return PlanExitConfigError
	}

	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return spotifyClient.GetPlaylistTracks(ctx, playlistID, opts)
	}
	generator := plan.NewGenerator(cfg, spotifyClient, playlistTracksFunc, audioProvider)
	optimizer := plan.NewOptimizer(true)

	ctx := context.Background()
	generatedPlan, err := generator.GeneratePlan(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Plan generation failed: %v\n", err)
		return PlanExitNetwork
	}

	optimizer.Optimize(generatedPlan)

	configFile := filepath.Base(configPath)
	if err := plan.SavePlanByHash(generatedPlan, cacheDir, hash, configFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving plan file: %v\n", err)
		return PlanExitFilesystem
	}

	trackCount := 0
	for _, item := range generatedPlan.Items {
		if item.ItemType == plan.PlanItemTypeTrack {
			trackCount++
		}
	}
	fmt.Printf("Plan generated successfully\n")
	fmt.Printf("Configuration: %s\n", configPath)
	fmt.Printf("Total tracks: %d\n", trackCount)
	fmt.Printf("Plan file: %s\n", plan.GetPlanFilePath(cacheDir, hash))
	return PlanExitSuccess
}

// downloadCLICommand runs the download subcommand: load config, load plan by hash, run executor.
// Returns exit code.
func downloadCLICommand(configPath string) int {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if _, ok := err.(*config.ConfigError); ok {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			return DownloadExitConfigError
		}
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return DownloadExitConfigError
	}

	hash, err := config.HashFromPath(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error computing config hash: %v\n", err)
		return DownloadExitFilesystem
	}

	cacheDir := getCacheDir()
	loadedPlan, err := plan.LoadPlanByHash(cacheDir, hash)
	if err != nil {
		if errors.Is(err, plan.ErrPlanNotFound) {
			fmt.Fprintf(os.Stderr, "Plan file not found. Run 'musicdl plan %s' first.\n", configPath)
			return DownloadExitPlanMissing
		}
		if errors.Is(err, plan.ErrPlanHashMismatch) {
			fmt.Fprintf(os.Stderr, "Plan file does not match configuration. Regenerate plan.\n")
			return DownloadExitPlanMissing
		}
		fmt.Fprintf(os.Stderr, "Error loading plan: %v\n", err)
		return DownloadExitPlanMissing
	}

	spotifyConfig := &spotify.Config{
		ClientID:          cfg.Download.ClientID,
		ClientSecret:      cfg.Download.ClientSecret,
		CacheMaxSize:      cfg.Download.CacheMaxSize,
		CacheTTL:          cfg.Download.CacheTTL,
		RateLimitEnabled:  cfg.Download.SpotifyRateLimitEnabled,
		RateLimitRequests: cfg.Download.SpotifyRateLimitRequests,
		RateLimitWindow:   cfg.Download.SpotifyRateLimitWindow,
		MaxRetries:        cfg.Download.SpotifyMaxRetries,
		RetryBaseDelay:    cfg.Download.SpotifyRetryBaseDelay,
		RetryMaxDelay:     cfg.Download.SpotifyRetryMaxDelay,
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Spotify client error: %v\n", err)
		return DownloadExitNetwork
	}

	audioConfig := &audio.Config{
		OutputFormat:   cfg.Download.Format,
		Bitrate:        cfg.Download.Bitrate,
		AudioProviders: cfg.Download.AudioProviders,
		CacheMaxSize:   cfg.Download.AudioSearchCacheMaxSize,
		CacheTTL:       cfg.Download.AudioSearchCacheTTL,
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audio provider error: %v\n", err)
		return DownloadExitConfigError
	}

	metadataEmbedder := metadata.NewEmbedder()
	downloader := download.NewDownloader(&cfg.Download, spotifyClient, audioProvider, metadataEmbedder)
	maxWorkers := cfg.Download.Threads
	if maxWorkers == 0 {
		maxWorkers = 4
	}
	executor := plan.NewExecutor(downloader, maxWorkers)

	ctx := context.Background()
	progressCallback := func(item *plan.PlanItem) {
		// Optional: print progress to stdout
		_ = item
	}
	stats, err := executor.Execute(ctx, loadedPlan, progressCallback)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		if strings.Contains(err.Error(), "network") || strings.Contains(err.Error(), "rate limit") {
			return DownloadExitNetwork
		}
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "no space") {
			return DownloadExitFilesystem
		}
		return DownloadExitNetwork
	}

	completed := stats["completed"]
	failed := stats["failed"]
	total := stats["total"]
	fmt.Printf("Download complete\n")
	fmt.Printf("Successful: %d\n", completed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Total: %d\n", total)

	if failed > 0 && completed > 0 {
		return DownloadExitPartial
	}
	if failed > 0 {
		return DownloadExitNetwork
	}
	return DownloadExitSuccess
}
