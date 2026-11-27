# internal/domain/pawn/stat/statmod

## Purpose
The declarative `stat_mod` definition class plugins add stat modifiers with. Pure and deterministic: integers only, defs in book order.

## Glossary
- stat mod: one `Def` that adds `Add` and scales by `Mul` (permille) one stat while its condition holds.
- condition (`Cond`): a discrete fact only: a trait key, a bio condition key (`pawngen.Condition.Key`, for example core.old_fracture) or being injured. No numeric threshold, so nothing can flap.

## Owns
`Def`, `Cond`, `Facts`, `Book` and `Book.Layer`, the last layer of a colonist's stat sheet (applied in `rebuildStats`).

## Contracts
- Bounds, checked by the codec `application/modding/class_statmod.go`: `add` within plus or minus 100000, `mul` 0 to 10000, an unknown stat or condition refuses the def.
- Defs are rebuilt from plugins on load, so modifiers survive save and load (`TestExtStatModsSurviveLoad`).
- A `Def` with `injured` or a bio condition needs the plugin capability `health`, others `stats`.

## Must not depend on
`application/*`, the simulation, adapters; only `domain/pawn/stat`.

## Tests
`statmod_test.go`; codec `class_statmod_test.go`; end to end `TestExtE2E/AC10`.
