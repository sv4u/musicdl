package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads and validates configuration from a YAML file.
func LoadConfig(path string) (*MusicDLConfig, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Configuration file not found: %s", path),
		}
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Error reading configuration file: %v", err),
		}
	}

	// Parse YAML
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Error parsing YAML file: %v", err),
		}
	}

	// Validate version (handle both string and float from YAML)
	version := raw["version"]
	if version != "1.2" && fmt.Sprintf("%v", version) != "1.2" {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Invalid version: %v. Expected 1.2", version),
		}
	}
	// Normalize to string
	raw["version"] = "1.2"

	// Convert sources from various formats to list format
	if err := convertSources(raw); err != nil {
		return nil, err
	}

	// Unmarshal into struct
	var config MusicDLConfig
	// Re-marshal to YAML for proper struct unmarshaling
	yamlData, err := yaml.Marshal(raw)
	if err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Error converting config data: %v", err),
		}
	}

	if err := yaml.Unmarshal(yamlData, &config); err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Invalid configuration: %v", err),
		}
	}

	// Validate
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// convertSources converts songs/artists/playlists/albums from various formats to list format.
func convertSources(raw map[string]interface{}) error {
	// Convert songs, artists (simple format, no create_m3u)
	for _, key := range []string{"songs", "artists"} {
		if sources, ok := raw[key]; ok {
			converted, err := convertSourceList(sources, false)
			if err != nil {
				return fmt.Errorf("error converting %s: %w", key, err)
			}
			raw[key] = converted
		}
	}

	// Convert playlists (supports create_m3u and extended format)
	if playlists, ok := raw["playlists"]; ok {
		converted, err := convertSourceList(playlists, true)
		if err != nil {
			return fmt.Errorf("error converting playlists: %w", err)
		}
		raw["playlists"] = converted
	}

	// Convert albums (supports create_m3u)
	if albums, ok := raw["albums"]; ok {
		converted, err := convertAlbumList(albums)
		if err != nil {
			return fmt.Errorf("error converting albums: %w", err)
		}
		raw["albums"] = converted
	}

	return nil
}

// convertSourceList converts a source list from various formats to []MusicSource.
func convertSourceList(sources interface{}, allowM3U bool) ([]MusicSource, error) {
	if sources == nil {
		return []MusicSource{}, nil
	}

	result := []MusicSource{}

	switch v := sources.(type) {
	case []interface{}:
		// Handle list format: [{name: url}, ...] or [url, ...] or extended format
		for _, item := range v {
			switch itemVal := item.(type) {
			case map[string]interface{}:
				// Check if extended format: {name: "...", url: "...", create_m3u: true/false}
				// Extended format is supported for all source types, but create_m3u only for playlists
				if nameVal, hasName := itemVal["name"]; hasName {
					if urlVal, hasURL := itemVal["url"]; hasURL {
						name, ok := nameVal.(string)
						if !ok {
							return nil, fmt.Errorf("invalid name type in source: expected string")
						}
						url, ok := urlVal.(string)
						if !ok {
							return nil, fmt.Errorf("invalid URL type in source: expected string")
						}
						createM3U := false
						if allowM3U {
							if m3uVal, hasM3U := itemVal["create_m3u"]; hasM3U {
								if m3uBool, ok := m3uVal.(bool); ok {
									createM3U = m3uBool
								}
							}
						}
						result = append(result, MusicSource{
							Name:      name,
							URL:       url,
							CreateM3U: createM3U,
						})
						continue
					}
				}
				// Simple format: {name: url}
				if len(itemVal) != 1 {
					return nil, fmt.Errorf(
						"invalid source format: dict with %d keys. Use extended format with 'name' and 'url' keys, or simple format with single key-value pair",
						len(itemVal),
					)
				}
				for name, urlVal := range itemVal {
					url, ok := urlVal.(string)
					if !ok {
						return nil, fmt.Errorf("invalid URL type in source: expected string")
					}
					result = append(result, MusicSource{
						Name:      name,
						URL:       url,
						CreateM3U: false,
					})
				}
			case string:
				// Handle [url, ...] - use URL as name
				result = append(result, MusicSource{
					Name:      itemVal,
					URL:       itemVal,
					CreateM3U: false,
				})
			default:
				return nil, fmt.Errorf("invalid source item type: expected map or string")
			}
		}
	case map[string]interface{}:
		// Handle dict format: {name: url, ...}
		for name, urlVal := range v {
			url, ok := urlVal.(string)
			if !ok {
				return nil, fmt.Errorf("invalid URL type in source: expected string")
			}
			result = append(result, MusicSource{
				Name:      name,
				URL:       url,
				CreateM3U: false,
			})
		}
	default:
		return nil, fmt.Errorf("invalid sources format: expected list or dict")
	}

	return result, nil
}

// convertAlbumList converts an album list from various formats to []MusicSource.
func convertAlbumList(albums interface{}) ([]MusicSource, error) {
	if albums == nil {
		return []MusicSource{}, nil
	}

	result := []MusicSource{}

	switch v := albums.(type) {
	case []interface{}:
		// Handle list format
		for _, item := range v {
			switch itemVal := item.(type) {
			case map[string]interface{}:
				// Check if extended format: {name: "...", url: "...", create_m3u: true/false}
				if nameVal, hasName := itemVal["name"]; hasName {
					if urlVal, hasURL := itemVal["url"]; hasURL {
						name, ok := nameVal.(string)
						if !ok {
							return nil, fmt.Errorf("invalid name type in album: expected string")
						}
						url, ok := urlVal.(string)
						if !ok {
							return nil, fmt.Errorf("invalid URL type in album: expected string")
						}
						createM3U := false
						if m3uVal, hasM3U := itemVal["create_m3u"]; hasM3U {
							if m3uBool, ok := m3uVal.(bool); ok {
								createM3U = m3uBool
							}
						}
						result = append(result, MusicSource{
							Name:      name,
							URL:       url,
							CreateM3U: createM3U,
						})
						continue
					}
				}
				// Simple format: {name: url}
				if len(itemVal) != 1 {
					return nil, fmt.Errorf(
						"invalid album format: dict with %d keys. Use extended format with 'name' and 'url' keys, or simple format with single key-value pair",
						len(itemVal),
					)
				}
				for name, urlVal := range itemVal {
					url, ok := urlVal.(string)
					if !ok {
						return nil, fmt.Errorf("invalid URL type in album: expected string")
					}
					result = append(result, MusicSource{
						Name:      name,
						URL:       url,
						CreateM3U: false,
					})
				}
			case string:
				// Handle [url, ...] - use URL as name
				result = append(result, MusicSource{
					Name:      itemVal,
					URL:       itemVal,
					CreateM3U: false,
				})
			default:
				return nil, fmt.Errorf("invalid album item type: expected map or string")
			}
		}
	case map[string]interface{}:
		// Handle dict format: {name: url, ...} - simple format
		for name, urlVal := range v {
			url, ok := urlVal.(string)
			if !ok {
				return nil, fmt.Errorf("invalid URL type in album: expected string")
			}
			result = append(result, MusicSource{
				Name:      name,
				URL:       url,
				CreateM3U: false,
			})
		}
	default:
		return nil, fmt.Errorf("invalid albums format: expected list or dict")
	}

	return result, nil
}
