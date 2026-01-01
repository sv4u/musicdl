# Plan: Reduce Scope of Artist Downloads to Just Artist Discography

## Executive Summary

This plan outlines the reduction of artist download scope to include only the artist's discography (albums and singles), excluding compilations and "Appears On" albums where the artist is featured but not the main artist. This change ensures that artist downloads focus on the artist's own releases rather than including collaborative works or compilation appearances, providing a cleaner and more focused discography download experience.

**Target Audience**: Technical leads, junior engineers, and technical management

**Estimated Effort**: 1-2 days

**Risk Level**: Low

**Priority**: Medium (improves download accuracy and user experience)

## Current State Analysis

### Current Implementation

#### Artist Download Flow (`core/downloader.py`)
```python
def download_artist(self, artist_url: str) -> List[Tuple[bool, Optional[Path]]]:
    """Download all albums for an artist."""
    try:
        albums = self.spotify.get_artist_albums(artist_url)
        all_tracks = []
        
        logger.info(f"Found {len(albums)} albums for artist")
        
        for album in albums:
            album_url = album["external_urls"]["spotify"]
            logger.info(f"Downloading album: {album['name']}")
            tracks = self.download_album(album_url)
            all_tracks.extend(tracks)
        
        return all_tracks
```

#### Spotify API Call (`core/spotify_client.py`)
```python
def get_artist_albums(self, artist_id_or_url: str) -> List[Dict[str, Any]]:
    """Get all albums for an artist (cached)."""
    def fetch_albums() -> List[Dict[str, Any]]:
        albums = []
        results = self.client.artist_albums(artist_id, limit=50)
        albums.extend(results.get("items", []))
        
        # Handle pagination
        while results.get("next"):
            results = self.client.next(results)
            albums.extend(results.get("items", []))
        
        return albums
```

### Current Behavior

**What Gets Downloaded**:
- All albums where the artist appears (no filtering)
- Includes:
  - **Albums**: Full studio albums where artist is main artist
  - **Singles**: Single releases where artist is main artist
  - **Compilations**: Compilation albums (e.g., "Greatest Hits", "Best Of")
  - **Appears On**: Albums where artist is featured but not main artist

**Problem**: Downloads include albums where the artist is not the primary artist, leading to:
- Duplicate content (same songs from different albums)
- Unwanted collaborative works
- Compilation albums that may not represent the artist's discography
- Larger download sizes than necessary

### Spotify API Details

The Spotify API `artist_albums` endpoint supports filtering via `include_groups` parameter:

- **`album`**: Full studio albums
- **`single`**: Single releases
- **`compilation`**: Compilation albums
- **`appears_on`**: Albums where artist appears but isn't main artist

**Current Code**: No `include_groups` parameter specified, so all types are returned.

## Objectives

1. **Primary**: Filter artist downloads to include only albums and singles (discography)
2. **Primary**: Exclude compilations and "Appears On" albums
3. **Secondary**: Maintain backward compatibility with existing functionality
4. **Tertiary**: Update documentation to reflect the change

## Technical Approach

### Phase 1: Update Spotify API Call

#### Step 1.1: Modify `get_artist_albums` Method
Update the Spotify API call to filter album types:

```python
def get_artist_albums(self, artist_id_or_url: str) -> List[Dict[str, Any]]:
    """Get all albums for an artist (cached), excluding compilations and appears_on."""
    artist_id = extract_id_from_url(artist_id_or_url)
    cache_key = f"artist_albums:{artist_id}"
    
    def fetch_albums() -> List[Dict[str, Any]]:
        """Fetch all albums for artist with pagination, filtered to discography only."""
        albums = []
        # Include only album and single types (exclude compilation and appears_on)
        results = self.client.artist_albums(
            artist_id,
            limit=50,
            include_groups="album,single"
        )
        albums.extend(results.get("items", []))
        
        # Handle pagination
        while results.get("next"):
            results = self.client.next(results)
            albums.extend(results.get("items", []))
        
        return albums
    
    return self._get_cached_or_fetch(cache_key, fetch_albums)
```

**Key Changes**:
- Add `include_groups="album,single"` parameter to `artist_albums()` call
- This filters out compilations and "appears_on" albums at the API level
- Cache key remains the same (no breaking change to cache)

#### Step 1.2: Update Method Documentation
Update docstring to reflect the filtering:

```python
def get_artist_albums(self, artist_id_or_url: str) -> List[Dict[str, Any]]:
    """
    Get all albums and singles for an artist (cached).
    
    Excludes compilations and "Appears On" albums to focus on the
    artist's discography only.
    
    Args:
        artist_id_or_url: Spotify artist URL or ID
    
    Returns:
        List of album dictionaries (albums and singles only)
    """
```

### Phase 2: Update Downloader Documentation

#### Step 2.1: Update `download_artist` Docstring
Clarify what gets downloaded:

```python
def download_artist(self, artist_url: str) -> List[Tuple[bool, Optional[Path]]]:
    """
    Download all albums and singles for an artist (discography only).
    
    Downloads the artist's discography, including:
    - Full studio albums
    - Single releases
    
    Excludes:
    - Compilation albums
    - "Appears On" albums (where artist is featured but not main artist)
    
    Args:
        artist_url: Spotify artist URL or ID
    
    Returns:
        List of (success, file_path) tuples
    """
```

### Phase 3: Update Tests

#### Step 3.1: Update Unit Tests
Update tests to verify filtering behavior:

```python
def test_get_artist_albums_filters_compilations(mock_spotify_client):
    """Test that get_artist_albums excludes compilations and appears_on."""
    # Mock Spotify API response with mixed album types
    mock_response = {
        "items": [
            {"id": "1", "name": "Studio Album", "album_type": "album"},
            {"id": "2", "name": "Single", "album_type": "single"},
            {"id": "3", "name": "Greatest Hits", "album_type": "compilation"},
            {"id": "4", "name": "Featured Album", "album_type": "appears_on"},
        ],
        "next": None
    }
    
    # Verify API is called with include_groups filter
    mock_spotify_client.client.artist_albums.assert_called_with(
        "artist_id",
        limit=50,
        include_groups="album,single"
    )
    
    # Verify only albums and singles are returned
    albums = mock_spotify_client.get_artist_albums("artist_id")
    assert len(albums) == 2
    assert all(album["album_type"] in ["album", "single"] for album in albums)
```

#### Step 3.2: Update Integration Tests
Update E2E tests if they expect specific album counts:

```python
def test_artist_download_excludes_compilations(spotify_client, downloader):
    """Test that artist download excludes compilations."""
    # Use an artist known to have compilations
    artist_url = "https://open.spotify.com/artist/..."
    
    tracks = downloader.download_artist(artist_url)
    
    # Verify that compilation albums are not included
    # This may require checking album metadata or counting albums
    assert len(tracks) > 0
    # Additional assertions based on known artist discography
```

### Phase 4: Update Documentation

#### Step 4.1: Update README.md
Update the documentation to clarify artist download behavior:

```markdown
### Music Sources

- `songs`: List of individual songs `{name: url}`
- `artists`: List of artists to download discography (albums and singles only, excludes compilations and featured appearances)
- `playlists`: List of playlists (creates M3U files)
```

#### Step 4.2: Update Code Comments
Add inline comments explaining the filtering:

```python
# Filter to discography only (albums and singles)
# Excludes compilations and "appears_on" albums where artist is featured
results = self.client.artist_albums(
    artist_id,
    limit=50,
    include_groups="album,single"
)
```

## Implementation Details

### API Parameter Usage

**Spotify API `artist_albums` endpoint**:
- **Parameter**: `include_groups`
- **Type**: Comma-separated string
- **Values**: `"album"`, `"single"`, `"compilation"`, `"appears_on"`
- **Default**: All types if not specified
- **Our Value**: `"album,single"` (excludes compilation and appears_on)

### Cache Considerations

**Cache Key**: `f"artist_albums:{artist_id}"`
- **Impact**: Cache key remains the same
- **Behavior**: Filtered results will be cached (desired behavior)
- **Invalidation**: No changes needed - cache will update naturally on TTL expiration

### Backward Compatibility

**Breaking Changes**: None
- Existing functionality remains the same
- Only the scope of what's downloaded changes
- No API changes to `download_artist()` method signature
- No configuration changes required

**User Impact**:
- Users will download fewer albums (only discography)
- This is the intended behavior (feature, not bug)
- May reduce download time and storage usage

## Testing Strategy

### Unit Tests

1. **Test API Call**: Verify `include_groups` parameter is passed correctly
2. **Test Filtering**: Verify only albums and singles are returned
3. **Test Cache**: Verify filtered results are cached correctly
4. **Test Pagination**: Verify filtering works with paginated results

### Integration Tests

1. **Test Real Artist**: Test with real Spotify artist that has compilations
2. **Test Album Count**: Verify fewer albums returned than before
3. **Test Download**: Verify downloads work correctly with filtered albums

### Manual Testing

1. Test with artist known to have compilations
2. Verify compilations are not downloaded
3. Verify "Appears On" albums are not downloaded
4. Verify albums and singles are still downloaded
5. Check logs to confirm filtering is working

## Risk Assessment

### Low Risk

- Simple API parameter change
- No breaking changes to method signatures
- Well-documented Spotify API feature
- Can be tested easily

### Medium Risk

- Cache behavior (mitigated by testing)
- Users may expect compilations (mitigated by documentation)

### High Risk

- None identified

### Mitigation Strategies

1. **Comprehensive Testing**: Test with various artists
2. **Documentation**: Clearly document what's included/excluded
3. **Logging**: Log filtered album counts for visibility
4. **Gradual Rollout**: Test in development before production

## Success Criteria

1. ✅ Artist downloads include only albums and singles
2. ✅ Compilations are excluded
3. ✅ "Appears On" albums are excluded
4. ✅ All existing tests pass
5. ✅ Documentation updated
6. ✅ No breaking changes to API

## Rollback Plan

If issues are discovered:

1. Remove `include_groups` parameter from API call
2. Revert to previous behavior (all album types)
3. Investigate root cause
4. Create fix branch
5. Re-test and re-deploy

## Timeline

- **Day 1**: Update API call, update documentation, write tests
- **Day 2**: Run tests, manual testing, final validation

## Dependencies

- Spotify API `artist_albums` endpoint (already in use)
- No new dependencies required

## Related Files

- `core/spotify_client.py` - Update `get_artist_albums()` method
- `core/downloader.py` - Update `download_artist()` docstring
- `tests/unit/test_spotify_client.py` - Update/add tests
- `tests/integration/test_spotify_integration.py` - Update integration tests
- `tests/e2e/test_artist_download.py` - Update E2E tests
- `README.md` - Update documentation

## Notes for Junior Engineers

### Why Filter Album Types?

- **Focus**: Downloads only the artist's own releases
- **Accuracy**: Avoids duplicate content from compilations
- **Efficiency**: Reduces download size and time
- **User Experience**: Cleaner discography downloads

### Spotify API `include_groups`

The `include_groups` parameter filters album types:
- `"album"`: Full studio albums
- `"single"`: Single releases
- `"compilation"`: Compilation albums (excluded)
- `"appears_on"`: Featured appearances (excluded)

### Common Pitfalls

1. **Cache Invalidation**: Filtered results are cached - this is correct behavior
2. **Pagination**: Filtering happens at API level, so pagination still works
3. **Album Type Field**: The `album_type` field in response indicates the type

### Debugging Tips

- Check API call parameters in logs
- Verify `include_groups` is set correctly
- Test with artists known to have compilations
- Compare album counts before/after change

## Notes for Technical Management

### Business Impact

- **User Experience**: More focused downloads (only discography)
- **Efficiency**: Reduced download size and time
- **Accuracy**: Avoids unwanted collaborative works
- **Storage**: Less storage used per artist download

### Resource Requirements

- **Development Time**: 1-2 days
- **Testing Time**: 0.5 day
- **Risk**: Low (simple API parameter change)

### Recommendation

Proceed with implementation. This is a straightforward improvement that aligns artist downloads with user expectations (discography only). The change is low-risk, well-tested, and improves the user experience.

