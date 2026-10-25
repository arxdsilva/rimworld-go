# Task Handoff

Task ID: SIM-060 (PLAY-017 §1, §5, §7, §8, §10, §11; plan `.cursor/PLAY017_PLAN.md`)
Status: complete

## Summary
**Sowing** (`food.go`):
- `assignZoneClear` cuts a wrong plant off a zone cell before sowing: wild, moss, bush, tree, or another crop.
- A ripe wrong crop is harvested instead of cut.
- Out of season and under buildings nothing is cleared (`sowableIgnoringPlant`).
- Forced `OrderSow` uses the species' sow work.

**Plants and light:**
- Plants grow for `max(sunTicks, lampTicks)`: the sun window on unroofed cells, or any hour with artificial light ≥ `plant.StrongGrowLight` (0.5).
- Grass follows the same rule, at 24/14 speed in the sun window, so its daily regrowth is unchanged.
- Campfires light only while fuelled.
- Ripening calls `markRipeHarvest`, and `wantsHarvest` needs `Designated`, so Cancel holds.
- `PlantView.GrowState`/`RipeIn`.

**Hauling** (`haul_claim.go`):
- `Stack.Unclaimed`/`HaulMarked`; only `haulToStockpile` skips unclaimed, unmarked stacks.
- Found items: map loot and chunks, vault and casket loot, raider and visitor gear, non-colony animal corpses, wild eggs.
- Pickup or storage claims a stack.
- Commands `DesignateHaul` and `SetHaulMark`.

**Apparel:**
- The best Body×Head outfit, scored with `comfortShift` for every option.
- `dressCheck` runs before work; `dressInterrupt` (60-tick bucket) drops interruptible work.
- 1 °C dead band and a per-slot cooldown; Equip is postponable.
- A forced "Wear" slot is kept (bits in `worn[NoSlot]`).
- Log lines `logPutOn`/`logTookOff` (not the message feed).
- Index `Game.clothes`.

**Tame animals:**
- They eat the nearest food the diet allows, stockpiles included, with a `FeedClaim` reservation.
- Zone and harvest-marked plants are skipped; pens and areas still bound the search.
- Tame predators hunt wild prey only, using the wild give-up and leash.

**Chase warnings** (`chase.go`):
- A danger message once per chase: the hungry fallback, revenge, a charge, or fighting back (via `hurtAs`).
- Re-warned only on a new target or after the chase has ended plus an hour (`ChaseWarnedAt`/`Target`).

## Notes
- Save v3: older saves mark loose chunks and carcasses outside stockpiles as found, and ripe zone crops get their mark.
- Goldens re-pinned. Field by field, only Animals and Messages moved (the chase warnings): 4 scale runs plus grown.
- Open:
  - food carried to a prisoner keeps its flags;
  - a stockpile painted over found items doesn't claim them;
  - pack members each warn once, so a pair of wolves pauses twice.
- Game package under -race: about 175 s on an idle M3.
