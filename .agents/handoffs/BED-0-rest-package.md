# BED-0: the rest package, stats and room formulas

Date: 2026-10-29. Programme BED/PRD/DEF (`.cursor/BEDS_GAP_ANALYSIS.md`). Wave A, committed with PRD-0 and DEF-0.

## What changed
- **`internal/application/game/rest`** (X2, opus) now holds sleeping: when and where (`Sleep`), bed claims and partner beds (`Beds`), and waking room thoughts (`Waking`). Its seven roles are value-only, adapted in `game/rest_host.go`. The `restedFree` field moved in, so its game field nets zero. Bed rest stays in `care`; lovin' stays in game.
- New no-flap test: `TestWakeDoesNotFlap`.
- **`internal/domain/pawn/stat`** (D2): Comfort, RestEffectiveness and Beauty stats.
- **`internal/domain/world/room/stats.go`** (D2):
  - the vanilla space, beauty, wealth and cleanliness stats;
  - the wiki impressiveness formula (65 × mean + 35 × min of four log-curved terms, integer-only so every platform agrees), with stage labels;
  - role rules for hospital, prison cell/barracks, rec room and kitchen.
- The existing `RoleOf`/`Impressiveness` are unchanged until BED-2 switches over.
- No behaviour change: the hashes are identical. Coverage: rest 100 %, room 96.5 %.
