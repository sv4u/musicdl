# Phase 3: Architecture Refactor - Preparation Guide

## Overview

**Phase**: 3.1 - Plan Architecture Refactor  
**Duration**: 10-14 days  
**Risk Level**: Medium-High  
**Priority**: High  
**Status**: 🟡 Ready to Begin

## Prerequisites Check

### ✅ Phase 2 Complete

- [x] Rate limiting implemented (`RateLimiter` class)
- [x] `SpotifyRateLimitError` exception exists
- [x] Rate limiting integrated into `SpotifyClient`
- [x] Configuration options available
- [x] Tests passing (96 passed, 21 skipped)

### Current Codebase State

**Key Files to Understand:**

- `download.py` - Current sequential orchestration (lines 43-108)
- `core/downloader.py` - Download logic (379 lines)
- `core/config.py` - Configuration models (205 lines)
- `core/spotify_client.py` - Spotify API client with rate limiting (413 lines)
- `core/models.py` - Current data models (Song, DownloadResult)

**Current Architecture:**

```
Config → Sequential Processing:
  1. Process songs (one by one)
  2. Process artists (one by one, then albums, then tracks)
  3. Process playlists (one by one, then tracks)
```

**Target Architecture:**

```
Config → Plan Generation → Plan Optimization → Plan Execution
  ↓           ↓                    ↓                    ↓
All items   All items         Deduplicated        Parallel
identified  in plan          & optimized         execution
```

## Implementation Plan

### Phase 1: Design Plan Data Models (Days 1-2)

**Files to Create/Modify:**

- `core/plan.py` - New file for plan models
  - `PlanItemType` (Enum)
  - `PlanItemStatus` (Enum)
  - `PlanItem` (dataclass)
  - `DownloadPlan` (dataclass)

**Key Design Decisions:**

- Use Spotify IDs as unique identifiers
- Track parent-child relationships (album → tracks)
- Support status tracking (pending, in_progress, completed, failed, skipped)
- Include metadata for optimization and reporting

### Phase 2: Implement Plan Generator (Days 3-4)

**Files to Create:**

- `core/plan_generator.py` - New file
  - `PlanGenerator` class
  - Methods: `generate_plan()`, `_process_song()`, `_process_artist()`, `_process_album()`, `_process_playlist()`

**Key Features:**

- Read entire configuration
- Generate plan items for all sources
- Fetch metadata during planning
- Track duplicates during generation
- Handle pagination for albums/playlists

### Phase 3: Implement Plan Optimizer (Days 5-6)

**Files to Create:**

- `core/plan_optimizer.py` - New file
  - `PlanOptimizer` class
  - Methods: `optimize()`, `_remove_duplicates()`, `_check_existing_files()`, `_sort_items()`

**Key Features:**

- Remove duplicate items (by Spotify ID)
- Check file existence and mark as skipped
- Sort items for optimal download order
- Generate statistics

### Phase 4: Implement Plan Executor (Days 7-8)

**Files to Create:**

- `core/plan_executor.py` - New file
  - `PlanExecutor` class
  - Methods: `execute()`, `_execute_item()`

**Key Features:**

- Execute plan items in parallel
- Update item status during execution
- Handle errors gracefully
- Support progress tracking

### Phase 5: Refactor Main Entry Point (Days 9-10)

**Files to Modify:**

- `download.py` - Refactor `process_downloads()` function
  - Replace sequential processing with plan-based flow
  - Integrate generator, optimizer, executor
  - Update result reporting

**Migration Strategy:**

- Implement alongside existing code
- Use feature flag for gradual migration
- Keep old code path as fallback

### Phase 6: Testing & Validation (Days 11-12)

**Test Coverage Needed:**

- Unit tests for plan models
- Unit tests for plan generator
- Unit tests for plan optimizer
- Unit tests for plan executor
- Integration tests for full workflow
- Performance comparison tests

**Files to Create:**

- `tests/unit/test_plan.py`
- `tests/unit/test_plan_generator.py`
- `tests/unit/test_plan_optimizer.py`
- `tests/unit/test_plan_executor.py`
- `tests/integration/test_plan_workflow.py`

### Phase 7: Documentation & Finalization (Days 13-14)

**Tasks:**

- Update README with plan architecture explanation
- Add docstrings to all new classes/methods
- Update code comments
- Performance benchmarking
- Final validation

## Key Implementation Details

### Plan Item Identification

**Strategy:**

- Extract Spotify IDs from URLs using regex
- Use IDs as unique identifiers
- Handle both URL and ID formats consistently

**Example:**

```python
def _extract_track_id(url: str) -> str:
    match = re.search(r'track/([a-zA-Z0-9]+)', url)
    return match.group(1) if match else url
```

### Duplicate Detection

**Strategy:**

- Track seen IDs during plan generation
- Use set for O(1) lookup
- Skip items with duplicate IDs

### File Existence Checking

**Strategy:**

- Format filename using template and song metadata
- Check if file exists
- Mark as skipped if exists and `overwrite == "skip"`

### Parallel Execution

**Strategy:**

- Use `ThreadPoolExecutor` for parallel downloads
- Respect `config.download.threads` setting
- Update item status atomically
- Handle errors per item (don't stop entire plan)

## Risk Mitigation

### 1. Feature Flag Approach

```python
# In config.py
use_plan_architecture: bool = False  # Feature flag

# In download.py
if config.download.use_plan_architecture:
    # New plan-based flow
else:
    # Old sequential flow
```

### 2. Incremental Migration

- Implement plan architecture alongside existing code
- Test thoroughly before switching
- Keep old code path available for rollback

### 3. Comprehensive Testing

- Maintain >80% test coverage
- Test all configuration scenarios
- Performance benchmarking

## Success Criteria

- [ ] Plan architecture implemented
- [ ] Plan generation works for all config types (songs, artists, playlists)
- [ ] Plan optimization removes duplicates
- [ ] Plan optimization checks file existence
- [ ] Plan execution works in parallel
- [ ] All existing tests pass
- [ ] New tests added with >80% coverage
- [ ] Performance equal or better than current implementation
- [ ] Documentation updated

## Dependencies

**Python Standard Library:**

- `dataclasses` - For plan models
- `enum` - For plan item types and statuses
- `json` - For plan persistence (optional)
- `concurrent.futures` - For parallel execution
- `re` - For ID extraction
- `time` - For timestamps

**Existing Dependencies:**

- `core.spotify_client.SpotifyClient` - For API calls
- `core.downloader.Downloader` - For actual downloads
- `core.config.MusicDLConfig` - For configuration
- `core.models.Song` - For metadata

## File Structure After Implementation

```
core/
  ├── __init__.py
  ├── plan.py                    # NEW: Plan data models
  ├── plan_generator.py         # NEW: Plan generation logic
  ├── plan_optimizer.py         # NEW: Plan optimization logic
  ├── plan_executor.py          # NEW: Plan execution logic
  ├── downloader.py             # Existing (may need minor updates)
  ├── spotify_client.py         # Existing (used by generator)
  ├── config.py                 # Existing
  ├── models.py                 # Existing
  └── ...

download.py                     # MODIFIED: Use plan architecture

tests/
  ├── unit/
  │   ├── test_plan.py          # NEW
  │   ├── test_plan_generator.py # NEW
  │   ├── test_plan_optimizer.py # NEW
  │   └── test_plan_executor.py  # NEW
  └── integration/
      └── test_plan_workflow.py  # NEW
```

## Next Steps

1. **Review this preparation guide** ✅
2. **Create plan data models** (`core/plan.py`)
3. **Implement plan generator** (`core/plan_generator.py`)
4. **Implement plan optimizer** (`core/plan_optimizer.py`)
5. **Implement plan executor** (`core/plan_executor.py`)
6. **Refactor download.py** to use plan architecture
7. **Add comprehensive tests**
8. **Update documentation**

## Questions to Consider

1. **Plan Persistence**: Should we save/load plans to disk? (Optional feature)
2. **Progress Reporting**: How detailed should progress tracking be?
3. **Error Recovery**: Should we support resuming failed plans?
4. **M3U Creation**: How should playlist M3U files be handled in plan architecture?

## Notes

- This is a major architectural change - proceed carefully
- Test thoroughly at each phase
- Keep old code path available for rollback
- Maintain backward compatibility where possible
- Focus on correctness before optimization

---

**Ready to begin Phase 3 implementation!** 🚀
