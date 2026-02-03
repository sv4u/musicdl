package audio

import (
	"testing"
)

func TestParsePlaylistEntry(t *testing.T) {
	tests := []struct {
		name      string
		entry     map[string]interface{}
		wantNil   bool
		wantID    string
		wantTitle string
	}{
		{
			name: "valid video entry",
			entry: map[string]interface{}{
				"id":          "dQw4w9WgXcQ",
				"title":       "Test Video",
				"duration":    float64(212),
				"uploader":    "Channel",
				"webpage_url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			},
			wantNil:   false,
			wantID:    "dQw4w9WgXcQ",
			wantTitle: "Test Video",
		},
		{
			name: "minimal entry with id only",
			entry: map[string]interface{}{
				"id": "abc123xyz01",
			},
			wantNil:   false,
			wantID:    "abc123xyz01",
			wantTitle: "",
		},
		{
			name:    "missing id returns nil",
			entry:   map[string]interface{}{"title": "No ID"},
			wantNil: true,
		},
		{
			name:    "empty id returns nil",
			entry:   map[string]interface{}{"id": ""},
			wantNil: true,
		},
		{
			name: "playlist type skipped",
			entry: map[string]interface{}{
				"id":    "PLplaylist123",
				"_type": "playlist",
				"title": "Nested Playlist",
			},
			wantNil: true,
		},
		{
			name: "duration as int",
			entry: map[string]interface{}{
				"id":       "vid123",
				"duration": 300,
			},
			wantNil: false,
			wantID:  "vid123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePlaylistEntry(tt.entry)
			if tt.wantNil {
				if got != nil {
					t.Errorf("parsePlaylistEntry() expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("parsePlaylistEntry() returned nil, expected result")
			}
			if got.VideoID != tt.wantID {
				t.Errorf("VideoID = %q, want %q", got.VideoID, tt.wantID)
			}
			if tt.wantTitle != "" && got.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tt.wantTitle)
			}
		})
	}
}

func TestParsePlaylistOutput(t *testing.T) {
	// Simulates yt-dlp --flat-playlist --dump-json: first line = playlist, rest = videos
	multiLineOutput := `{"id": "PLtest123", "title": "Test Playlist", "uploader": "Uploader", "playlist_count": 2}
{"id": "video1id12345", "title": "First Video", "duration": 120, "webpage_url": "https://www.youtube.com/watch?v=video1id12345"}
{"id": "video2id67890", "title": "Second Video", "duration": 240}
`
	info, err := parsePlaylistOutput(multiLineOutput)
	if err != nil {
		t.Fatalf("parsePlaylistOutput() error = %v", err)
	}
	if info.PlaylistID != "PLtest123" {
		t.Errorf("PlaylistID = %q, want PLtest123", info.PlaylistID)
	}
	if info.Title != "Test Playlist" {
		t.Errorf("Title = %q, want Test Playlist", info.Title)
	}
	if info.Uploader != "Uploader" {
		t.Errorf("Uploader = %q, want Uploader", info.Uploader)
	}
	if len(info.Entries) != 2 {
		t.Fatalf("Entries length = %d, want 2", len(info.Entries))
	}
	if info.Entries[0].VideoID != "video1id12345" || info.Entries[0].Title != "First Video" {
		t.Errorf("Entries[0] = %+v", info.Entries[0])
	}
	if info.Entries[1].VideoID != "video2id67890" || info.Entries[1].Title != "Second Video" {
		t.Errorf("Entries[1] = %+v", info.Entries[1])
	}
}

func TestParsePlaylistOutput_EmptyOutput(t *testing.T) {
	_, err := parsePlaylistOutput("")
	if err == nil {
		t.Error("parsePlaylistOutput(empty) expected error, got nil")
	}
}

func TestParsePlaylistOutput_FirstLineOnlyPlaylist(t *testing.T) {
	// Only playlist metadata line (no video lines) — Entries should be empty
	output := `{"id": "PLonly", "title": "Playlist Only", "_type": "playlist"}`
	info, err := parsePlaylistOutput(output)
	if err != nil {
		t.Fatalf("parsePlaylistOutput() error = %v", err)
	}
	if info.PlaylistID != "PLonly" || info.Title != "Playlist Only" {
		t.Errorf("Playlist metadata: %+v", info)
	}
	if len(info.Entries) != 0 {
		t.Errorf("Entries length = %d, want 0 (no video lines)", len(info.Entries))
	}
}

func TestParsePlaylistOutput_InlineEntries(t *testing.T) {
	// Some yt-dlp versions return playlist with nested "entries" array in first line
	output := `{"id": "PLinline", "title": "With Inline Entries", "entries": [{"id": "v1", "title": "V1"}, {"id": "v2", "title": "V2"}]}`
	info, err := parsePlaylistOutput(output)
	if err != nil {
		t.Fatalf("parsePlaylistOutput() error = %v", err)
	}
	if info.PlaylistID != "PLinline" {
		t.Errorf("PlaylistID = %q, want PLinline", info.PlaylistID)
	}
	if len(info.Entries) != 2 {
		t.Fatalf("Entries length = %d, want 2 (from inline entries)", len(info.Entries))
	}
	if info.Entries[0].VideoID != "v1" || info.Entries[1].VideoID != "v2" {
		t.Errorf("Entries = %+v", info.Entries)
	}
}
