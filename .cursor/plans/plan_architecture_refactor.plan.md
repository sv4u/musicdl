# Plan: Refactor into Plan Architecture

## Executive Summary

This plan outlines a major architectural refactoring to transform musicdl from a sequential, configuration-driven execution model to a plan-based architecture. Instead of processing configuration items sequentially, the system will read the entire configuration, generate a comprehensive download plan, optimize it (removing duplicates, resolving dependencies), and then execute the plan efficiently. This architecture enables better optimization, parallelization, progress tracking, and error recovery.**Target Audience**: Technical leads, junior engineers, and technical management**Estimated Effort**: 10-14 days**Risk Level**: Medium-High (major architectural change)**Priority**: High (foundational improvement)

## Current State Analysis

### Current Architecture

#### Sequential Processing Flow

```javascript
1. Load configuration
2. For each song in config.songs:
            - Download song
3. For each artist in config.artists:
            - Get artist albums
            - For each album:
                    - Get album tracks
                    - For each track:
                            - Download track
4. For each playlist in config.playlists:
            - Get playlist tracks
            - For each track:
                    - Download track
```



#### Current Implementation (`download.py`)

```python
def process_downloads(config):
    # Process songs sequentially
    for song in config.songs:
        downloader.download_track(song.url)
    
    # Process artists sequentially
    for artist in config.artists:
        downloader.download_artist(artist.url)
    
    # Process playlists sequentially
    for playlist in config.playlists:
        downloader.download_playlist(playlist.url)
```



### Problems with Current Architecture

1. **No Global View**: Can't see all downloads before starting
2. **Duplicate Downloads**: Same track may be downloaded multiple times
3. **No Optimization**: Can't optimize order or batch operations
4. **Poor Progress Tracking**: Hard to show overall progress
5. **Error Recovery**: Difficult to resume after failures
6. **Resource Planning**: Can't estimate total work upfront

## Objectives

1. **Primary**: Create a plan-based architecture that reads entire config first
2. **Primary**: Generate a comprehensive download plan with all items
3. **Primary**: Optimize plan by removing duplicates and resolving dependencies
4. **Secondary**: Enable better parallelization and progress tracking
5. **Tertiary**: Support plan persistence and resumption
6. **Tertiary**: Provide plan visualization and reporting

## Technical Approach

### Phase 1: Design Plan Data Models

#### Step 1.1: Define Plan Item Model

Create data models for plan items:

```python
# In core/models.py or new core/plan.py
from dataclasses import dataclass
from enum import Enum
from typing import Optional, List, Set
from pathlib import Path

class PlanItemType(Enum):
    """Type of plan item."""
    TRACK = "track"
    ALBUM = "album"
    PLAYLIST = "playlist"
    ARTIST = "artist"

class PlanItemStatus(Enum):
    """Status of plan item."""
    PENDING = "pending"
    IN_PROGRESS = "in_progress"
    COMPLETED = "completed"
    FAILED = "failed"
    SKIPPED = "skipped"

@dataclass
class PlanItem:
    """Represents a single item in the download plan."""
    # Identity
    id: str  # Unique identifier (e.g., Spotify track ID)
    type: PlanItemType
    url: str
    name: str  # Human-readable name
    
    # Metadata (populated during planning)
    artist: Optional[str] = None
    title: Optional[str] = None
    album: Optional[str] = None
    track_number: Optional[int] = None
    duration: Optional[int] = None
    
    # Execution
    status: PlanItemStatus = PlanItemStatus.PENDING
    output_path: Optional[Path] = None
    error: Optional[str] = None
    retry_count: int = 0
    
    # Relationships
    parent_id: Optional[str] = None  # For tracks in albums/playlists
    dependencies: List[str] = None  # Items that must complete first
    
    def __post_init__(self):
        if self.dependencies is None:
            self.dependencies = []
    
    def __hash__(self):
        return hash(self.id)
    
    def __eq__(self, other):
        return isinstance(other, PlanItem) and self.id == other.id
```



#### Step 1.2: Define Plan Model

Create the overall plan structure:

```python
@dataclass
class DownloadPlan:
    """Complete download plan with all items."""
    items: List[PlanItem]
    created_at: float  # Timestamp
    config_version: str
    
    def __post_init__(self):
        # Create lookup dictionaries
        self._items_by_id: Dict[str, PlanItem] = {
            item.id: item for item in self.items
        }
        self._items_by_type: Dict[PlanItemType, List[PlanItem]] = {}
        for item in self.items:
            if item.type not in self._items_by_type:
                self._items_by_type[item.type] = []
            self._items_by_type[item.type].append(item)
    
    def get_item(self, item_id: str) -> Optional[PlanItem]:
        """Get item by ID."""
        return self._items_by_id.get(item_id)
    
    def get_items_by_type(self, item_type: PlanItemType) -> List[PlanItem]:
        """Get all items of a specific type."""
        return self._items_by_type.get(item_type, [])
    
    def get_pending_items(self) -> List[PlanItem]:
        """Get all pending items."""
        return [item for item in self.items if item.status == PlanItemStatus.PENDING]
    
    def get_completed_items(self) -> List[PlanItem]:
        """Get all completed items."""
        return [item for item in self.items if item.status == PlanItemStatus.COMPLETED]
    
    def get_failed_items(self) -> List[PlanItem]:
        """Get all failed items."""
        return [item for item in self.items if item.status == PlanItemStatus.FAILED]
    
    def get_statistics(self) -> Dict[str, int]:
        """Get plan statistics."""
        return {
            "total": len(self.items),
            "pending": len(self.get_pending_items()),
            "completed": len(self.get_completed_items()),
            "failed": len(self.get_failed_items()),
            "skipped": len([i for i in self.items if i.status == PlanItemStatus.SKIPPED]),
        }
```



### Phase 2: Implement Plan Generator

#### Step 2.1: Create PlanGenerator Class

Create a class to generate plans from configuration:

```python
# In core/plan_generator.py
from typing import List, Set
from core.config import MusicDLConfig
from core.spotify_client import SpotifyClient
from core.models import PlanItem, PlanItemType, PlanItemStatus, DownloadPlan
import time

class PlanGenerator:
    """Generates download plans from configuration."""
    
    def __init__(self, spotify: SpotifyClient):
        self.spotify = spotify
        self._seen_ids: Set[str] = set()  # Track duplicates
    
    def generate_plan(self, config: MusicDLConfig) -> DownloadPlan:
        """Generate complete download plan from configuration."""
        items: List[PlanItem] = []
        
        # Process songs
        for song in config.songs:
            items.extend(self._process_song(song.url, song.name))
        
        # Process artists
        for artist in config.artists:
            items.extend(self._process_artist(artist.url, artist.name))
        
        # Process playlists
        for playlist in config.playlists:
            items.extend(self._process_playlist(playlist.url, playlist.name))
        
        return DownloadPlan(
            items=items,
            created_at=time.time(),
            config_version=config.version
        )
    
    def _process_song(self, url: str, name: str) -> List[PlanItem]:
        """Process a single song URL."""
        track_id = self._extract_track_id(url)
        
        # Check for duplicates
        if track_id in self._seen_ids:
            return []  # Skip duplicate
        
        self._seen_ids.add(track_id)
        
        # Fetch metadata
        try:
            track_data = self.spotify.get_track(url)
            album_data = self.spotify.get_album(track_data["album"]["id"])
            
            item = PlanItem(
                id=track_id,
                type=PlanItemType.TRACK,
                url=url,
                name=name or track_data["name"],
                artist=track_data["artists"][0]["name"],
                title=track_data["name"],
                album=album_data.get("name"),
                track_number=track_data.get("track_number"),
                duration=int(track_data.get("duration_ms", 0) / 1000),
            )
            return [item]
        except Exception as e:
            logger.error(f"Failed to process song {url}: {e}")
            return []
    
    def _process_artist(self, url: str, name: str) -> List[PlanItem]:
        """Process an artist URL."""
        items: List[PlanItem] = []
        
        try:
            albums = self.spotify.get_artist_albums(url)
            
            for album in albums:
                album_items = self._process_album(
                    album["external_urls"]["spotify"],
                    album["name"],
                    parent_name=name
                )
                items.extend(album_items)
            
            return items
        except Exception as e:
            logger.error(f"Failed to process artist {url}: {e}")
            return []
    
    def _process_album(self, url: str, name: str, parent_name: str = None) -> List[PlanItem]:
        """Process an album URL."""
        items: List[PlanItem] = []
        album_id = self._extract_album_id(url)
        
        try:
            album_data = self.spotify.get_album(url)
            tracks_obj = album_data["tracks"]
            track_items = tracks_obj["items"]
            
            # Handle pagination
            while tracks_obj.get("next"):
                next_data = self.spotify.client.next(tracks_obj)
                track_items.extend(next_data["items"])
                tracks_obj = next_data
            
            for track_item in track_items:
                track_id = track_item["id"]
                
                # Check for duplicates
                if track_id in self._seen_ids:
                    continue
                
                self._seen_ids.add(track_id)
                
                item = PlanItem(
                    id=track_id,
                    type=PlanItemType.TRACK,
                    url=f"https://open.spotify.com/track/{track_id}",
                    name=track_item["name"],
                    artist=track_item["artists"][0]["name"],
                    title=track_item["name"],
                    album=album_data["name"],
                    track_number=track_item.get("track_number"),
                    duration=int(track_item.get("duration_ms", 0) / 1000),
                    parent_id=album_id,
                )
                items.append(item)
            
            return items
        except Exception as e:
            logger.error(f"Failed to process album {url}: {e}")
            return []
    
    def _process_playlist(self, url: str, name: str) -> List[PlanItem]:
        """Process a playlist URL."""
        items: List[PlanItem] = []
        playlist_id = self._extract_playlist_id(url)
        
        try:
            playlist_data = self.spotify.get_playlist(url)
            tracks_obj = playlist_data["tracks"]
            track_items = tracks_obj["items"]
            
            # Handle pagination
            while tracks_obj.get("next"):
                next_data = self.spotify.client.next(tracks_obj)
                track_items.extend(next_data["items"])
                tracks_obj = next_data
            
            for track_item in track_items:
                track = track_item.get("track")
                if not track or track.get("is_local"):
                    continue
                
                track_id = track["id"]
                
                # Check for duplicates
                if track_id in self._seen_ids:
                    continue
                
                self._seen_ids.add(track_id)
                
                item = PlanItem(
                    id=track_id,
                    type=PlanItemType.TRACK,
                    url=track["external_urls"]["spotify"],
                    name=track["name"],
                    artist=track["artists"][0]["name"],
                    title=track["name"],
                    parent_id=playlist_id,
                )
                items.append(item)
            
            return items
        except Exception as e:
            logger.error(f"Failed to process playlist {url}: {e}")
            return []
    
    def _extract_track_id(self, url: str) -> str:
        """Extract track ID from URL."""
        # Implementation similar to existing extract_id_from_url
        import re
        match = re.search(r'track/([a-zA-Z0-9]+)', url)
        return match.group(1) if match else url
    
    def _extract_album_id(self, url: str) -> str:
        """Extract album ID from URL."""
        import re
        match = re.search(r'album/([a-zA-Z0-9]+)', url)
        return match.group(1) if match else url
    
    def _extract_playlist_id(self, url: str) -> str:
        """Extract playlist ID from URL."""
        import re
        match = re.search(r'playlist/([a-zA-Z0-9]+)', url)
        return match.group(1) if match else url
```



### Phase 3: Implement Plan Optimizer

#### Step 3.1: Create PlanOptimizer Class

Optimize the plan by removing duplicates and organizing items:

```python
# In core/plan_optimizer.py
from core.models import DownloadPlan, PlanItem, PlanItemStatus
from pathlib import Path
from typing import Dict, Set

class PlanOptimizer:
    """Optimizes download plans."""
    
    def __init__(self, output_template: str, overwrite: str = "skip"):
        self.output_template = output_template
        self.overwrite = overwrite
    
    def optimize(self, plan: DownloadPlan) -> DownloadPlan:
        """Optimize plan by removing duplicates and checking file existence."""
        # Step 1: Remove duplicate items (by ID)
        unique_items = self._remove_duplicates(plan.items)
        
        # Step 2: Check file existence and mark as skipped if needed
        if self.overwrite == "skip":
            unique_items = self._check_existing_files(unique_items)
        
        # Step 3: Sort items (optional: by artist, album, track number)
        unique_items = self._sort_items(unique_items)
        
        # Create optimized plan
        optimized_plan = DownloadPlan(
            items=unique_items,
            created_at=plan.created_at,
            config_version=plan.config_version
        )
        
        return optimized_plan
    
    def _remove_duplicates(self, items: List[PlanItem]) -> List[PlanItem]:
        """Remove duplicate items, keeping first occurrence."""
        seen_ids: Set[str] = set()
        unique_items: List[PlanItem] = []
        
        for item in items:
            if item.id not in seen_ids:
                seen_ids.add(item.id)
                unique_items.append(item)
            else:
                logger.debug(f"Skipping duplicate: {item.name} ({item.id})")
        
        logger.info(f"Removed {len(items) - len(unique_items)} duplicate items")
        return unique_items
    
    def _check_existing_files(self, items: List[PlanItem]) -> List[PlanItem]:
        """Check if files already exist and mark as skipped."""
        from core.downloader import format_filename, _sanitize
        from core.models import Song
        
        skipped_count = 0
        
        for item in items:
            if item.status != PlanItemStatus.PENDING:
                continue
            
            # Create Song object for filename formatting
            song = Song(
                title=item.title or item.name,
                artist=item.artist or "Unknown",
                album=item.album,
                track_number=item.track_number,
                # ... other fields ...
            )
            
            # Format filename
            filename = format_filename(self.output_template, song, "mp3")
            file_path = Path(filename)
            
            # Check if exists
            if file_path.exists():
                item.status = PlanItemStatus.SKIPPED
                item.output_path = file_path
                skipped_count += 1
                logger.debug(f"File exists, skipping: {file_path}")
        
        logger.info(f"Marked {skipped_count} items as skipped (files exist)")
        return items
    
    def _sort_items(self, items: List[PlanItem]) -> List[PlanItem]:
        """Sort items for optimal download order."""
        # Sort by: artist, album, track_number
        return sorted(
            items,
            key=lambda x: (
                x.artist or "",
                x.album or "",
                x.track_number or 0
            )
        )
```



### Phase 4: Implement Plan Executor

#### Step 4.1: Create PlanExecutor Class

Execute the optimized plan:

```python
# In core/plan_executor.py
from core.models import DownloadPlan, PlanItem, PlanItemStatus
from core.downloader import Downloader
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import Dict
import logging

logger = logging.getLogger(__name__)

class PlanExecutor:
    """Executes download plans."""
    
    def __init__(self, downloader: Downloader):
        self.downloader = downloader
    
    def execute(self, plan: DownloadPlan, max_workers: int = None) -> DownloadPlan:
        """Execute download plan."""
        if max_workers is None:
            max_workers = self.downloader.config.threads
        
        # Get pending items
        pending_items = plan.get_pending_items()
        
        if not pending_items:
            logger.info("No pending items to download")
            return plan
        
        logger.info(f"Executing plan: {len(pending_items)} items to download")
        
        # Execute in parallel
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_item = {
                executor.submit(self._execute_item, item): item
                for item in pending_items
            }
            
            for future in as_completed(future_to_item):
                item = future_to_item[future]
                try:
                    result = future.result()
                    if result:
                        item.status = PlanItemStatus.COMPLETED
                        item.output_path = result
                    else:
                        item.status = PlanItemStatus.FAILED
                except Exception as e:
                    logger.error(f"Failed to execute item {item.id}: {e}")
                    item.status = PlanItemStatus.FAILED
                    item.error = str(e)
        
        return plan
    
    def _execute_item(self, item: PlanItem) -> Optional[Path]:
        """Execute a single plan item."""
        if item.type != PlanItemType.TRACK:
            logger.warning(f"Unsupported item type: {item.type}")
            return None
        
        item.status = PlanItemStatus.IN_PROGRESS
        
        try:
            success, path = self.downloader.download_track(item.url)
            if success:
                return path
            else:
                return None
        except Exception as e:
            logger.error(f"Error executing item {item.id}: {e}")
            item.error = str(e)
            return None
```



### Phase 5: Refactor Main Entry Point

#### Step 5.1: Update `download.py`

Refactor main entry point to use plan architecture:

```python
# In download.py
from core.plan_generator import PlanGenerator
from core.plan_optimizer import PlanOptimizer
from core.plan_executor import PlanExecutor

def process_downloads(config) -> Dict[str, Dict[str, int]]:
    """Orchestrate downloads using plan architecture."""
    # Initialize components
    downloader = Downloader(config.download)
    spotify = downloader.spotify
    
    # Step 1: Generate plan
    logger.info("Generating download plan...")
    generator = PlanGenerator(spotify)
    plan = generator.generate_plan(config)
    
    logger.info(f"Plan generated: {plan.get_statistics()}")
    
    # Step 2: Optimize plan
    logger.info("Optimizing download plan...")
    optimizer = PlanOptimizer(
        output_template=config.download.output,
        overwrite=config.download.overwrite
    )
    plan = optimizer.optimize(plan)
    
    logger.info(f"Plan optimized: {plan.get_statistics()}")
    
    # Step 3: Execute plan
    logger.info("Executing download plan...")
    executor = PlanExecutor(downloader)
    plan = executor.execute(plan, max_workers=config.download.threads)
    
    # Step 4: Generate results
    stats = plan.get_statistics()
    results = {
        "songs": {
            "success": stats["completed"],
            "failed": stats["failed"]
        },
        "artists": {"success": 0, "failed": 0},  # Tracked at item level
        "playlists": {"success": 0, "failed": 0},  # Tracked at item level
    }
    
    return results
```



### Phase 6: Add Plan Persistence (Optional)

#### Step 6.1: Save/Load Plans

Add ability to save and load plans:

```python
# In core/plan.py
import json
from pathlib import Path

class DownloadPlan:
    # ... existing code ...
    
    def to_dict(self) -> Dict:
        """Serialize plan to dictionary."""
        return {
            "items": [
                {
                    "id": item.id,
                    "type": item.type.value,
                    "url": item.url,
                    "name": item.name,
                    "status": item.status.value,
                    # ... other fields ...
                }
                for item in self.items
            ],
            "created_at": self.created_at,
            "config_version": self.config_version,
        }
    
    @classmethod
    def from_dict(cls, data: Dict) -> "DownloadPlan":
        """Deserialize plan from dictionary."""
        items = [
            PlanItem(
                id=item_data["id"],
                type=PlanItemType(item_data["type"]),
                url=item_data["url"],
                name=item_data["name"],
                status=PlanItemStatus(item_data["status"]),
                # ... other fields ...
            )
            for item_data in data["items"]
        ]
        
        return cls(
            items=items,
            created_at=data["created_at"],
            config_version=data["config_version"]
        )
    
    def save(self, file_path: Path) -> None:
        """Save plan to file."""
        with open(file_path, "w") as f:
            json.dump(self.to_dict(), f, indent=2)
    
    @classmethod
    def load(cls, file_path: Path) -> "DownloadPlan":
        """Load plan from file."""
        with open(file_path, "r") as f:
            data = json.load(f)
        return cls.from_dict(data)
```



## Implementation Details

### Plan Item Identification

- Use Spotify track/album/playlist IDs as unique identifiers
- Handle URL and ID formats consistently
- Track seen IDs during plan generation to detect duplicates

### Optimization Strategies

1. **Deduplication**: Remove items with same ID
2. **File Existence**: Check and skip existing files
3. **Sorting**: Organize for efficient download order
4. **Dependency Resolution**: Handle parent-child relationships

### Execution Strategy

- Parallel execution using ThreadPoolExecutor
- Progress tracking per item
- Error handling and retry logic
- Status updates throughout execution

## Testing Strategy

### Unit Tests

1. Test plan generation from config
2. Test plan optimization (deduplication, file checks)
3. Test plan execution
4. Test plan persistence

### Integration Tests

1. Test full plan workflow
2. Test with various configuration scenarios
3. Test error handling and recovery

### Performance Tests

1. Compare plan-based vs sequential execution
2. Measure plan generation time
3. Measure optimization time

## Risk Assessment

### Low Risk

- Plan data models (standard dataclasses)
- Plan generation (similar to current logic)

### Medium Risk

- Plan optimization logic (may have edge cases)
- Plan execution (parallel execution complexity)
- Breaking changes to existing functionality

### High Risk

- Major architectural change (requires comprehensive testing)
- Potential for introducing bugs in refactored code

### Mitigation Strategies

1. **Incremental Migration**: Implement alongside existing code, switch gradually
2. **Comprehensive Testing**: Full test suite before merging
3. **Feature Flag**: Allow switching between old and new architecture
4. **Rollback Plan**: Keep old code path available

## Success Criteria

1. ✅ Plan architecture implemented
2. ✅ Plan generation works for all config types
3. ✅ Plan optimization removes duplicates
4. ✅ Plan execution works in parallel
5. ✅ All existing tests pass
6. ✅ Performance equal or better than current implementation
7. ✅ Plan persistence works (if implemented)

## Rollback Plan

If issues are discovered:

1. Revert to sequential processing
2. Investigate root cause
3. Create fix branch
4. Re-test and re-deploy

## Timeline

- **Day 1-2**: Design and implement plan data models
- **Day 3-4**: Implement plan generator
- **Day 5-6**: Implement plan optimizer
- **Day 7-8**: Implement plan executor
- **Day 9-10**: Refactor main entry point, integration
- **Day 11-12**: Testing and bug fixes
- **Day 13-14**: Documentation, final validation

## Dependencies

- `dataclasses` (standard library)
- `enum` (standard library)
- `json` (standard library, for persistence)
- `concurrent.futures` (standard library, for execution)

## Related Files

- `core/models.py` - May need plan models (or create `core/plan.py`)
- `core/plan_generator.py` - New file
- `core/plan_optimizer.py` - New file
- `core/plan_executor.py` - New file
- `download.py` - Major refactoring needed
- `core/downloader.py` - May need minor updates
- `core/spotify_client.py` - Used by plan generator

## Notes for Junior Engineers

### Why Plan Architecture?

- **Optimization**: Can optimize before execution
- **Visibility**: See all work upfront
- **Control**: Better error handling and recovery
- **Efficiency**: Remove duplicates before starting

### Plan Lifecycle

1. **Generation**: Read config, create plan items
2. **Optimization**: Remove duplicates, check files
3. **Execution**: Download items in parallel
4. **Reporting**: Show results and statistics

### Key Concepts

- **Plan Item**: Single unit of work (e.g., one track)
- **Plan**: Collection of all plan items
- **Optimization**: Process plan before execution
- **Execution**: Run plan items in parallel

### Common Pitfalls

1. **Duplicate Detection**: Must use consistent IDs
2. **File Existence**: Check before marking as skipped
3. **Status Updates**: Keep status current during execution
4. **Error Handling**: Don't let one failure stop others

## Notes for Technical Management

### Business Impact

- **Performance**: Better optimization and parallelization
- **User Experience**: Better progress tracking
- **Reliability**: Better error handling and recovery
- **Maintainability**: Cleaner architecture

### Resource Requirements

- **Development Time**: 10-14 days
- **Testing Time**: 3-4 days
- **Risk**: Medium-High (major architectural change)

### Recommendation

Proceed with implementation, but consider:

1. **Phased Approach**: Implement incrementally
2. **Feature Flag**: Allow switching between architectures
3. **Comprehensive Testing**: Full test coverage before merging