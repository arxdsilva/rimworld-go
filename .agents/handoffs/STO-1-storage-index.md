# STO-1: storage index (behaviour-neutral)

Checklist §6.3 "Storage capacity/index": find valid free storage cells without map-wide scans.

## What changed
- `game/haul/ground.go` `haul.Ground`: a per-cell list of loose stacks in `itemIDs` order, verified against the store on every lookup.
  - `looseIndexed`/`looseScan` pair, which panics on mismatch under `indexSelfCheck` (on in tests).
  - `First`, `Others`, `Room` and `Shelf` feed `itemAt`, `cellFull`, `haulRoom`, `mergeOrSetFrom`, `spillTile` and the storage search.
  - It rebuilds when the item generation moves. `putStack` replaces `g.stacks.Set` at all 48 sites and bumps on spawn, drop, pickup or a loose stack moving, but not on carried steps or quantity-only writes. `deleteItem` always bumps.
- `domain/building/zone/storage.go` `zone.Stores`: storage zones by priority, keyed on `zone.Map.Ver()`. `storeSearch` returns the same (distance, y, x) minimum, skipping far zones and rows.
- `roomAbove` stops at the first valid cell; `craftOutput.Store` lost its duplicate search.
- Deleted: `looseOnStockpiles`, `releaseLoose`, `shelfLoads`, `otherStackAt`.
- Still scanning (not hot): `bridge_remove.go` `looseAt`, `wildlife.go` carcass merge, `defense.go` `clearItemsFrom`, `haulCands` (needs every item).

## Proof
- Benchmark state hashes identical to HEAD: grown/250 0x429354592b6a8327, grown/500 0xc2b7ae25d546ae89, raid/250 0x92136f1abb1b0ce7, raid/500 0xd7ae890ebd51dc0.
- TestScaleGolden's six hashes are identical to HEAD.

Frame p99 and PhaseAI p99, HEAD → now (ms):

| Fixture | Frame p99 | PhaseAI p99 |
|---|---|---|
| grown/250 | 1.56 → 1.50 | 0.398 → 0.384 |
| grown/500 | 1.70 → 1.42 | 0.489 → 0.401 |
| raid/250 | 5.07 (noisy run) → 1.78 | 0.737 → 0.410 |
| raid/500 | 2.21 → 2.04 | 0.485 → 0.432 |

- Package game: −57 lines and −5 Game methods; budget lowered to 54,371 lines and 1,930 methods.

## Found on the way: stale TestScaleGolden
TestScaleGolden skips under -race, and the determinism gate's `-run` filter (`Determin|SameSeed|Hash`) didn't match it. Its pins went stale over the intended behaviour changes since the SIM-06x programme, each of which re-pinned TestGrownGolden with its reason.
- It is now re-pinned to commit 121821cac's values, which STO-1 and the contract keep.
- The gate in `PARALLEL_WORK.md` §5 now runs `Determin|SameSeed|Hash|Golden` without -race.

## Contract (CT3, same commit; inert)
- **Items:** NutrientPasteMeal, SmokepopPack, ShieldBelt, RocketLauncher, TripleRocketLauncher, SmokeLauncher, ChargeRifle, ChargeLance, Ikwa. They are filed in the zone filter tree, not equippable until CBT-3, and hidden from scenario choices.
- **Construction:** Hopper, NutrientPasteDispenser, Sarcophagus, all hidden from the Architect.
- **Commands:** HaulUrgently, DropWeapon, LinkStorage, UnlinkStorage, SetStorageSource, SetZoneOverlay, routed to a no-op.
- **Damage and jobs:** a `Stun` damage type; jobs FillHopper and DispensePaste.
- **View fields:**
  - PawnView: WeaponQuality, WeaponStuff, Kills, DamageDealt;
  - BuildingView and ZoneView: Group, Stored, Capacity;
  - ItemView: Urgent.
- Inertness test: `game/sto_cbt_contract_test.go`.
