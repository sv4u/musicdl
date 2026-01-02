# Overarching Development Plan for musicdl

## Executive Summary

This document consolidates all development plans for the musicdl project into a phased development stream. The plan prioritizes higher-risk architectural changes first, followed by lower-risk improvements, ensuring a stable foundation before optimization and feature additions.

**Status**: Active Development  
**Last Updated**: 2025-01-XX  
**Total Estimated Duration**: 35-50 days  
**Current Phase**: Phase 3 - Architecture Refactor (Prepared)

## Development Philosophy

1. **Higher-Risk First**: Address major architectural changes early to establish stable foundation
2. **Foundation Before Optimization**: Build reliability and core features before performance enhancements
3. **Incremental Progress**: Each phase builds upon the previous, minimizing breaking changes
4. **Risk Management**: Higher-risk items completed first to allow time for stabilization

## Phase Overview

| Phase | Focus | Duration | Risk Level | Priority |
|-------|-------|----------|------------|----------|
| **Phase 1** | Foundation Improvements | 3-5 days | Low | High |
| **Phase 2** | API Reliability | 4-5 days | Medium | High |
| **Phase 3** | Architecture Refactor | 10-14 days | Medium-High | High |
| **Phase 4** | Performance Optimization | 9-12 days | Medium | Medium-High |
| **Phase 5** | Feature Enhancements | 3-5 days | Low | Medium |
| **Phase 6** | Infrastructure & Documentation | 6-9 days | Low | Medium |

---

## Phase 1: Foundation Improvements

**Duration**: 3-5 days  
**Risk Level**: Low  
**Priority**: High  
**Goal**: Quick wins and UX improvements before major changes

### 1.1: Reduce Artist Download Scope

- **Plan**: [reduce_artist_download_scope.plan.md](./reduce_artist_download_scope.plan.md)
- **Effort**: 1-2 days
- **Risk**: Low
- **Priority**: Medium
- **Status**: ⏳ Pending

**Objectives**:

- Filter artist downloads to include only albums and singles (discography)
- Exclude compilations and "Appears On" albums
- Improve download accuracy and user experience

**Key Changes**:

- Update `core/spotify_client.py` `get_artist_albums()` method
- Add `include_groups="album,single"` parameter to Spotify API call
- Update documentation and tests

**Dependencies**: None  
**Blocks**: None

---

## Phase 2: API Reliability

**Duration**: 4-5 days  
**Risk Level**: Medium  
**Priority**: High  
**Goal**: Ensure reliable API interactions before parallelization

### 2.1: Spotipy Rate Limiting

- **Plan**: [spotipy_rate_limiting.plan.md](./spotipy_rate_limiting.plan.md)
- **Effort**: 4-5 days
- **Risk**: Medium
- **Priority**: High
- **Status**: ⏳ Pending

**Objectives**:

- Detect and handle HTTP 429 rate limit responses
- Implement exponential backoff with jitter for retries
- Respect `Retry-After` headers from Spotify API
- Implement proactive rate limiting to prevent hitting limits

**Key Changes**:

- Create `SpotifyRateLimitError` exception
- Implement retry decorator with exponential backoff
- Add `RateLimiter` class for proactive throttling
- Update `SpotifyClient` with rate limit handling
- Add configuration options for rate limiting

**Dependencies**: Phase 1 (should be complete)  
**Blocks**: Phase 4 (parallelization needs reliable API handling)

**Rationale**: Critical for Phase 4 parallelization - without proper rate limiting, parallel requests will hit API limits and fail.

---

## Phase 3: Architecture Refactor

**Duration**: 10-14 days  
**Risk Level**: Medium-High  
**Priority**: High  
**Goal**: Establish plan-based architecture for better optimization and control

### 3.1: Plan Architecture Refactor

- **Plan**: [plan_architecture_refactor.plan.md](./plan_architecture_refactor.plan.md)
- **Preparation**: [PHASE_3_PREPARATION.md](./PHASE_3_PREPARATION.md)
- **Effort**: 10-14 days
- **Risk**: Medium-High
- **Priority**: High
- **Status**: 🟡 Ready to Begin

**Objectives**:

- Transform from sequential configuration-driven to plan-based architecture
- Generate comprehensive download plan from entire configuration
- Optimize plan by removing duplicates and resolving dependencies
- Enable better parallelization and progress tracking

**Key Changes**:

- Create plan data models (`PlanItem`, `DownloadPlan`)
- Implement `PlanGenerator` class
- Implement `PlanOptimizer` class
- Implement `PlanExecutor` class
- Refactor `download.py` to use plan architecture
- Add plan persistence (optional)

**Dependencies**: Phase 2 (rate limiting ensures API reliability during plan generation)  
**Blocks**: Phase 4 (parallelization will be built on plan architecture)

**Rationale**: This is the highest-risk change and should be done early. It provides the foundation for all future optimizations and features.

**Mitigation Strategy**:

- Implement alongside existing code initially
- Use feature flag to switch between architectures
- Comprehensive testing before full migration
- Keep old code path as fallback

---

## Phase 4: Performance Optimization

**Duration**: 9-12 days  
**Risk Level**: Medium  
**Priority**: Medium-High  
**Goal**: Improve performance through caching and parallelization

### 4.1: Comprehensive Caching

- **Plan**: [add_caching.plan.md](./add_caching.plan.md)
- **Effort**: 4-5 days
- **Risk**: Low-Medium
- **Priority**: Medium-High
- **Status**: ⏳ Pending

**Objectives**:

- Cache audio search results to avoid redundant searches
- Cache file existence checks to reduce filesystem I/O
- Enhance cache implementation with thread safety and statistics
- Add persistent cache option (optional)

**Key Changes**:

- Enhance `TTLCache` with thread safety
- Add cache to `AudioProvider` for search results
- Add file existence cache to `Downloader`
- Add cache statistics and monitoring
- Add configuration options for cache settings

**Dependencies**: Phase 3 (plan architecture provides better structure for caching)  
**Blocks**: None

### 4.2: Parallelize Queries and Downloads

- **Plan**: [parallelize_queries_downloads.plan.md](./parallelize_queries_downloads.plan.md)
- **Effort**: 5-7 days
- **Risk**: Medium
- **Priority**: High
- **Status**: ⏳ Pending

**Objectives**:

- Implement parallel processing for Spotify API queries
- Implement parallel processing for audio downloads
- Respect rate limits and avoid overwhelming external services
- Provide progress reporting for parallel operations

**Key Changes**:

- Add parallel API query methods using `ThreadPoolExecutor`
- Add parallel download methods
- Refactor `download_album()`, `download_playlist()`, `download_artist()` for parallelization
- Update main orchestration in `download.py`
- Add progress reporting with `tqdm` (optional)

**Dependencies**:

- Phase 2 (rate limiting required for parallel API calls)
- Phase 3 (plan architecture provides better structure for parallelization)
- Phase 4.1 (caching reduces redundant operations)

**Blocks**: None

**Rationale**: Parallelization provides 60-80% performance improvement but requires reliable rate limiting and plan architecture for optimal results.

---

## Phase 5: Feature Enhancements

**Duration**: 3-5 days  
**Risk Level**: Low  
**Priority**: Medium  
**Goal**: Add user-requested features

### 5.1: Add Album Download Support

- **Plan**: [add_album_download_support.plan.md](./add_album_download_support.plan.md)
- **Effort**: 2-3 days
- **Risk**: Low
- **Priority**: Medium
- **Status**: ⏳ Pending

**Objectives**:

- Add `albums` section to configuration model
- Support both simple and extended album entry formats
- Add per-album M3U creation configuration
- Integrate album processing into main download orchestration

**Key Changes**:

- Create `AlbumSource` model extending `MusicSource`
- Update `MusicDLConfig` to include `albums` field
- Update config parsing to handle album formats
- Update `download_album()` to support M3U creation
- Update `process_downloads()` to handle albums

**Dependencies**: Phase 3 (plan architecture will handle albums naturally)  
**Blocks**: None

---

## Phase 6: Infrastructure & Documentation

**Duration**: 6-9 days  
**Risk Level**: Low  
**Priority**: Medium  
**Goal**: Optimize infrastructure and improve documentation

### 6.1: Docker Slim Image

- **Plan**: [docker_slim_image.plan.md](./docker_slim_image.plan.md)
- **Effort**: 2-3 days
- **Risk**: Low-Medium
- **Priority**: Medium
- **Status**: ⏳ Pending

**Objectives**:

- Reduce Docker image size by 50-70% using slim base image
- Maintain 100% functional compatibility
- Improve build times and reduce storage/bandwidth costs

**Key Changes**:

- Update `musicdl.Dockerfile` to use `python:3.12-slim`
- Install required system dependencies explicitly
- Test all functionality with slim image
- Update CI/CD workflows if needed
- Update documentation

**Dependencies**: None (can be done independently)  
**Blocks**: None

### 6.2: Structured Changelog Generation

- **Plan**: [structured_changelog_generation.plan.md](./structured_changelog_generation.plan.md)
- **Effort**: 3-4 days
- **Risk**: Low
- **Priority**: Medium
- **Status**: ⏳ Pending

**Objectives**:

- Implement structured changelog generation with categorization
- Group changes by type (feat, fix, docs, etc.) and scope
- Highlight breaking changes prominently
- Include PR and issue links in changelog
- Maintain CHANGELOG.md file in repository

**Key Changes**:

- Install and configure `git-cliff`
- Create `.cliff.toml` configuration file
- Update release workflow to use git-cliff
- Add CHANGELOG.md maintenance
- Update documentation

**Dependencies**: None (can be done independently)  
**Blocks**: None

---

## Completed Work

### ✅ Re-implement spotDL for musicdl

- **Plan**: [re-implement_spotdl_for_musicdl.plan.md](./re-implement_spotdl_for_musicdl.plan.md)
- **Status**: ✅ Complete
- **Notes**: Native Python implementation complete, no spotDL dependency

---

## Development Timeline

### Immediate (Week 1-2)

- **Phase 1**: Foundation Improvements (3-5 days)
  - Reduce artist download scope (1-2 days)
- **Phase 2**: API Reliability (4-5 days)
  - Spotipy rate limiting (4-5 days)

### Short-term (Week 3-5)

- **Phase 3**: Architecture Refactor (10-14 days)
  - Plan architecture refactor (10-14 days)

### Medium-term (Week 6-8)

- **Phase 4**: Performance Optimization (9-12 days)
  - Comprehensive caching (4-5 days)
  - Parallelize queries and downloads (5-7 days)

### Long-term (Week 9-10)

- **Phase 5**: Feature Enhancements (3-5 days)
  - Add album download support (2-3 days)
- **Phase 6**: Infrastructure & Documentation (6-9 days)
  - Docker slim image (2-3 days)
  - Structured changelog generation (3-4 days)

**Total Estimated Duration**: 35-50 days (7-10 weeks)

---

## Dependency Graph

```
Phase 1 (Foundation)
  └─> Phase 2 (API Reliability)
        └─> Phase 3 (Architecture Refactor)
              ├─> Phase 4.1 (Caching)
              └─> Phase 4.2 (Parallelization)
                    └─> Phase 5 (Features)
                          └─> Phase 6 (Infrastructure)
```

**Key Dependencies**:

- Phase 2 (Rate Limiting) is required before Phase 4.2 (Parallelization)
- Phase 3 (Plan Architecture) provides foundation for Phase 4 optimizations
- Phase 4.1 (Caching) should complete before Phase 4.2 (Parallelization) for optimal results
- Phase 5 and Phase 6 can proceed in parallel after Phase 4

---

## Risk Management

### High-Risk Items

1. **Phase 3: Plan Architecture Refactor** (Medium-High Risk)
   - **Mitigation**: Feature flag, incremental migration, comprehensive testing
   - **Rollback**: Keep old code path available

### Medium-Risk Items

1. **Phase 2: Spotipy Rate Limiting** (Medium Risk)
   - **Mitigation**: Comprehensive testing, gradual rollout, monitoring
   - **Rollback**: Disable via configuration

2. **Phase 4.2: Parallelization** (Medium Risk)
   - **Mitigation**: Build on stable rate limiting, test with various workloads
   - **Rollback**: Revert to sequential processing

### Low-Risk Items

- Phase 1, Phase 4.1, Phase 5, Phase 6
- Standard patterns, well-understood implementations

---

## Success Criteria

### Phase 1 Success

- ✅ Artist downloads exclude compilations and "Appears On" albums
- ✅ All existing tests pass
- ✅ Documentation updated

### Phase 2 Success

- ✅ HTTP 429 errors detected and handled
- ✅ Exponential backoff with jitter implemented
- ✅ Retry-After headers respected
- ✅ Proactive rate limiting prevents hitting limits
- ✅ No increase in API failures

### Phase 3 Success

- ✅ Plan architecture implemented
- ✅ Plan generation works for all config types
- ✅ Plan optimization removes duplicates
- ✅ Plan execution works in parallel
- ✅ All existing tests pass
- ✅ Performance equal or better than current implementation

### Phase 4 Success

- ✅ Audio search results cached
- ✅ File existence checks cached
- ✅ Parallelization implemented for API queries
- ✅ Parallelization implemented for downloads
- ✅ 60-80% performance improvement for large workloads
- ✅ Rate limiting prevents API throttling

### Phase 5 Success

- ✅ Albums can be specified in configuration file
- ✅ Both simple and extended formats supported
- ✅ Per-album M3U creation works
- ✅ All existing tests pass

### Phase 6 Success

- ✅ Image size reduced by at least 50%
- ✅ All existing tests pass
- ✅ Structured changelog with categorized sections
- ✅ Breaking changes highlighted prominently

---

## Monitoring and Tracking

### Progress Tracking

- Update TODO.md as phases complete
- Mark plans as complete in `.cursor/plans/` directory
- Update this document with status changes

### Key Metrics

- **Test Coverage**: Maintain >80% coverage
- **Performance**: Measure before/after for optimization phases
- **API Reliability**: Track rate limit events and failures
- **Build Times**: Monitor Docker build times

### Decision Points

- After Phase 2: Evaluate rate limiting effectiveness
- After Phase 3: Assess plan architecture stability before proceeding
- After Phase 4: Measure performance improvements
- After Phase 5: Review feature completeness

---

## Notes

### Why This Order?

1. **Phase 1 First**: Quick win, low risk, improves UX immediately
2. **Phase 2 Second**: Critical for parallelization, prevents API failures
3. **Phase 3 Third**: Highest risk, done early to allow stabilization time
4. **Phase 4 Fourth**: Performance optimizations built on stable foundation
5. **Phase 5 Fifth**: Features added after core architecture is stable
6. **Phase 6 Last**: Infrastructure improvements, can be done in parallel

### Flexibility

- Phases can be adjusted based on findings during development
- Some items in Phase 6 can proceed in parallel with Phase 5
- Phase 4.1 (Caching) and Phase 4.2 (Parallelization) should be sequential

### Communication

- Update this plan as development progresses
- Document any deviations or discoveries
- Keep TODO.md synchronized with actual progress

---

## Related Documents

- [TODO.md](../../TODO.md) - Current task list
- Individual plan documents in `.cursor/plans/` directory
- [README.md](../../README.md) - Project documentation

---

**Next Steps**: Begin Phase 1.1 - Reduce Artist Download Scope
