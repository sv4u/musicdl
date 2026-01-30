# CI/CD Workflows

## Overview

This project uses GitHub Actions for automated testing, code coverage, Docker image building, and publishing. All workflows run on pull requests and pushes to the `main` branch. The project is Go-based (CLI-only); there is no Protocol Buffers step or web server.

## Workflows

### 1. Test & Coverage

**Triggers:** Pull requests (opened, synchronize, reopened); pushes to `main`.

**What it does:**

1. Sets up Go 1.24
2. Caches Go modules
3. Optionally installs yt-dlp for integration tests
4. Runs unit tests (excludes integration/e2e via build tags): `go test ./... -tags="!integration !e2e"`
5. Runs race detector on unit tests
6. Runs integration tests if yt-dlp is available (optional, continue-on-error)
7. Generates coverage (unit + optional integration merge)
8. Produces HTML and XML coverage reports and uploads as artifacts

**No proto step:** Build and tests do not require Protocol Buffers or protoc.

### 2. Docker Build & Test

**Triggers:** Pull requests; pushes to `main`.

**What it does:**

1. Builds Docker image from `musicdl.Dockerfile` (CLI-only; no proto in image).
2. **Smoke tests:**
   - Directory `/download` and binary `/usr/local/bin/musicdl` exist and are executable
   - `musicdl` (no args) prints usage with "plan" and "download"
   - `musicdl version` returns a valid version string
   - ffmpeg available in image
3. **Functional tests:**
   - `musicdl plan` with missing config file exits 1
   - `musicdl download` with valid config but no plan file exits 2

**No web server or healthcheck:** Container has no default entrypoint; tests use `musicdl` CLI only.

### 3. Security & SBOM

**Triggers:** Pull requests; pushes to `main`; release events (published).

**What it does:**

- Trivy scan on source; upload SARIF to GitHub Security
- SBOM generation (Syft, Trivy) for source in CycloneDX and SPDX
- On release: scan published Docker image; generate image SBOMs; Grype scan

### 4. Release & Publish

**Triggers:** Manual dispatch only.

**What it does:**

1. Validates branch (main), clean working directory, up-to-date with remote
2. Calculates next version (major/minor/hotfix)
3. Creates and pushes git tag
4. Generates changelog (git-cliff), updates CHANGELOG.md, creates GitHub release
5. Builds Docker image from `musicdl.Dockerfile` (no protoc)
6. Publishes image to GHCR (version tag and latest)

## Running Tests Locally

- **Unit tests:** `make test` or `go test ./... -tags="!integration !e2e" -v`
- **Coverage:** `make test-coverage` or `make test-cov-html`
- **Race:** `make test-race`
- **Integration:** `make test-integration` (requires yt-dlp and optionally Spotify credentials)
- **Specific file/function:** `make test-specific FILE=./control/main_test.go`, `make test-function FILE=./control/main_test.go FUNC=TestPrintUsage`

## Dependency Management

Dependabot is configured (`.github/dependabot.yml`) for security and version updates (Go modules, Docker, GitHub Actions).
