# internal/adapters/inbound/cli/atlasgen

## Purpose
The `atlasgen` command's logic (NG-1 7.1, ART §24 and §26): packs a sprite family into atlas pages and validates them with `-check`. `cmd/atlasgen/main.go` only calls `Main`.

## Owns
`Main(args, out, errw) int`, the flag set, packing (collect, measure, layout, write) and `checkAtlas`.

## Depends on
`pkg/flatyaml` for sprite metadata.

## Contracts
Flags `-family`, `-max`, `-root`, `-check` are used by the Makefile art targets. Output is deterministic (`atlasgen_test.go`); a pawn pack hashed the same before and after the move out of `cmd/`.

## Tests
`atlasgen_test.go`.
