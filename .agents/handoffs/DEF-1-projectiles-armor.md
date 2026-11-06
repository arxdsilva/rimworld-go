# DEF-1: projectiles, burst, armor, combat overlays

Date: 2026-10-29. Wave B (committed with PRD-1 and BED-1). Backlog `.cursor/DEFENCE_GAP_ANALYSIS.md`.

## What changed
- **Shots** (C1, opus; `combat/shot.go`, `strike.go`, domain `projectile`, `attack`, `armor`):
  - each ranged shot is decided at the muzzle: aim roll (band × per-tile shooter accuracy^distance × size × light × weather), a wild miss within the miss radius, then a cover roll that can damage the cover;
  - in flight, walls, doors and rock stop it, and creatures on the path may intercept it (40 % × body size). Allies within 5 tiles of the shooter are safe.
  - **Original tuning:** a shot hits its target only if the target is within 1 tile of the aim point; otherwise it is a 50 % stray hit on whatever is in that cell. The wiki publishes no such rule.
  - Burst shots left are kept in `job.Count`.
- **Weapons and armor:**
  - weapons: knife, club, spear and the rifle (now the wiki's bolt-action) have their wiki stats; fists, animal attacks and the sentry gun keep today's values;
  - apparel armor (`armor/apparel.go`): leather parka, wool parka, duster, tuque;
  - hits roll a body region, then each layer outer to inner; a halved sharp hit becomes blunt.
- **Read models:**
  - `TraceView.Stage` (Whole, Fired, Landed);
  - `WorldView.Shots`;
  - `Game.HitChance(shooter, target) (attack.Report, bool)`, paid for by turning `hasRanged` into a function.
  - Save: `Shots` in flight.
- **UI** (U2 draws, U3 wiring, sonnet):
  - the selected drafted colonist's range ring;
  - a line-of-sight line and hit-chance card over a hovered hostile;
  - a cover overlay on Shift+O (input action `overlay.cover`, journey `TestJourneyCombatOverlay`);
  - projectile streaks from `WorldView.Shots`.
  - Lead fixes: the overlay feed became plain functions with reused scratch buffers (App/Renderer method budgets, no per-frame allocation), and `HitChance` folded into `SimWorld` (the Simulation union stays at 8).
- **Sounds** (S1, sonnet): nine procedural recipes (`cmd/audiogen/combat_recipes.go`) mapped by stage in `render/cue/combat_sounds.go`. Fired plays the shot; Landed plays a miss, a wall hit, a flesh hit or an armor deflect. The audio budget is raised from 20 MB to 30 MB by user decision (`cmd/audiogen`, ART doc §25).
- **Gap left in DEF-1:** the hit chance uses fixed light (no published penalty) and clear-weather factors; it doesn't read the map's light or the current weather (fog 50 %) yet. That comes with DEF-3/DEF-4.
- **Tests:** `combat_model_test.go`; combat coverage 96.9 %; journey `TestJourneyCombatHitChanceTooltip`, rewritten by the lead against `HitChance` (permille) and `WorldView.Shots`. Burst fire is asserted in DEF-2, with the burst weapons.

## Wave B numbers and gates
- Game package: 54,621 → 54,565 lines, 181 files, `Game` methods 1,950 → 1,945, fields 147. Budget lines lowered.
- Draw benchmark (`DrawArt500/z1`, back to back against HEAD): 0.95 vs 1.08 ms, 419 vs 438 draw calls, 3,636 vs 4,041 allocs. The fixture scene is built by the simulation, so the wave's sleep and combat changes alter what is drawn; this is not a renderer change.
- `make art-check` and `audiogen -check` pass. Full `go test -race ./...` passes.
- Hashes: grown 250 `0xf6c61f40b377d985`, grown 500 `0x1a01790f2f095343` and raid 500 `0x7185b71c1b1e1272` are unchanged; raid 250 → `0x7405cf38329631f4` (combat). `TestGrownGolden` was re-pinned to `0x1f3b4995d6ddfe70`, its reason written beside the pin, stable over `-count=2`.
- Lead fixes from the gate:
  - The Forbid toggle shows only on sleeping places, the buildings whose users honour it, which restores the command-order tests.
  - The Bills tab width is clamped to the 560 px expand maximum.
  - The Architect rotate journey places beds clear of the taller furniture menu.
