# Reimplement CI/CD Workflows - Implementation Plan

## Context

The musicdl project currently has 5 separate GitHub Actions workflows that need to be simplified and consolidated into 4 high-level, maintainable workflows. The current workflows are:

1. `test.yml` - Runs unit tests
2. `coverage.yml` - Generates code coverage reports
3. `docker-build.yml` - Builds Docker images (no testing)
4. `release.yml` - Creates GitHub releases with changelog generation
5. `docker-publish.yml` - Publishes Docker images to GHCR

## Goal

Create a detailed implementation plan to consolidate these into 4 streamlined workflows:

1. **Test & Coverage Workflow** - Combined unit tests and coverage reporting
2. **Docker Build & Test Workflow** - Build Docker images and validate they function correctly
3. **Release & Publish Workflow** - Combined GitHub release creation and Docker image publishing
4. **Security & SBOM Workflow** - Security scanning and SBOM generation for both source code and Docker images

## Requirements

### Workflow 1: Test & Coverage

**Purpose**: Run unit tests and generate coverage reports in a single workflow

**Triggers**:

- Pull requests (opened, synchronize, reopened)
- Pushes to `main` branch

**Requirements**:

- Combine functionality from `test.yml` and `coverage.yml`
- Run pytest with coverage enabled
- Generate both HTML and XML coverage reports
- Upload coverage artifacts (HTML and XML)
- Display coverage summary in workflow logs
- Use Python 3.12
- Cache pip dependencies for faster runs
- Support concurrency (cancel in-progress runs)

**Outputs**:

- Test results (pass/fail)
- Coverage HTML report (uploaded as artifact)
- Coverage XML report (uploaded as artifact)
- Coverage percentage in logs

### Workflow 2: Docker Build & Test

**Purpose**: Build Docker images and validate they function correctly

**Triggers**:

- Pull requests (opened, synchronize, reopened)
- Pushes to `main` branch

**Requirements**:

- Build Docker image using `musicdl.Dockerfile`
- Tag images appropriately:
  - PRs: `ghcr.io/sv4u/musicdl:pr-{number}`
  - Main branch: `ghcr.io/sv4u/musicdl:sha-{short-sha}` and `ghcr.io/sv4u/musicdl:latest`
- **Docker Image Testing** (NEW - currently missing):
  - Verify container can start successfully
  - Test that entrypoint script executes
  - Verify Python environment is set up correctly
  - Test that required dependencies are installed (ffmpeg, aria2, etc.)
  - Optionally: Run a smoke test with a minimal test configuration
  - Verify file permissions and directory structure
- Save Docker image as artifact for PRs (for manual testing)
- Use Docker Buildx for multi-platform support (if needed)
- Add OCI labels (source, revision, created timestamp)
- Support concurrency (cancel in-progress runs)

**Outputs**:

- Built Docker image (loaded locally for testing)
- Docker image artifact (for PRs)
- Test results (container validation tests)

### Workflow 3: Release & Publish

**Purpose**: Create GitHub releases and publish Docker images

**Triggers**:

- Manual dispatch (`workflow_dispatch`) with inputs:
  - `release_type`: choice (major, minor, hotfix)
  - `dry_run`: boolean (default: false)

**Requirements**:

- **Release Creation** (from `release.yml`):
  - Verify branch is `main` and working directory is clean
  - Verify local branch is up-to-date with remote `origin/main`
  - Fetch all tags to ensure latest tags are available
  - Get current version from latest git tag (format: `vX.Y`, e.g., `v0.13`)
  - Calculate next version based on release type:
    - **major**: Increment major version, reset minor to 0 (e.g., `v0.13` → `v1.0`)
    - **minor**: Increment minor version (e.g., `v0.13` → `v0.14`)
    - **hotfix**: Increment minor version (same as minor for two-part versioning)
  - Handle first release: If no tags exist, default to `v0.1` regardless of release type
  - Validate version format: Must match `vX.Y` pattern (where X and Y are non-negative integers)
  - Validate tag name format and length (Git limit: 255 characters)
  - Validate commit range (must have commits to release, prevents duplicate releases)
  - **Tag Existence Checking**:
    - Check if calculated tag already exists locally (fail if exists, warn in dry-run)
    - Check if calculated tag already exists on remote (fail if exists, warn in dry-run)
    - Prevent duplicate tag creation
  - Create annotated git tag with descriptive message:
    - Include release type, date, commit count
    - Reference previous tag (if not first release)
    - Validate tag message length (Git limit: ~64KB, use conservative 60KB)
  - Push tag to remote repository
  - Create GitHub release with auto-generated changelog using GitHub CLI
  - Support dry-run mode (validate without creating releases/tags)
  - **Tag Rollback on Failure**:
    - If release process fails after tag creation, automatically rollback:
      - Delete local tag if it exists
      - Delete remote tag if it exists
      - Provide manual cleanup instructions if automatic rollback fails
    - Ensure clean state for retry
  - Verify release was created successfully (using GitHub CLI or API)
  - Display release summary with tag, version, commit count, and release URL

- **Docker Publishing** (from `docker-publish.yml`):
  - Build Docker image with versioned tag
  - Tag with both version tag and `latest`
  - Push to GHCR (`ghcr.io/sv4u/musicdl:{version}` and `ghcr.io/sv4u/musicdl:latest`)
  - Verify image was published successfully
  - Add OCI labels (source, revision, created, version)

- **Integration**:
  - Combine both processes into a single workflow
  - Ensure Docker publishing happens after successful release creation
  - Handle failures gracefully (rollback if needed)
  - Support dry-run mode for both release and Docker publishing

**Outputs**:

- GitHub release with changelog
- Published Docker images (versioned and latest)
- Release summary with URLs

### Workflow 4: Security & SBOM

**Purpose**: Security scanning and SBOM generation for source code and Docker images

**Triggers**:

- Pull requests (opened, synchronize, reopened)
- Pushes to `main` branch
- Release events (when releases are published)

**Requirements**:

**Source Code Security Scanning**:

- Use **Trivy** to scan source code for vulnerabilities
- Scan dependency files (`requirements.txt`, `test-requirements.txt`)
- Generate security report
- Upload scan results as artifact
- Fail workflow on critical/high severity vulnerabilities (configurable)

**Docker Image Security Scanning**:

- Use **Trivy** to scan Docker images for vulnerabilities
- Use **Grype** as a secondary scanner for comparison
- Scan images built in Docker Build & Test workflow
- For releases: scan the published image
- Generate security reports from both scanners
- Upload scan results as artifacts
- Compare results from both scanners
- Fail workflow on critical/high severity vulnerabilities (configurable)

**SBOM Generation from Source Code**:

- Use **Syft** to generate SBOM from source code
- Use **GitHub SBOM** (native GitHub feature) if available
- Generate SBOM in CycloneDX format
- Generate SBOM in SPDX format
- Include all dependencies from `requirements.txt` and `test-requirements.txt`
- Upload SBOM artifacts

**SBOM Generation from Docker Image**:

- Use **Syft** to generate SBOM from Docker image
- Use **Trivy** to generate SBOM from Docker image (alternative format)
- Generate SBOMs in multiple formats (CycloneDX, SPDX)
- Include all packages installed in the Docker image
- Upload SBOM artifacts
- Attach SBOMs to GitHub releases (if triggered by release event)

**Artifacts & Reporting**:

- Upload all security scan reports as artifacts
- Upload all SBOM files as artifacts
- Display summary of vulnerabilities found
- Display summary of packages in SBOMs
- Support concurrency (cancel in-progress runs)

**Outputs**:

- Trivy source code scan report
- Trivy Docker image scan report
- Grype Docker image scan report
- Syft source code SBOM (CycloneDX, SPDX)
- GitHub SBOM (if available)
- Syft Docker image SBOM (CycloneDX, SPDX)
- Trivy Docker image SBOM

## Implementation Details

### Tool Versions & Actions

**Security Scanning**:

- Trivy: `aquasecurity/trivy-action@v0.29.0` (or latest stable)
- Grype: `anchore/grype-action@v1.0.0` (or latest stable)

**SBOM Generation**:

- Syft: `anchore/sbom-action@v0.15.0` (or latest stable)
- GitHub SBOM: Use GitHub's native SBOM generation API/feature
- Trivy: Can also generate SBOMs (use `--format cyclonedx` or `--format spdx`)

### Version Calculation & Tag Handling

**Version Format**:

- musicdl uses two-part versioning: `vX.Y` (e.g., `v0.13`, `v1.0`)
- This differs from semantic versioning (`vX.Y.Z`) used in some other projects
- Tags must match the pattern: `v[0-9]+\.[0-9]+`

**Version Calculation Logic**:

- Parse latest tag to extract major and minor components
- For first release (no tags exist): Default to `v0.1` regardless of release type
- For major releases: Increment major, reset minor to 0 (e.g., `v0.13` → `v1.0`)
- For minor/hotfix releases: Increment minor (e.g., `v0.13` → `v0.14`)
- Validate calculated version format before proceeding

**Tag Validation**:

- Check tag format matches `vX.Y` pattern using regex: `^v[0-9]+\.[0-9]+$`
- Validate tag name length (max 255 characters per Git specification)
- Validate tag message length (max 60KB to be conservative, Git limit is ~64KB)

**Tag Existence Checking**:

- Check local tags: `git rev-parse --verify "$TAG"`
- Check remote tags: `git ls-remote --tags origin "$TAG"`
- Fail workflow if tag exists (warn in dry-run mode)
- Prevents duplicate tag creation and release conflicts

**Tag Rollback**:

- Delete local tag: `git tag -d "$TAG"`
- Delete remote tag: `git push origin ":refs/tags/$TAG"`
- Execute rollback if release process fails after tag creation
- Provide manual cleanup instructions if automatic rollback fails

### Workflow Structure

Each workflow should:

- Use appropriate permissions (minimal required)
- Support concurrency where applicable
- Have clear job and step names
- Include error handling
- Upload artifacts with appropriate retention periods
- Display clear summaries and status messages

### Migration Strategy

1. Create new workflows alongside existing ones
2. Test new workflows thoroughly
3. Update documentation
4. Remove old workflows once new ones are validated

## Deliverables

Create a detailed implementation plan that includes:

1. **Workflow File Structure**: Complete YAML structure for each of the 4 workflows
2. **Step-by-Step Implementation**: Detailed steps for each workflow with explanations
3. **Tool Configuration**: Specific configurations for Trivy, Grype, Syft, and GitHub SBOM
4. **Artifact Management**: What artifacts to upload, retention periods, naming conventions
5. **Error Handling**: How to handle failures, rollbacks, and edge cases
6. **Testing Strategy**: How to test the new workflows before removing old ones
7. **Documentation Updates**: What README/documentation needs to be updated
8. **Migration Checklist**: Step-by-step checklist for migrating from old to new workflows

## Tag Handling Requirements (Decided)

The following tag handling requirements have been determined for the musicdl repository:

1. **Tag Format**: Two-part versioning (`vX.Y` format, e.g., `v0.13`, `v1.0`)
   - Different from semantic versioning (`vX.Y.Z`)
   - Major version increments reset minor to 0 (e.g., `v0.13` → `v1.0`)
   - Minor version increments increment the minor part (e.g., `v0.13` → `v0.14`)
   - Hotfix releases increment minor version (same as minor for two-part versioning)

2. **Tag Existence Checking**: ✅ Required
   - Check both local and remote tags before creation
   - Fail workflow if tag exists (warn in dry-run mode)
   - Prevents duplicate tag creation

3. **Tag Rollback**: ✅ Required
   - Automatically delete tags (local and remote) if release process fails
   - Provide manual cleanup instructions if automatic rollback fails
   - Ensures clean state for retry

4. **Tag Validation**: ✅ Required
   - Validate tag format matches `vX.Y` pattern
   - Validate tag name length (Git limit: 255 characters)
   - Validate tag message length (Git limit: ~64KB, use conservative 60KB)

5. **Tag Push Verification**: ❌ Not required
   - Simple push is sufficient (no retry logic needed)
   - No verification of tag propagation on remote

6. **Branch Name**: `main` (not `master`)

## Questions to Consider

1. Should Docker image testing include functional tests (running actual download script) or just smoke tests?
2. Should security scanning fail the workflow on high/critical vulnerabilities, or just warn?
3. Should SBOMs be attached to GitHub releases automatically?
4. What retention periods should be used for artifacts?
5. Should the workflows run in parallel or sequentially where dependencies exist?
6. How should dry-run mode work for the combined Release & Publish workflow?

## Success Criteria

The implementation plan should result in:

- ✅ 4 streamlined workflows (down from 5)
- ✅ All current functionality preserved
- ✅ Enhanced Docker image testing
- ✅ Comprehensive security scanning
- ✅ Complete SBOM generation
- ✅ Clear documentation
- ✅ Easy to maintain and extend
