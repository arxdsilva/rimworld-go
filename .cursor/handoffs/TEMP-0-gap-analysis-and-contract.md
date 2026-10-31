# TEMP-0: temperature gap analysis and contract

User request (2026-10-31): temperature management "just like rimworld has", with a deep gap analysis first. ENG-003 is extended to temperature management (AGENTS.md).

## What changed
- `.cursor/TEMPERATURE_GAP_ANALYSIS.md`: the gap analysis (outdoor and events, rooms, heat sources, pawn exposure, animals, clothing layers, temperature-driven effects, UI), each row with wiki behaviour, evidence, gap, phase (TEMP-0..5) and state, plus a decision record (ENG-003, layered clothing, normal storyteller events, water freezing out of scope).
- `AGENTS.md`: the ENG-003 exception now names temperature management (TEMP-*) and 2026-10-31.
- Checklist corrections: vanilla "Outdoor temperature" description, vanilla "Heat sources/sinks" from `[x]` to `[-]` (no passive cooler), MAIN_UI time-controls row ("no temperature/weather" was stale).
- Craft temperature-units bug (benches and barrel read tenths as whole degrees) is a TEMP-0 row; the fix and regression test are the lead's.

## Craft temperature fix
`craftBenches.TemperatureAt` hands craft tenths of a degree (`craft/deps.go` now says so). The bench (`craft/station.go` tempSpeed) and the fermenting barrel (`craft/ferment.go`) compared it against whole degrees, so every bench in a warm room ran at 70 % and barrels fermented only at 0.7–3.2 C. Both now compare tenths. The craft fakes moved to tenths too, which is why their tests missed it. Regression test: `game/craft_temp_test.go` TestBenchSpeedFollowsRoomTemperature fails on the old code (504 at both 21 C and 5 C). `temperature.Temp.Whole()` was added (String uses it).

## Contract names (CT5, inert: both goldens unchanged)
- **construction:** `PassiveCooler` (wood), `Vent` (steel, rotatable, not minifiable); `Kind.WallMounted()` (Cooler, Vent); `construction.WallFacing` (moved from coolerFacing). Room boundaries and wall-gap placement use WallMounted. No Architect rows yet.
- **command:** `ToggleVent` (Target, Enabled), a no-op in tick.go.
- **item:** `TShirt`, `ButtonDownShirt`, `Pants`, `Jacket`, `CowboyHat`, `BowlerHat`, `Tribalwear`; `IsContractApparel()` (not apparel, not craftable, not spawned; exempted in filter_test and food_test until TEMP-4).
- **layer slots:** `SkinTorso`, `SkinLegs`, `Middle`, `NumSlots` = 8, `Outer` = Body; `SlotSet`, `Kind.Occupies()`. In TEMP-4, `craft.Slots` grows from 5 to 8, and ImportOutfits moves a saved flak vest from Body to Middle. Flak pants stay in Legs.
- **body:** `CondFrostbite`, `CondHypothermicSlowdown` (cond_frostbite.go, wiki stages); `FrostbiteMinHypothermia` 370, `FrostbiteMaxTemp` 0.
- **mood:** `Chilly` −4, `Shivering` −8, `NumbingCold` −12, `AbsolutelyFreezing` −16; `Sweaty` −4, `Overheated` −8, `Baking` −12, `BlisteringHot` −16 (one group per ladder); `Naked` −6.
- **incident:** `HeatWave`, `ColdSnap`, `VolcanicWinter` (weight 0).
- **views:** PawnView `Frostbite`; BuildingView `Open`, `HeatRate` (tenths of heat/s); RoomView `ExposedWalls`, `DoubleWalls`, `HeatRate`, `RoofKind`; AnimalView `ComfortMin`, `ComfortMax`, `Hypothermia`, `Heatstroke`; WorldView `ClimateEvent`, `ClimateEventEnds`, `TempVer`.
- **overlay source:** `RoomGrid` + `Rooms[id].Temp`, with `Game.TemperatureAt(x, y)` for cells outside rooms, cached on `TempVer` (filled in TEMP-1).

## Proof
Full -race suite green; determinism gate (-count=2) green; arch green; game budget lowered to 54010 lines.
