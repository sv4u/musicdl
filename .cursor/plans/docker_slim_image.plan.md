# Plan: Make Slim Docker Image the Default

## Executive Summary

This plan outlines the migration from the current full Debian-based Docker image (`python:3.12-bookworm`) to a slim variant (`python:3.12-slim`) as the default build. This change will significantly reduce image size (estimated 50-70% reduction), improve build times, and reduce attack surface while maintaining full functionality.**Target Audience**: Technical leads, junior engineers, and technical management**Estimated Effort**: 2-3 days**Risk Level**: Low-Medium**Priority**: Medium

## Current State Analysis

### Current Dockerfile Structure

- **Base Image**: `python:3.12-bookworm` (full Debian image, ~900MB+)
- **System Dependencies**: Installed via `apt-get`:
- `ca-certificates`, `curl`, `ffmpeg`, `openssl`, `aria2`, `g++`, `git`, `python3-cffi`, `libffi-dev`, `zlib1g-dev`
- **Python Dependencies**: Installed from `requirements.txt`
- **Build Context**: Includes `download.py`, `core/`, and `config.yaml`

### Current Image Size

- Estimated base image: ~900MB (python:3.12-bookworm)
- With dependencies: ~1.2-1.5GB
- Final image size: ~1.3-1.6GB

### Dependencies Analysis

All current system dependencies are required:

- **ffmpeg**: Required for audio processing and format conversion
- **aria2**: Used by yt-dlp for faster downloads
- **g++**: Required for compiling Python C extensions (if any)
- **git**: May be required for some Python packages during installation
- **python3-cffi, libffi-dev, zlib1g-dev**: Required for Python package compilation
- **ca-certificates, openssl**: Required for HTTPS connections
- **curl**: May be used by some tools

## Objectives

1. **Primary**: Reduce Docker image size by 50-70% using slim base image
2. **Secondary**: Maintain 100% functional compatibility with current image
3. **Tertiary**: Improve build times and reduce storage/bandwidth costs
4. **Security**: Reduce attack surface by using minimal base image

## Technical Approach

### Phase 1: Create Slim Dockerfile Variant

#### Step 1.1: Create New Slim Dockerfile

- **File**: `musicdl.Dockerfile.slim` (temporary, for testing)
- **Base Image**: `python:3.12-slim` (~150MB base)
- **Strategy**: Multi-stage build (optional, for further optimization)

#### Step 1.2: Install System Dependencies

The slim image requires explicit installation of build dependencies:

```dockerfile
# Install build dependencies and runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Build dependencies (can be removed after pip install)
    g++ \
    git \
    python3-cffi \
    libffi-dev \
    zlib1g-dev \
    # Runtime dependencies
    ca-certificates \
    curl \
    ffmpeg \
    openssl \
    aria2 \
    && rm -rf /var/lib/apt/lists/*
```

**Note**: Consider multi-stage build to remove build dependencies after pip install.

#### Step 1.3: Test Dependency Installation

Verify all Python packages install correctly:

- `spotipy` (may require SSL/certificates)
- `yt-dlp` (requires ffmpeg, aria2)
- `mutagen` (may require zlib)
- `pydantic` (pure Python, should work)
- `PyYAML` (may require C extensions)
- `requests` (requires ca-certificates)

### Phase 2: Validation and Testing

#### Step 2.1: Build and Size Comparison

- Build both images (full and slim)
- Compare sizes: `docker images`
- Document size reduction percentage

#### Step 2.2: Functional Testing

Run existing test suite against slim image:

- **Unit Tests**: `pytest tests/unit/`
- **Integration Tests**: `pytest tests/integration/`
- **E2E Tests**: `pytest tests/e2e/`
- **Smoke Tests**: Verify container startup and basic functionality

#### Step 2.3: Runtime Validation

Test actual download scenarios:

- Download single track
- Download playlist
- Download artist albums
- Verify metadata embedding
- Verify file format conversion

### Phase 3: Multi-Stage Build Optimization (Optional)

#### Step 3.1: Create Multi-Stage Dockerfile

Separate build and runtime stages:

- **Stage 1 (builder)**: Install build dependencies, compile Python packages
- **Stage 2 (runtime)**: Copy only runtime artifacts, exclude build tools

#### Step 3.2: Optimize Layer Caching

- Order Dockerfile commands by change frequency
- Separate dependency installation from code copying
- Use `.dockerignore` to exclude unnecessary files

### Phase 4: Migration Strategy

#### Step 4.1: Update Main Dockerfile

- Replace `python:3.12-bookworm` with `python:3.12-slim`
- Update system package installation
- Test thoroughly

#### Step 4.2: Update CI/CD Workflows

- Verify GitHub Actions workflows work with slim image
- Update any size-based checks if needed
- Update documentation references

#### Step 4.3: Update Documentation

- Update `README.md` with new image size information
- Update `truenas-scale-deployment.md` if applicable
- Add migration notes if breaking changes exist

## Implementation Details

### Dockerfile Changes

**Current**:

```dockerfile
FROM python:3.12-bookworm
```

**Proposed**:

```dockerfile
FROM python:3.12-slim

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    ffmpeg \
    openssl \
    aria2 \
    g++ \
    git \
    python3-cffi \
    libffi-dev \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*
```



### Multi-Stage Build Option

For further optimization:

```dockerfile
# Stage 1: Build dependencies
FROM python:3.12-slim as builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    g++ \
    git \
    python3-cffi \
    libffi-dev \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt /tmp/requirements.txt
RUN pip install --user --no-cache-dir -r /tmp/requirements.txt

# Stage 2: Runtime
FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    ffmpeg \
    openssl \
    aria2 \
    && rm -rf /var/lib/apt/lists/*

# Copy Python packages from builder
COPY --from=builder /root/.local /root/.local

# Make sure scripts in .local are usable
ENV PATH=/root/.local/bin:$PATH

# ... rest of Dockerfile
```



## Testing Strategy

### Automated Testing

1. **Build Test**: Verify image builds successfully
2. **Size Test**: Assert image size is below threshold (e.g., <800MB)
3. **Functionality Test**: Run full test suite in container
4. **Integration Test**: Test with real Spotify API calls

### Manual Testing Checklist

- [ ] Container starts successfully
- [ ] Entrypoint script executes
- [ ] Python environment is correct
- [ ] All system tools available (ffmpeg, aria2, etc.)
- [ ] Spotify API authentication works
- [ ] Audio download works
- [ ] Metadata embedding works
- [ ] File format conversion works
- [ ] M3U playlist generation works

## Risk Assessment

### Low Risk

- Base image change (slim is well-maintained)
- System package installation (standard Debian packages)

### Medium Risk

- Python package compilation issues (some packages may need build tools)
- Missing system libraries (may cause runtime errors)
- SSL/certificate issues (may affect HTTPS connections)

### Mitigation Strategies

1. **Comprehensive Testing**: Run full test suite before merging
2. **Gradual Rollout**: Test in development/staging first
3. **Fallback Plan**: Keep old Dockerfile as `musicdl.Dockerfile.full` for reference
4. **Documentation**: Document any known issues or workarounds

## Success Criteria

1. ✅ Image size reduced by at least 50%
2. ✅ All existing tests pass
3. ✅ No functional regressions
4. ✅ Build time acceptable (<5 minutes)
5. ✅ Documentation updated

## Rollback Plan

If issues are discovered:

1. Revert Dockerfile to `python:3.12-bookworm`
2. Investigate root cause
3. Create fix branch
4. Re-test and re-deploy

## Timeline

- **Day 1**: Create slim Dockerfile variant, initial testing
- **Day 2**: Comprehensive testing, fix any issues
- **Day 3**: Update main Dockerfile, CI/CD, documentation, final validation

## Dependencies

- No external dependencies
- Requires access to Docker build environment
- Requires test suite to be passing

## Related Files

- `musicdl.Dockerfile` - Main Dockerfile to update
- `.github/workflows/docker-build-and-test.yml` - CI/CD workflow
- `README.md` - Documentation
- `truenas-scale-deployment.md` - Deployment documentation
- `Makefile` - Build commands (if any Docker-related targets)

## Notes for Junior Engineers

### Why Slim Images?

- **Smaller Size**: Faster downloads, less storage
- **Security**: Fewer packages = smaller attack surface
- **Performance**: Faster container startup times

### Common Pitfalls

1. **Missing Dependencies**: Some Python packages need system libraries
2. **Build Tools**: Need `g++` and dev packages for compilation, but can remove after
3. **SSL Issues**: Ensure `ca-certificates` is installed for HTTPS

### Debugging Tips

- Use `docker run -it <image> /bin/bash` to inspect container
- Check installed packages: `dpkg -l`
- Test Python imports: `python3 -c "import spotipy"`
- Verify tools: `ffmpeg -version`, `aria2c --version`

## Notes for Technical Management

### Business Impact

- **Cost Reduction**: Smaller images = less storage/bandwidth costs
- **Performance**: Faster deployments and container pulls
- **Security**: Reduced attack surface improves security posture

### Resource Requirements

- **Development Time**: 2-3 days