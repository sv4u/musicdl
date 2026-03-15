package audius

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient_Defaults(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.baseURL != defaultBaseURL {
		t.Errorf("expected default base URL %q, got %q", defaultBaseURL, client.baseURL)
	}
	if client.maxResults != defaultMaxResults {
		t.Errorf("expected default max results %d, got %d", defaultMaxResults, client.maxResults)
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	client := NewClient(
		WithBaseURL("https://custom.example.com"),
		WithMaxResults(10),
	)
	if client.baseURL != "https://custom.example.com" {
		t.Errorf("expected custom base URL, got %q", client.baseURL)
	}
	if client.maxResults != 10 {
		t.Errorf("expected max results 10, got %d", client.maxResults)
	}
}

func TestTrack_TrackURL(t *testing.T) {
	tests := []struct {
		name     string
		track    Track
		expected string
	}{
		{
			name: "valid track",
			track: Track{
				ID:        "abc123",
				Permalink: "my-track",
				User:      User{Handle: "artist-name"},
			},
			expected: "https://audius.co/artist-name/my-track",
		},
		{
			name: "missing handle",
			track: Track{
				ID:        "abc123",
				Permalink: "my-track",
				User:      User{Handle: ""},
			},
			expected: "",
		},
		{
			name: "missing permalink",
			track: Track{
				ID:   "abc123",
				User: User{Handle: "artist-name"},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.track.TrackURL()
			if result != tt.expected {
				t.Errorf("TrackURL() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestSearchTracks_Success(t *testing.T) {
	tracks := []Track{
		{
			ID:        "track1",
			Title:     "Test Song",
			Permalink: "test-song",
			User:      User{Handle: "test-artist", Name: "Test Artist"},
			Duration:  200,
		},
		{
			ID:        "track2",
			Title:     "Another Song",
			Permalink: "another-song",
			User:      User{Handle: "test-artist", Name: "Test Artist"},
			Duration:  180,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tracks/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		query := r.URL.Query().Get("query")
		if query == "" {
			t.Error("missing query parameter")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResponse{Data: tracks})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	results, err := client.SearchTracks(context.Background(), "test query")
	if err != nil {
		t.Fatalf("SearchTracks() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "Test Song" {
		t.Errorf("expected first track title %q, got %q", "Test Song", results[0].Title)
	}
	if results[0].User.Handle != "test-artist" {
		t.Errorf("expected user handle %q, got %q", "test-artist", results[0].User.Handle)
	}
}

func TestSearchTracks_MaxResults(t *testing.T) {
	tracks := make([]Track, 10)
	for i := range tracks {
		tracks[i] = Track{ID: "id", Title: "track", Permalink: "track", User: User{Handle: "a"}}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResponse{Data: tracks})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithMaxResults(3))
	results, err := client.SearchTracks(context.Background(), "query")
	if err != nil {
		t.Fatalf("SearchTracks() error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results (maxResults), got %d", len(results))
	}
}

func TestSearchTracks_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResponse{Data: []Track{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	results, err := client.SearchTracks(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("SearchTracks() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchTracks_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.SearchTracks(context.Background(), "query")
	if err == nil {
		t.Fatal("SearchTracks() expected error for 500 response")
	}
}

func TestSearchTracks_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.SearchTracks(context.Background(), "query")
	if err == nil {
		t.Fatal("SearchTracks() expected error for invalid JSON")
	}
}

func TestSearchTracks_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.SearchTracks(ctx, "query")
	if err == nil {
		t.Fatal("SearchTracks() expected error for cancelled context")
	}
}

func TestSearchBestMatch_Success(t *testing.T) {
	tracks := []Track{
		{
			ID:        "track1",
			Title:     "Best Match",
			Permalink: "best-match",
			User:      User{Handle: "artist"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResponse{Data: tracks})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	url, err := client.SearchBestMatch(context.Background(), "best match")
	if err != nil {
		t.Fatalf("SearchBestMatch() error: %v", err)
	}
	expected := "https://audius.co/artist/best-match"
	if url != expected {
		t.Errorf("SearchBestMatch() = %q, expected %q", url, expected)
	}
}

func TestSearchBestMatch_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResponse{Data: []Track{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	url, err := client.SearchBestMatch(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("SearchBestMatch() unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty URL for no results, got %q", url)
	}
}

func TestSearchBestMatch_TrackMissingURL(t *testing.T) {
	tracks := []Track{
		{ID: "track1", Title: "No URL", User: User{Handle: ""}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResponse{Data: tracks})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.SearchBestMatch(context.Background(), "query")
	if err == nil {
		t.Fatal("SearchBestMatch() expected error when track has no URL")
	}
}
