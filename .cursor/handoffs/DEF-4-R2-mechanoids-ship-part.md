# DEF-4 (R2): mechanoids and the crashed ship part

Date: 2026-10-29. Wave E, agent R2. Backlog `.cursor/DEFENCE_GAP_ANALYSIS.md` (mechanoid and ship-part rows). Built on R1a's `game/raid` package; raids, drafted control and the storyteller wiring are in `DEF-4-R1-raids-drafted-control.md`; the shredding recipe is PRD-4's. Contract (CT2): roster rows, `weapon.MechBlades/PulseLance/NeedleCaster`, `construction.ShipPart`, incident `CrashedShipPart`.

## What changed
- **Species** (original designs, wiki role stats): Harrow (blade skirmisher, fast), Spindle (sniper), Bulwark (heavy gunner, XL, slow), Quill (skirmisher). Archetype `ArchMech`, behaviour `Mechanoid` (`domain/animal/animal.go`, `mech.go`).
- **Brain** (`game/raid/mech.go`, `Mechs.Think`): mechs are animal entities with their own brain, never the animal brain. No needs, never tamed, never down, never flee. They retarget every 60 ticks or when the target falls. Own weapon damage, cooldown and burst; plating counts as armor. Guard leash 30 tiles, release 36 (hysteresis, original).
- **EMP:** stuns through the burst hook (wiki: 15 ticks per damage point), then 2200 ticks of immunity (wiki adaptation). `AnimalView.Mech` and `Stunned`.
- **No wound body:** no bleeding, infection or stains.
- **Wreck:** a corpse stack (`item.CorpseAnimal`, `Stack.Animal` a mech), Dried so it never rots, predators skip it, no meat. `ItemView.Wreck`, kind "harrow wreck". The machining table's shred recipe (PRD-4) turns it into salvage through `animal.WreckYield` (wiki: Harrow, Spindle, Quill 15 steel; Bulwark 30 steel and 10 plasteel; PRD-4 divides by 10 like other costs).
- **Mech raid:** `Mechs.Raid(points, arrival, now)` from 300 points (wiki). Mechs land at the arrival spot; there is no pod entity.
- **Ship part** (`game/raid/shippart.go`): `construction.ShipPart` "ship part", 6x3, 1200 HP, Ancient (no blueprint, not wealth). It withers plants in a radius growing from 5 to 100 tiles over 20 days (`BlightRadius`). Its mech guards sleep until a colonist comes close, the hull is hit, or a guard is hurt or EMPed, then wake for good. Destroyed, it gives 60 steel, 10 plasteel, a component and an advanced component. Incident weight 2 (wiki), 400-point gate, one at a time. Saved through one field, `Mechs`. The storyteller's weighting of the face is R1's (ship part 40 vs people raids 74).

## Wiki vs original
Wiki: health, speed, armor, combat power, 96 % shooting and 62 % melee, EMP stun and adaptation, shredding yields, ship part HP, size and blight, weights, the 300 and 400 point gates. Original: guard release distance, ranges, cooldowns, names and looks.

## Creature checklist (skill rimworld-go-creatures)
- **Roster:** four rows appended in Go and mirrored in `animal_roster.py` (A4). `TestEverySpeciesHasArt` and `TestRealAnimalBodies` pass.
- **Art review sheets** (scratchpad): `/private/tmp/claude-501/-Users-arxdsilva-go-src-github-com-insurgencygames-rimworld-go/f49cfdd3-15de-432d-9933-c0c697f11506/scratchpad/review_mech_harrow.png`, `review_mech_spindle.png`, `review_mech_bulwark.png`, `review_mech_quill.png` (and `mech_static.png` beside them). Full sets per mech: walk, graze, flinch, rest, slash or shoot, wrecks v0-v2 on an oil pool, icons. Validator: 5957 sprites, 0 errors.
- **Sounds: BLOCKED, `[!]`.** Prompts are written (`assets/source/audio/animal_prompts.json`, four rows per mech) but **no voice was generated and no credits were spent**. The insect group stands in (`render/cue/animal_sounds.go` `VoiceGroups`: harrow, spindle, bulwark, quill). Why blocked on the user: the ElevenLabs key is missing, and an open-source MOSS-SoundEffect model is under evaluation as the alternative. Nothing is approved or in the ledger. Rule 5 of the creature skill says the placeholder is a temporary state; this is the record of it.
- **License line / notices:** no new voice entry, so `THIRD_PARTY_NOTICES.md` is unchanged.
- **Checklists:** vanilla 24.5 items ticked with the voices caveat; UI checklist has no mech tab item.

## Hysteresis
Guard leash 30 and release 36; guards wake once and stay awake (`TestShipPartWakeNoFlap`); EMP immunity 2200 ticks after a stun; retarget every 60 ticks.

## Tests
`game/raid/mech_test.go`, `shippart_test.go`, `domain/animal` (coverage 98.3 %); raid functions 80-100 %. Journey `TestJourneyMechEMP` (`journey_wave_e_test.go`: the colonist is armed before the mech lands; the end state is a wreck, not steel).

## Gaps
- Mechs cannot pass closed doors (raiders bash them; mechs do not yet).
- Mech groups do not use the pod landing hook.
- Mech voices: blocked on the user (above).
