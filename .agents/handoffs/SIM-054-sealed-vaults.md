# Task Handoff

Task ID: SIM-054 (PLAY-014, MAPGEN-3 §4)
Status: complete

## Summary
**mapgen** (`steps/features/vaults.go`, runs after start, before ruins):
- **Attempts:** round(W·H/250²), clamped to 1–3, none on sea ice. A vault is skipped when nothing fits.
- **Layout:** k = 1–4 casket bays (3 wide, 1×2 caskets), in 1–2 facing rows; a contents strip; a 1-cell wall ring. That makes footprints 6–15 × 9–12.
- **Placement:** the anchor is rock; ≥ 70% of the footprint plus a 2-cell ring is rock; ≥ 30 cells from the landing spot and ≥ 12 from other vaults; no caves, water, roads or features.
- **Carve:** smoothed stone floor; walls of local stone blocks (70%) or steel; ore under the vault cleared; roofs kept.
- **Contents:**
  - occupants: Grateful 30%, Hostile 25%, sleeper + crawler 15%, Beetles 10%, Empty 10%, Remains 10%;
  - loot (3–6 stacks): components, medicine, silver, gold, a rifle (35%), a rare prosthetic/eye/tablet (15%);
  - 2–6 defender spots.

**Game** (`vault.go`, `casket.go`), with states Generated → Warned → Breached → CasketsOpened → Cleared:
- **Defenders:** dormant hive insects, (points + 25)/50 clamped to 2–6, with brutes ≤ n/3. None at threat scale 0, on the Wanderer preset, or when hive swarms are disabled. Dormant animals skip thinking and needs; hurting one wakes it.
- **Warning:** when a colonist comes within 2 of a vault, the "Ominous walls" letter fires once.
- **Breach:** when fog reveals the interior, the defenders wake and a danger letter is posted. Breaching doesn't open the caskets.
- **Caskets:** opened by the Open casket order (it reuses the deconstruct job for 250 work; refused with "No colonist can reach it") or by any damage. Opening one opens every casket in its vault. The occupants:
  - a grateful sleeper joins;
  - a hostile one is a one-person raider with a rifle;
  - a crawler comes out hostile;
  - beetles come out hostile;
  - empty;
  - remains are bones with an old melee weapon.

  Opened shells salvage for 20 steel.
- **Ancient pieces:** not colony wealth, not Home anchors, not repaired.
- **Cleared:** once the caskets are open (or there were none) and no threat is left.

## Files
- `world/vault` (the occupant and state vocabulary).
- mapgen `Vault`/`Casket`.
- game `vault.go`, `casket.go`, plus hooks:
  - `setup_mapgen.go`, `fog.go`, `animals.go`, `combat.go`;
  - `defense.go`, `tick.go`, `placement.go`, `raids.go`, `save.go`, `game.go`.
- Renderer: `vault_draw.go` (casket sprites, the inspector button with `BuildingView.OpenReason`, `sfx.casket_open` cued when a casket turns open), the "Dormant" label, and the dev "Map fields" terrain layer marking vaults and bridges.
- `cmd/mapgen` draws vaults.

## Notes
- No mechanoids exist, so hive insects are the only defenders.
- `TestScenarioKitSpawned` subtracts vault loot, as it does ruin loot.
