# GEAR-1 (facts): the Gear tab's summary numbers, and armour slows the wearer

GEAR-001, GEAR-002 and GEAR-009 in `GEAR_GAP_ANALYSIS.md`. Wiki numbers by ENG-003 (GEAR-*). The facts are filled in the read model; drawing them is GEAR-2.

## What changed
- `domain/combat/armor/gear.go`:
  - `MoveOffset(kind)` gives each garment's move offset in hundredths of a cell per second. From the wiki Armor page: flak vest −0.12, flak pants −0.12, marine armour −0.25, plate armour −0.8. The wiki's cataphract armour (−0.5) has no kind here, so it isn't mapped. The flak jacket, helmets and clothes cost nothing.
  - `SpeedPermille` and `SpeedPct` convert those offsets against `BaseSpeed`, which is 460 (a human's 4.6 cells/s on the wiki). Both round half away from zero. Plate armour is −174 ‰ (−17 %), a flak vest −26 ‰ (−3 %).
  - `Overall(worn)` is our own formula, since the wiki publishes none. For each humanlike body part, the ratings of every worn layer covering it are added, capped at 200 % (`Set.Add`). Each part then counts by its `body.Def.Coverage` (the parts sum to 1000). Missing parts still count, so the figure describes the clothes, not the wounds. A flak vest alone gives 49 % Sharp, because the torso group takes 490 ‰ of hits.
  - `Region.Names()` lists what a garment covers, for example "torso, neck, shoulders, arms". A full helmet reads "head".
- New `application/game/gearfacts`, a pure package:
  - `Capacity` is 75 kg × Manipulation × body size (wiki: Carrying capacity).
  - `Grams` and `Moves`/`MovePermille` add up mass and move offsets. The penalties are summed in cells/s and converted once.
  - `Mods` is the stat sheet's gear step.
  - `Of` builds `gearview.Pawn`: mass, capacity, overall armour and move %.
- `craft/garment.go`: `WornView` now fills `Grams` (`item.MassGrams`, the existing wiki masses), `Cover` and `MovePct`.
- Root `body.go`: `gearFacts` adds the wielded weapon, the spare weapon (`Drafts.Spare`), the worn apparel and the pack. Manipulation comes from the body key, and body size from `hitBodies.Size`, which is 1000 for humanlikes. The pack's own 40 kg × Manipulation limit (`sense/pack.go`) is unchanged.
- Root `skills.go`: the speed stat.
  - The sheet's MoveSpeed now gets the gear offset as an Add step. It sits after traits and difficulty and before the body's Moving multiplier, so a lame wearer's capacity scales the armoured speed.
  - `pawnFx.gear` records the offset the sheet was built with.
  - `paceStep` is the only reader of MoveSpeed. It compares the outfit's current offset with that record and rebuilds the sheet only when they differ. This was needed because outfits are written from about ten places in files GEAR-1 doesn't own. The per-step cost is one map read plus an 8-slot switch, with no allocation. A pawn with no bio gets a sheet while it wears armour and loses it when the armour comes off.
- Root package: +10 lines net, no new `Game` methods, fields or files.

## Hash
The state hash changes for any colony or raid where someone wears a flak vest, flak pants, marine armour or plate armour, because those pawns walk slower. Raider kits (`raids.go`, CBT-3) wear flak, so raid fixtures move. The lead re-pins `TestGrownGolden`.

## Tests
- `armor/gear_test.go`: the wiki penalties and their conversion, the overall-armour weighting and cap, and region names.
- `gearfacts/gearfacts_test.go`: capacity, masses, penalties that add, and `Of`.
- `craft/garment_gear_test.go`: the facts on each `WornView` row.
- `game/gear_speed_test.go`, which covers four cases:
  - The stat sheet with each armour set and with none.
  - The sheet is not rebuilt when nothing changed.
  - A pawn with no bio.
  - A real 32-cell walk, bare against plate (≈ 1/0.826 of the time).

## Gaps
- No UI. GEAR-2 draws the tab.
- Cataphract armour isn't mapped: we have no such kind.
- `Overall` ignores missing body parts.
