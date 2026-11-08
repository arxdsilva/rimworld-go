# internal/adapters/inbound/cli/game

## Purpose
`cmd/game`'s command line, moved out of `cmd/game` (2026-11, GAR): the flags and their usage text, `Parse` and `ExitCode`. The composition root (`internal/compose/game`) takes the `Options`.

## Owns
`Options` (a DTO), `Parse(os.Args)`, `ExitCode(err)`, `BiomeKeys()`.

## Depends on
`internal/domain/world/biome` for the `-biome` keys.

## Must not depend on
`internal/compose` (only `cmd/` imports it, `TestOnlyCmdImportsCompose`) or the Ebitengine adapter.

## Contracts
- `bin/game -h` is byte-equal to the pre-move help (GAR proof); `-h` exits 0 and a bad flag 2, as `flag.ExitOnError` does (`TestBadFlagsAndHelpExitAsTheFlagPackageDoes`).
- `HiDPI` and `VSync` are nil unless the flag was given; `SeedSet` says whether `-seed` was (`TestOnlyGivenGraphicsFlagsOverride`).
- Every scenario the `-scenario` help names is one `internal/application/devscene` builds (`TestHelpedScenariosExist`).

## Notes
A new flag is a field on `Options` and one `fs.*Var` line in `parse`; the composition root reads it.
