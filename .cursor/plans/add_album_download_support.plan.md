# Plan: Add Support for Downloading Albums

## Executive Summary

This plan outlines the addition of album download support as a top-level configuration option, similar to songs, artists, and playlists. Currently, albums can only be downloaded indirectly through artist downloads. This plan adds direct album download support with per-album M3U playlist file creation configuration, allowing users to download specific albums directly from their configuration file.**Target Audience**: Technical leads, junior engineers, and technical management**Estimated Effort**: 2-3 days**Risk Level**: Low**Priority**: Medium (enhances user experience and feature parity)

## Current State Analysis

### Current Implementation

#### Album Download Method

The `download_album()` method already exists in `core/downloader.py`:

```python
def download_album(self, album_url: str) -> List[Tuple[bool, Optional[Path]]]:
    """Download all tracks in an album."""
    try:
        album_data = self.spotify.get_album(album_url)
        tracks = []
        
        # Get all tracks (handle pagination)
        items = album_data["tracks"]["items"]
        tracks_obj = album_data["tracks"]
        while tracks_obj.get("next"):
            next_data = self.spotify.client.next(tracks_obj)
            items.extend(next_data["items"])
            tracks_obj = next_data
        
        logger.info(f"Found {len(items)} tracks in album: {album_data['name']}")
        
        for track_item in items:
            track_id = track_item["id"]
            track_url = f"https://open.spotify.com/track/{track_id}"
            result = self.download_track(track_url)
            tracks.append(result)
        
        return tracks
```

**Status**: Method exists and works, but is only used internally by `download_artist()`.

#### Configuration Structure

Current config supports:

- `songs`: List of `MusicSource` objects
- `artists`: List of `MusicSource` objects
- `playlists`: List of `MusicSource` objects
- **Missing**: `albums` section

#### MusicSource Model

```python
class MusicSource(BaseModel):
    """Music source entry."""
    name: str
    url: str
```

**Limitation**: Only supports `name` and `url`, no additional options like `create_m3u`.

### Current Usage

Albums are currently downloaded only through:

1. **Artist Downloads**: `download_artist()` calls `download_album()` for each album
2. **No Direct Access**: Users cannot specify albums directly in config

### Existing Tests

There are already E2E tests for album downloads:

- `tests/e2e/test_album_download.py` - Tests album download functionality
- Tests verify track downloads and metadata consistency

## Objectives

1. **Primary**: Add `albums` section to configuration model
2. **Primary**: Support both simple and extended album entry formats
3. **Primary**: Add per-album M3U creation configuration
4. **Primary**: Integrate album processing into main download orchestration
5. **Secondary**: Update `download_album()` to support M3U creation
6. **Tertiary**: Update documentation and examples

## Technical Approach

### Phase 1: Extend Configuration Model

#### Step 1.1: Create AlbumSource Model

Create a new model that extends `MusicSource` with M3U option:

```python
# In core/config.py
class MusicSource(BaseModel):
    """Music source entry."""
    name: str
    url: str

class AlbumSource(BaseModel):
    """Album source entry with optional M3U creation."""
    name: str
    url: str
    create_m3u: bool = False  # Default to False for backward compatibility
```



#### Step 1.2: Update MusicDLConfig

Add albums field to main config:

```python
class MusicDLConfig(BaseModel):
    """Main configuration model."""
    version: Literal["1.2"]
    download: DownloadSettings
    songs: List[MusicSource] = Field(default_factory=list)
    artists: List[MusicSource] = Field(default_factory=list)
    playlists: List[MusicSource] = Field(default_factory=list)
    albums: List[AlbumSource] = Field(default_factory=list)  # New field
```



#### Step 1.3: Update Config Parsing

Extend `from_yaml()` to handle albums with both formats:

```python
def convert_album_sources(sources):
    """Convert dict or list format to AlbumSource list, supporting both formats."""
    if not sources:
        return []
    result = []
    if isinstance(sources, list):
        for item in sources:
            if isinstance(item, dict):
                # Extended format: {name: "...", url: "...", create_m3u: true/false}
                if "name" in item and "url" in item:
                    result.append(AlbumSource(
                        name=item["name"],
                        url=item["url"],
                        create_m3u=item.get("create_m3u", False)
                    ))
                else:
                    # Simple format: {name: url}
                    for name, url in item.items():
                        result.append(AlbumSource(name=name, url=url, create_m3u=False))
            elif isinstance(item, str):
                # Handle [url, ...] - use URL as name
                result.append(AlbumSource(name=item, url=item, create_m3u=False))
    elif isinstance(sources, dict):
        # Handle {name: url, ...} - simple format
        for name, url in sources.items():
            result.append(AlbumSource(name=name, url=url, create_m3u=False))
    return result

# In from_yaml method, add:
if "albums" in data:
    data["albums"] = convert_album_sources(data["albums"])
```



### Phase 2: Update Download Orchestration

#### Step 2.1: Add Album Processing to `process_downloads()`

Update `download.py` to process albums:

```python
def process_downloads(config) -> Dict[str, Dict[str, int]]:
    """Orchestrate all downloads."""
    downloader = Downloader(config.download)
    results = {
        "songs": {"success": 0, "failed": 0},
        "artists": {"success": 0, "failed": 0},
        "playlists": {"success": 0, "failed": 0},
        "albums": {"success": 0, "failed": 0},  # New category
    }
    
    # ... existing song, artist, playlist processing ...
    
    # Process albums
    logger.info(f"Processing {len(config.albums)} albums...")
    for album in config.albums:
        logger.info(f"Downloading album: {album.name}")
        try:
            tracks = downloader.download_album(album.url, create_m3u=album.create_m3u)
            success_count = sum(1 for success, _ in tracks if success)
            failed_count = len(tracks) - success_count
            results["albums"]["success"] += success_count
            results["albums"]["failed"] += failed_count
            logger.info(
                f"Album {album.name}: {success_count} successful, {failed_count} failed"
            )
        except Exception as e:
            logger.error(f"Error downloading album {album.name}: {e}")
    
    return results
```



#### Step 2.2: Update Summary Printing

Update `print_summary()` to include albums:

```python
def print_summary(results: Dict[str, Dict[str, int]]) -> None:
    """Print download summary."""
    print("\n" + "=" * 80)
    print("DOWNLOAD SUMMARY")
    print("=" * 80)
    
    total_success = 0
    total_failed = 0
    
    for category, stats in results.items():
        success = stats["success"]
        failed = stats["failed"]
        total_success += success
        total_failed += failed
        print(f"{category.capitalize()}: {success} successful, {failed} failed")
    
    print("-" * 80)
    print(f"Total: {total_success} successful, {total_failed} failed")
    print("=" * 80)
```



### Phase 3: Update Downloader Method

#### Step 3.1: Add M3U Support to `download_album()`

Update `download_album()` to support M3U creation:

```python
def download_album(
    self, album_url: str, create_m3u: bool = False
) -> List[Tuple[bool, Optional[Path]]]:
    """
    Download all tracks in an album.
    
    Args:
        album_url: Spotify album URL or ID
        create_m3u: Whether to create M3U playlist file
    
    Returns:
        List of (success, file_path) tuples
    """
    try:
        album_data = self.spotify.get_album(album_url)
        tracks = []
        
        # Get all tracks (handle pagination)
        items = album_data["tracks"]["items"]
        tracks_obj = album_data["tracks"]
        while tracks_obj.get("next"):
            next_data = self.spotify.client.next(tracks_obj)
            items.extend(next_data["items"])
            tracks_obj = next_data
        
        logger.info(f"Found {len(items)} tracks in album: {album_data['name']}")
        
        for track_item in items:
            track_id = track_item["id"]
            track_url = f"https://open.spotify.com/track/{track_id}"
            result = self.download_track(track_url)
            tracks.append(result)
        
        # Create M3U file if requested
        if create_m3u:
            self._create_m3u(album_data["name"], tracks)
        
        return tracks
        
    except Exception as e:
        logger.error(f"Failed to download album {album_url}: {e}")
        return [(False, None)]
```

**Note**: The `_create_m3u()` method already exists and can be reused.

### Phase 4: Update Tests

#### Step 4.1: Update Config Tests

Add tests for album configuration parsing:

```python
# In tests/unit/test_config.py
def test_config_with_albums_simple_format():
    """Test config with albums in simple format."""
    config_data = {
        "version": "1.2",
        "download": {...},
        "albums": {
            "Album Name": "https://open.spotify.com/album/..."
        }
    }
    config = MusicDLConfig(**config_data)
    assert len(config.albums) == 1
    assert config.albums[0].name == "Album Name"
    assert config.albums[0].create_m3u is False  # Default

def test_config_with_albums_extended_format():
    """Test config with albums in extended format."""
    config_data = {
        "version": "1.2",
        "download": {...},
        "albums": [
            {
                "name": "Album Name",
                "url": "https://open.spotify.com/album/...",
                "create_m3u": True
            }
        ]
    }
    config = MusicDLConfig(**config_data)
    assert len(config.albums) == 1
    assert config.albums[0].name == "Album Name"
    assert config.albums[0].create_m3u is True
```



#### Step 4.2: Update E2E Tests

Update existing album E2E tests to test M3U creation:

```python
# In tests/e2e/test_album_download.py
def test_album_download_with_m3u(self, downloader, tmp_test_dir):
    """Test album download with M3U file creation."""
    album_url = "https://open.spotify.com/album/77CZUF57sYqgtznUe3OikQ"
    
    results = downloader.download_album(album_url, create_m3u=True)
    
    assert len(results) > 0
    
    # Check M3U file was created
    m3u_file = tmp_test_dir / "I Love My Computer.m3u"
    assert m3u_file.exists()
    assert "#EXTM3U" in m3u_file.read_text()
```



#### Step 4.3: Add Integration Tests

Test full workflow with config:

```python
# In tests/integration/test_downloader_integration.py
def test_album_download_from_config(self, tmp_test_dir, spotify_credentials):
    """Test album download from configuration."""
    config = MusicDLConfig(
        version="1.2",
        download=DownloadSettings(...),
        albums=[
            AlbumSource(
                name="Test Album",
                url="https://open.spotify.com/album/...",
                create_m3u=True
            )
        ]
    )
    
    results = process_downloads(config)
    
    assert results["albums"]["success"] > 0
```



### Phase 5: Update Documentation

#### Step 5.1: Update README.md

Add albums section to documentation:

````markdown
### Music Sources

- `songs`: List of individual songs `{name: url}`
- `artists`: List of artists to download discography (albums and singles only, excludes compilations and featured appearances)
- `playlists`: List of playlists (creates M3U files)
- `albums`: List of albums to download

#### Album Configuration

Albums support two formats:

**Simple format** (no M3U file):
```yaml
albums:
    - "Album Name": https://open.spotify.com/album/...
````

**Extended format** (with M3U option):

```yaml
albums:
    - name: "Album Name"
    url: https://open.spotify.com/album/...
    create_m3u: true  # Optional, defaults to false
```
````javascript

#### Step 5.2: Update Sample Config
Add album examples to `tests/fixtures/sample_config.yaml`:

```yaml
albums:
    - name: Test Album
    url: https://open.spotify.com/album/77CZUF57sYqgtznUe3OikQ
    create_m3u: true
    - "Simple Album": https://open.spotify.com/album/...
````



## Implementation Details

### Configuration Format Support

**Simple Format**:

```yaml
albums:
    - "Album Name": https://open.spotify.com/album/...
    - "Another Album": https://open.spotify.com/album/...
```

**Extended Format**:

```yaml
albums:
    - name: "Album Name"
    url: https://open.spotify.com/album/...
    create_m3u: true
    - name: "Another Album"
    url: https://open.spotify.com/album/...
    create_m3u: false
```

**Mixed Format** (both in same config):

```yaml
albums:
    - "Simple Album": https://open.spotify.com/album/...
    - name: "Extended Album"
    url: https://open.spotify.com/album/...
    create_m3u: true
```



### M3U File Creation

- **Location**: Created in current working directory (same as playlists)
- **Naming**: Uses sanitized album name (same as playlist M3U creation)
- **Format**: Standard M3U format with `#EXTM3U` header
- **Content**: Includes all successfully downloaded tracks

### Backward Compatibility

- **No Breaking Changes**: Existing configs without `albums` section continue to work
- **Default Behavior**: `create_m3u` defaults to `False` (no M3U created)
- **Simple Format**: Defaults to no M3U (backward compatible with simple usage)

## Testing Strategy

### Unit Tests

1. Test `AlbumSource` model creation
2. Test config parsing with simple format
3. Test config parsing with extended format
4. Test config parsing with mixed formats
5. Test `download_album()` with `create_m3u=True`
6. Test `download_album()` with `create_m3u=False`

### Integration Tests

1. Test full workflow with albums in config
2. Test M3U file creation for albums
3. Test album processing in `process_downloads()`
4. Test summary includes albums

### E2E Tests

1. Test album download end-to-end
2. Test album download with M3U creation
3. Test multiple albums in config
4. Test mixed simple and extended formats

## Risk Assessment

### Low Risk

- `download_album()` method already exists and works
- Config parsing follows same pattern as existing sources
- M3U creation reuses existing method
- No breaking changes to existing functionality

### Medium Risk

- Config format complexity (mitigated by comprehensive parsing logic)
- M3U file naming conflicts (mitigated by sanitization)

### High Risk

- None identified

### Mitigation Strategies

1. **Comprehensive Testing**: Test all config formats
2. **Backward Compatibility**: Ensure existing configs still work
3. **Documentation**: Clear examples of both formats
4. **Validation**: Validate album URLs in config parsing

## Success Criteria

1. ✅ Albums can be specified in configuration file
2. ✅ Both simple and extended formats supported
3. ✅ Per-album M3U creation works
4. ✅ Albums processed in main download orchestration
5. ✅ Summary includes album statistics
6. ✅ All existing tests pass
7. ✅ Documentation updated

## Rollback Plan

If issues are discovered:

1. Remove `albums` field from config model
2. Remove album processing from `process_downloads()`
3. Revert `download_album()` signature changes
4. Investigate root cause
5. Create fix branch
6. Re-test and re-deploy

## Timeline

- **Day 1**: Extend config model, update config parsing, add tests
- **Day 2**: Update download orchestration, update downloader method, integration tests
- **Day 3**: Update documentation, E2E tests, final validation

## Dependencies

- `download_album()` method (already exists)
- `_create_m3u()` method (already exists)
- Spotify API `get_album()` (already in use)
- No new external dependencies

## Related Files

- `core/config.py` - Add `AlbumSource` model and `albums` field
- `core/downloader.py` - Update `download_album()` signature
- `download.py` - Add album processing to `process_downloads()`
- `tests/unit/test_config.py` - Add album config tests
- `tests/e2e/test_album_download.py` - Update/add album E2E tests
- `tests/integration/test_downloader_integration.py` - Add integration tests
- `tests/fixtures/sample_config.yaml` - Add album examples
- `README.md` - Update documentation

## Notes for Junior Engineers

### Why Add Album Support?

- **User Experience**: Users can download specific albums directly
- **Feature Parity**: Matches functionality of songs, artists, playlists
- **Flexibility**: Per-album M3U creation option
- **Convenience**: No need to download entire artist discography for one album

### Configuration Formats

**Simple Format**: Quick and easy for basic use

```yaml
albums:
    - "Album Name": https://open.spotify.com/album/...
```

**Extended Format**: More control with M3U option

```yaml
albums:
    - name: "Album Name"
    url: https://open.spotify.com/album/...
    create_m3u: true
```



### Common Pitfalls

1. **Config Parsing**: Handle both formats correctly
2. **M3U Naming**: Use sanitized album name (same as playlists)
3. **Default Values**: `create_m3u` defaults to `False`
4. **Error Handling**: Handle invalid album URLs gracefully

### Debugging Tips

- Test config parsing with both formats
- Verify M3U files are created when `create_m3u=True`
- Check album processing in download summary
- Test with real Spotify album URLs

## Notes for Technical Management

### Business Impact

- **User Experience**: More flexible download options
- **Feature Completeness**: All major Spotify content types supported
- **Usability**: Direct album downloads without artist context

### Resource Requirements

- **Development Time**: 2-3 days
- **Testing Time**: 1 day
- **Risk**: Low (reuses existing functionality)

### Recommendation