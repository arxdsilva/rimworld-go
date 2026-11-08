# internal/adapters/inbound/cli/normalgen

## Purpose
The `normalgen` command's logic (ART §6.1): writes `<name>_n.png` normal maps next to world sprites. `cmd/normalgen/main.go` only calls `Main`.

## Owns
`Main(args, out, errw) int`, the flag set, and the per-sprite normal-map pipeline (autotile edge extension, flat-normal sprites).

## Depends on
`pkg/spritenormals` (bevel maths) and `pkg/flatyaml` (sprite metadata).

## Contracts
`Main` keeps the flags `-dirs` and `-force`; the Makefile art targets rely on them (`TestRunWritesNormals` and neighbours in `normalgen_test.go`).

## Tests
`normalgen_test.go`. Output is byte-stable: a `-force` run over `assets/generated/world` hashed the same before and after the move out of `cmd/`.
