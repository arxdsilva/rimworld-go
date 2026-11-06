# TEMP-3: pawn and animal exposure (agent X)

Wiki fidelity under ENG-003 (temperature). Gap rows: `.cursor/TEMPERATURE_GAP_ANALYSIS.md` §3.4, §3.5.

## What changed
- `domain/pawn/exposure`: base range 16-26 C; fixed-point severity (`Full` = 1.0); `HypoGain` max(0.00075, (d-10) x 0.0000645), `HeatGain` 0.0000645 x curve (1:1 to 25, 50→40, 100→60, 400→100), floor 0.000375; `Recovery` max(0.0015, min(0.015, 0.027 x sev)); `Step` runs 60-tick sub-steps. Hypothermia recovers at Min or warmer, heatstroke at Max or cooler, both hold in the margin. `Bands.Thought`: the 4-rung ladders, one rung per 10 C past the range (original). `FrostbiteChance` 500-4500 ppm per 60 ticks from 37 % to 100 % (original).
- `domain/pawn/body`: `CondHypothermia` (the contract's never-saved `CondFrostbite` slot), `CondHypothermicSlowdown`, `CondHeatstroke` with the wiki stages; `SetExposure`, `ExposureSev`. Frostbite is a wound kind (`body.Frostbite`, as vanilla's injury): `StrikeFrostbite` on ears, nose, hands, feet, paws and tails, never a whole part, no bleeding, 25 % infection untended (`infection.WoundChance`), 1 % scar, `FrostbiteParts`.
- `domain/animal`: `comfort.go` (wiki range per species; invented species take their role's numbers), `Slows` (hive insects), consciousness in `DownsNow`/`StandsUp` (30 %/35 %). The pure animal-body helpers moved from `game/animal_health.go` to `domain/animal/medical.go` (`NewBody`, `HealthScale`, `BleedPerDay`, `StepBlood`, `BleedDeathHours`): pays the game lines.
- `game/bodytemp` (new): `Ledger` (saved as `Exposure`, v8 `MigratePermille`: old 600 → 62 %), `Animals.Step` (hooked in `tickNeeds` on the animal's own decision tick), `Spot` (warm-spot search moved out of heat.go).
- Game: `heat.go` expose for colonists, prisoners and guests (stages, downing with cause, frostbite, death), `chillHost`; `rooms.go` ladder; views: `PawnView.Hypothermia/Heatstroke/Frostbite` (prisoners and guests too), `AnimalView.ComfortMin/Max/Hypothermia/Heatstroke`. Cold/Freezing/Hot/Sweltering retired. Warmth-seeking starts at 4 %, sleepers wake at 35 %.

## UI
Existing Hypothermia/Heatstroke status lines and alerts read the same permille fields; frostbite shows in the Health tab's wound list ("frostbite"). Animal temperature lines and stage names in the health tab are TEMP-5.

## Follow-up (same phase)
- Animal frostbite on the decision tick (paws, ears, nose, tail, feet), as hit points with an infection roll; hostile raiders take exposure, stages and frostbite (no frostbite messages for hostiles); slept-in-the-cold/heat judged before apparel (`rest_host.go` Band, base range with traits).
- `bodytemp.Animals.Step` returns an Outcome (Same/Changed/Died) and the game follows up: an animal handed to an interface method escaped to the heap on every tick (phaseAnimals' `animals.Get`, +260 allocs a quiet tick). Keep the animal off `AnimalHost`.
- Animal shelter seeking: not vanilla (the wiki leaves it to the player's barns); none added.
- `guest_rescue_test.go`: a debug edit was undone with `git checkout`; the file matched HEAD (fine). `TestDownedVisitorRescuedTendedAndLeaves` pins 20 C: the guest's open-air bed on −1 °C nights gave real wiki hypothermia.

## Open
None for TEMP-3. Animal temperature lines and stage names in the health tab are TEMP-5.
