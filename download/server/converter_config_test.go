package server

import (
	"testing"

	"github.com/sv4u/musicdl/download/proto"
)

func TestConvertProtoConfigToConfig(t *testing.T) {
	protoConfig := &proto.Config{
		Version: "1.2",
		Download: &proto.DownloadSettings{
			ClientId:     "test_client_id",
			ClientSecret: "test_client_secret",
			Threads:      4,
			MaxRetries:   3,
			Format:       "mp3",
			Bitrate:      "128k",
			Output:       "{artist}/{album}/{title}.{ext}",
			AudioProviders: []string{"youtube-music"},
			Overwrite:    "skip",
			CacheMaxSize: 1000,
			CacheTtl:     3600,
			SpotifyRateLimitEnabled:  true,
			SpotifyRateLimitRequests: 10,
			SpotifyRateLimitWindow:   1.0,
			DownloadRateLimitEnabled:  true,
			DownloadRateLimitRequests: 2,
			DownloadRateLimitWindow:   1.0,
			PlanGenerationEnabled:      true,
			PlanOptimizationEnabled:    true,
			PlanExecutionEnabled:       true,
			PlanPersistenceEnabled:     true,
			PlanStatusReportingEnabled: true,
		},
		Ui: &proto.UISettings{
			HistoryPath:      "/path/to/history",
			HistoryRetention: 10,
			SnapshotInterval: 10,
			LogPath:          "/path/to/log",
		},
		Songs: []*proto.MusicSource{
			{Name: "Song 1", Url: "https://open.spotify.com/track/123", CreateM3U: false},
		},
		Artists: []*proto.MusicSource{
			{Name: "Artist 1", Url: "https://open.spotify.com/artist/456", CreateM3U: false},
		},
		Playlists: []*proto.MusicSource{
			{Name: "Playlist 1", Url: "https://open.spotify.com/playlist/789", CreateM3U: true},
		},
		Albums: []*proto.MusicSource{
			{Name: "Album 1", Url: "https://open.spotify.com/album/012", CreateM3U: true},
		},
	}

	cfg, err := convertProtoConfigToConfig(protoConfig)
	if err != nil {
		t.Fatalf("convertProtoConfigToConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("convertProtoConfigToConfig returned nil")
	}

	// Verify basic fields
	if cfg.Version != "1.2" {
		t.Errorf("expected version 1.2, got %s", cfg.Version)
	}

	// Verify download settings
	if cfg.Download.ClientID != "test_client_id" {
		t.Errorf("expected ClientID 'test_client_id', got '%s'", cfg.Download.ClientID)
	}

	if cfg.Download.Threads != 4 {
		t.Errorf("expected Threads 4, got %d", cfg.Download.Threads)
	}

	if cfg.Download.Format != "mp3" {
		t.Errorf("expected Format 'mp3', got '%s'", cfg.Download.Format)
	}

	// Verify UI settings
	if cfg.UI.HistoryPath != "/path/to/history" {
		t.Errorf("expected HistoryPath '/path/to/history', got '%s'", cfg.UI.HistoryPath)
	}

	// Verify music sources
	if len(cfg.Songs) != 1 {
		t.Errorf("expected 1 song, got %d", len(cfg.Songs))
	} else if cfg.Songs[0].Name != "Song 1" {
		t.Errorf("expected song name 'Song 1', got '%s'", cfg.Songs[0].Name)
	}

	if len(cfg.Artists) != 1 {
		t.Errorf("expected 1 artist, got %d", len(cfg.Artists))
	}

	if len(cfg.Playlists) != 1 {
		t.Errorf("expected 1 playlist, got %d", len(cfg.Playlists))
	}

	if len(cfg.Albums) != 1 {
		t.Errorf("expected 1 album, got %d", len(cfg.Albums))
	}
}

func TestConvertProtoConfigToConfig_Nil(t *testing.T) {
	_, err := convertProtoConfigToConfig(nil)
	if err == nil {
		t.Error("expected error for nil config, got nil")
	}
}

func TestConvertProtoMusicSources(t *testing.T) {
	protoSources := []*proto.MusicSource{
		{Name: "Source 1", Url: "https://example.com/1", CreateM3U: false},
		{Name: "Source 2", Url: "https://example.com/2", CreateM3U: true},
		nil, // Test nil handling
		{Name: "Source 3", Url: "https://example.com/3", CreateM3U: false},
	}

	result := convertProtoMusicSources(protoSources)

	// Should have 3 sources (nil is skipped)
	if len(result) != 3 {
		t.Errorf("expected 3 sources, got %d", len(result))
	}

	if result[0].Name != "Source 1" {
		t.Errorf("expected first source name 'Source 1', got '%s'", result[0].Name)
	}

	if !result[1].CreateM3U {
		t.Error("expected second source CreateM3U to be true")
	}

	if result[2].Name != "Source 3" {
		t.Errorf("expected third source name 'Source 3', got '%s'", result[2].Name)
	}
}

func TestConvertProtoMusicSources_Empty(t *testing.T) {
	result := convertProtoMusicSources([]*proto.MusicSource{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}
