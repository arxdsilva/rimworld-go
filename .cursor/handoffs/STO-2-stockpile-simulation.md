# STO-2: stockpile simulation

Checklist §6.3/6.4: Storage filters, priorities, linked groups, hopper and dispenser, graves and sarcophagi, stack limits, haul urgently.

## What changed
- `domain/building/zone/filter.go`, `special.go`, `defaults.go`: vanilla-shaped tree to depth 3 (Foods > Raw food > Plant matter, Meat, Animal products; Foods > Meals and Animal feed; Manufactured > Medicine, Drugs, Textiles, Components, Body parts; Raw resources; Items; Weapons; Apparel; Corpses; Chunks; Bones). Human corpses and bones are storable. `FilterRoot` is allow all / clear all. Ten special toggles including grave colonist and stranger filters.
- Defaults: a new stockpile takes no corpses, bones, chunks or rotten; a dump takes corpses and chunks; old zones keep their switches.
- `group.go`: Link/Unlink storage groups (an edit reaches every member); `zone.Map.SameStore`.
- New `game/store` package: storage buildings, groups, grave picking, urgent marks, dispenser, and the storage cell search moved out of game. Coverage: store 95.4 %, zone 97 %. Net -11 game lines.
- Wiki stack limits for every item kind; corpses and gear are one per stack.
- Bugs fixed: forced hauls, delivery leftovers and bill products honour special filters. `SetZoneOverlay` is UI-only (a no-op tick case on purpose).

## Wiki versus original
Wiki (Nutrient paste dispenser, Hopper): 0.3 nutrition makes 0.9 meal, up to 13 hoppers in draw order, 200 W, research Nutrient paste (needs Electricity), paste thought -4. Original tuning: hoppers hold raw food at Important and refill below half (no-flap); sarcophagus colonists only, beauty from stuff.

## UI
See STO-3.

## Tests
Zone and store unit tests; journeys StorageAllowAllAndSearch, HaulUrgently, NutrientPasteDispenser (in `journey_wave2_test.go`). Golden hashes re-pinned for stack limits and defaults.

## Gaps
The `FillHopper` job is unused (fills run as a plain haul). Wardens and doctors do not fetch paste for prisoners and patients.
