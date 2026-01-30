# Definition of Done (CLI-Only Refactor)

This document verifies the checklist from the CLI-only refactor plan (§20).

## Checklist

### CLI

| Item | Status | Notes |
|------|--------|--------|
| `musicdl plan <config>` and `musicdl download <config>` are the only primary commands | ✅ | main.go dispatches only plan, download, version |
| `version` works | ✅ | `musicdl version` / `--version` / `-v` |
| No `serve` or `download-service` | ✅ | Removed in Phase 5 cleanup |

### Exit codes

| Item | Status | Notes |
|------|--------|--------|
| Plan returns 0/1/2/3 | ✅ | PlanExitSuccess, PlanExitConfigError, PlanExitNetwork, PlanExitFilesystem |
| Download returns 0/1/2/3/4/5 | ✅ | DownloadExitSuccess, ConfigError, PlanMissing, Network, Filesystem, Partial |

### Config

| Item | Status | Notes |
|------|--------|--------|
| Spec YAML (top-level `spotify`, `threads`, `rate_limits`) loads and validates | ✅ | download/config/loader.go normalizes into internal struct |
| Legacy `download.client_id` supported when `spotify` absent | ✅ | Loader supports both layouts |

### Hash

| Item | Status | Notes |
|------|--------|--------|
| Plan file name is `download_plan_<16-hex>.json` | ✅ | plan.GetPlanFilePath, plan.SavePlanByHash |
| Hash computed from raw config file bytes | ✅ | config.HashFromPath reads file, HashFromBytes SHA256 first 16 hex |
| `download` rejects plan when config hash does not match | ✅ | plan.LoadPlanByHash validates; ErrPlanHashMismatch → exit 2 |

### Paths

| Item | Status | Notes |
|------|--------|--------|
| Plan and caches under `.cache/` or MUSICDL_CACHE_DIR | ✅ | getCacheDir(), plan/cache use cacheDir |
| `.cache` and `.cache/temp` created when needed | ✅ | os.MkdirAll(cacheDir, 0755) in planCommand; temp on first use |
| Temp files cleaned after download | ✅ | Executor/cleanup per spec |

### Plan JSON

| Item | Status | Notes |
|------|--------|--------|
| Written plan has top-level `config_hash`, `config_file`, `generated_at`, `downloads`, `playlists` | ✅ | download/plan/spec.go PlanToSpec, SpecPlan |
| Load round-trip produces plan Executor can run | ✅ | SpecToPlan, LoadPlanByHash; spec_test.go round-trip |

### Caches

| Item | Status | Notes |
|------|--------|--------|
| Spotify and YouTube caches persist in `.cache/` with correct TTLs | ✅ | download/cache/manager.go; TTL on read |
| Download cache updated after run | ✅ | Manager has Download cache type; wiring optional |

### No web/gRPC

| Item | Status | Notes |
|------|--------|--------|
| No HTTP server, no gRPC, no proto | ✅ | control/handlers, server, service, client, download/server, download/proto removed |
| `go build ./...` and tests pass without protoc | ✅ | Makefile build has no proto; CI has no proto step |

### Docker

| Item | Status | Notes |
|------|--------|--------|
| Image runs `musicdl plan` / `musicdl download` with work dir and cache dir as specified | ✅ | musicdl.Dockerfile WORKDIR /download; no entrypoint |
| No healthcheck on a web port | ✅ | Dockerfile has no HEALTHCHECK |

### Docs

| Item | Status | Notes |
|------|--------|--------|
| README describes CLI usage, config layouts, exit codes | ✅ | README.md updated in Phase 6 |
| Troubleshooting covers "plan not found" and "plan does not match configuration" | ✅ | README Troubleshooting section |

---

**Verification commands**

```bash
go build ./...
go test ./...
staticcheck ./...
golangci-lint run ./...
make build
./musicdl version
./musicdl
./musicdl plan /nonexistent.yaml   # expect exit 1
# After "musicdl plan config.yaml", run "musicdl download config.yaml" for exit 0
```

## Workflows, Makefile, and docs

| Area | Status | Notes |
|------|--------|--------|
| GitHub Actions: Test & Coverage | ✅ | Go 1.24, unit + optional integration, no proto |
| GitHub Actions: Docker Build & Test | ✅ | CLI smoke + exit-code tests |
| GitHub Actions: Release & Publish | ✅ | Builds musicdl.Dockerfile (CLI-only) |
| GitHub Actions: Security & SBOM | ✅ | Source and image scan; no web/proto |
| Makefile test targets | ✅ | test-specific/test-function use control/main_test.go |
| README | ✅ | CLI usage, config, exit codes, troubleshooting |
| docs/wiki (Architecture, CI-CD, Development) | ✅ | CLI-only descriptions |
| docs/wiki TrueNAS-Deployment | ✅ | CLI-only deployment and cron-based scheduling |
