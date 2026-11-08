# internal/adapters/inbound/cli/mapgen

## Purpose
The `mapgen` command's logic (MAPGEN-2): debug images, metrics, seed contact sheets, hills sweeps and exact-tile sites. `cmd/mapgen/main.go` only calls `Main`.

## Owns
`Main(args, out, errw) int`, `ParseArgs`/`Options` (flags and site parsing) and `Run` (single, grid, sweep and tile modes). The `render` sub-package turns fields into images.

## Depends on
The world domain (`biome`, `geology`, `site`, `mapgen`, `pipeline`, `planet`) and `faction`, read-only.

## Contracts
The flags are used by the Makefile and docs; sheets for a seed are byte-stable (`mapgen_test.go`).

## Tests
`mapgen_test.go`; `render/` has its own tests.
