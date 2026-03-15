package audius

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBaseURL    = "https://discoveryprovider.audius.co"
	apiPrefix         = "/v1"
	defaultTimeout    = 15 * time.Second
	defaultMaxResults = 5
)

// Track represents an Audius track from the search API.
type Track struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	User        User   `json:"user"`
	Duration    int    `json:"duration"`
	Permalink   string `json:"permalink"`
	Description string `json:"description,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Mood        string `json:"mood,omitempty"`
	Artwork     *struct {
		URL150x150   string `json:"150x150,omitempty"`
		URL480x480   string `json:"480x480,omitempty"`
		URL1000x1000 string `json:"1000x1000,omitempty"`
	} `json:"artwork,omitempty"`
}

// User represents an Audius user.
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Handle   string `json:"handle"`
	Verified bool   `json:"is_verified"`
}

// TrackURL returns the full Audius URL for this track.
func (t *Track) TrackURL() string {
	if t.User.Handle != "" && t.Permalink != "" {
		return fmt.Sprintf("https://audius.co/%s/%s", t.User.Handle, t.Permalink)
	}
	return ""
}

type searchResponse struct {
	Data []Track `json:"data"`
}

// Client is a lightweight REST client for the Audius discovery provider API.
// No authentication is required.
type Client struct {
	baseURL    string
	httpClient *http.Client
	maxResults int
}

// NewClient creates a new Audius API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		maxResults: defaultMaxResults,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Option configures the Audius client.
type Option func(*Client)

// WithBaseURL overrides the default discovery provider URL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithMaxResults sets the maximum number of search results.
func WithMaxResults(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxResults = n
		}
	}
}

// SearchTracks searches for tracks matching the query string.
func (c *Client) SearchTracks(ctx context.Context, query string) ([]Track, error) {
	params := url.Values{}
	params.Set("query", query)

	endpoint := fmt.Sprintf("%s%s/tracks/search?%s", c.baseURL, apiPrefix, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("audius: failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("audius: search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("audius: search returned status %d: %s", resp.StatusCode, string(body))
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("audius: failed to decode search response: %w", err)
	}

	if c.maxResults > 0 && len(result.Data) > c.maxResults {
		result.Data = result.Data[:c.maxResults]
	}

	return result.Data, nil
}

// SearchBestMatch searches for tracks and returns the best match URL.
// Returns (audiusTrackURL, error). Empty string if no results found.
func (c *Client) SearchBestMatch(ctx context.Context, query string) (string, error) {
	tracks, err := c.SearchTracks(ctx, query)
	if err != nil {
		return "", err
	}
	if len(tracks) == 0 {
		return "", nil
	}
	trackURL := tracks[0].TrackURL()
	if trackURL == "" {
		return "", fmt.Errorf("audius: track has no URL (id=%s title=%s)", tracks[0].ID, tracks[0].Title)
	}
	return trackURL, nil
}
