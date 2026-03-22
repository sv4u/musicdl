package plex

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	plexProduct          = "musicdl"
	plexVersion          = "1.0"
	plexClientIdentifier = "musicdl-plex-sync"
	clientTimeout        = 30 * time.Second
)

// Client is an HTTP client for the Plex Media Server REST API.
type Client struct {
	serverURL  string
	token      string
	httpClient *http.Client
}

// LibrarySection represents a Plex library section (e.g. Music, Movies).
type LibrarySection struct {
	Key   string // section ID
	Title string
	Type  string // "artist" for music
}

// Playlist represents a Plex playlist.
type Playlist struct {
	RatingKey    string
	Title        string
	PlaylistType string // "audio", "video", "photo"
	LeafCount    int
	Smart        bool
}

// SyncResult records the outcome of syncing a single playlist.
type SyncResult struct {
	PlaylistName string `json:"playlistName"`
	M3UPath      string `json:"m3uPath"`
	Action       string `json:"action"`
	Error        string `json:"error,omitempty"`
	TrackCount   int    `json:"trackCount,omitempty"`
}

// XML response structures for Plex API deserialization.
type sectionsContainer struct {
	XMLName    xml.Name         `xml:"MediaContainer"`
	Directories []sectionEntry `xml:"Directory"`
}

type sectionEntry struct {
	Key   string `xml:"key,attr"`
	Title string `xml:"title,attr"`
	Type  string `xml:"type,attr"`
}

type playlistsContainer struct {
	XMLName   xml.Name        `xml:"MediaContainer"`
	Playlists []playlistEntry `xml:"Playlist"`
}

type playlistEntry struct {
	RatingKey    string `xml:"ratingKey,attr"`
	Title        string `xml:"title,attr"`
	PlaylistType string `xml:"playlistType,attr"`
	LeafCount    int    `xml:"leafCount,attr"`
	Smart        string `xml:"smart,attr"`
}

// NewClient creates a new Plex API client with a 30-second timeout.
func NewClient(serverURL, token string) *Client {
	return &Client{
		serverURL: strings.TrimRight(serverURL, "/"),
		token:     token,
		httpClient: &http.Client{
			Timeout: clientTimeout,
		},
	}
}

// TestConnection verifies the Plex server is reachable by hitting GET /identity.
func (c *Client) TestConnection() error {
	resp, err := c.doRequest(http.MethodGet, "/identity", nil)
	if err != nil {
		return fmt.Errorf("plex: connection test failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("plex: server returned status %d on identity check", resp.StatusCode)
	}
	return nil
}

// GetSections returns all library sections from the Plex server.
func (c *Client) GetSections() ([]LibrarySection, error) {
	resp, err := c.doRequest(http.MethodGet, "/library/sections", nil)
	if err != nil {
		return nil, fmt.Errorf("plex: failed to get sections: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("plex: failed to read sections response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex: get sections returned status %d: %s", resp.StatusCode, string(body))
	}
	var container sectionsContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("plex: failed to parse sections XML: %w", err)
	}
	sections := make([]LibrarySection, len(container.Directories))
	for i, d := range container.Directories {
		sections[i] = LibrarySection{Key: d.Key, Title: d.Title, Type: d.Type}
	}
	return sections, nil
}

// FindMusicSectionID returns the key of the first library section with type "artist".
func (c *Client) FindMusicSectionID() (string, error) {
	sections, err := c.GetSections()
	if err != nil {
		return "", err
	}
	for _, s := range sections {
		if s.Type == "artist" {
			return s.Key, nil
		}
	}
	return "", fmt.Errorf("plex: no music library section found (type=artist)")
}

// GetPlaylists returns all playlists from the Plex server.
func (c *Client) GetPlaylists() ([]Playlist, error) {
	resp, err := c.doRequest(http.MethodGet, "/playlists", nil)
	if err != nil {
		return nil, fmt.Errorf("plex: failed to get playlists: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("plex: failed to read playlists response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex: get playlists returned status %d: %s", resp.StatusCode, string(body))
	}
	var container playlistsContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("plex: failed to parse playlists XML: %w", err)
	}
	playlists := make([]Playlist, len(container.Playlists))
	for i, p := range container.Playlists {
		playlists[i] = Playlist{
			RatingKey:    p.RatingKey,
			Title:        p.Title,
			PlaylistType: p.PlaylistType,
			LeafCount:    p.LeafCount,
			Smart:        p.Smart == "1",
		}
	}
	return playlists, nil
}

// FindPlaylistByTitle returns the first audio playlist matching the given title
// (case-insensitive), or nil if none found.
func (c *Client) FindPlaylistByTitle(title string) *Playlist {
	playlists, err := c.GetPlaylists()
	if err != nil {
		return nil
	}
	titleLower := strings.ToLower(title)
	for _, p := range playlists {
		if p.PlaylistType == "audio" && strings.ToLower(p.Title) == titleLower {
			return &p
		}
	}
	return nil
}

// DeletePlaylist deletes a playlist by its ratingKey.
func (c *Client) DeletePlaylist(ratingKey string) error {
	path := fmt.Sprintf("/playlists/%s", ratingKey)
	resp, err := c.doRequest(http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("plex: failed to delete playlist %s: %w", ratingKey, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("plex: delete playlist returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// UploadPlaylist uploads an M3U playlist file to the Plex server.
func (c *Client) UploadPlaylist(sectionID, m3uPath string) error {
	query := url.Values{}
	query.Set("sectionID", sectionID)
	query.Set("path", m3uPath)
	path := "/playlists/upload?" + query.Encode()
	resp, err := c.doRequest(http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("plex: failed to upload playlist: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("plex: upload playlist returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// doRequest builds and executes an HTTP request with Plex identification headers.
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	fullURL := c.serverURL + path
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Plex-Token", c.token)
	req.Header.Set("X-Plex-Product", plexProduct)
	req.Header.Set("X-Plex-Version", plexVersion)
	req.Header.Set("X-Plex-Client-Identifier", plexClientIdentifier)
	req.Header.Set("Accept", "application/xml")
	return c.httpClient.Do(req)
}
