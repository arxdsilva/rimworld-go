# internal/platform/buildinfo

## Purpose
The build's version (`Version`, "dev" by default), moved out of `cmd/game`'s `main.version` (2026-11, GAR). Release builds set it with `-ldflags "-X rimworld-go/internal/platform/buildinfo.Version=<tag>"` (`.github/workflows/release.yml`).

## Contracts
"dev" means a developer build: no update check and no usage data (`internal/compose/game`, `TestUpdateCheckIsOffForDevBuildsAndDevRuns`, `TestUsageEndpoint`).

## Invariants
Only the linker sets `Version`; no code assigns it (the SOLID `global` rule).
