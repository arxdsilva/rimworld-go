# SIM-032 Colonist identity, skills in play and character creation

Part of NG-1 (`.cursor/NG1_NEW_GAME_PLAN.md` §3.5-§3.9, §4.6, §5.4; decisions D3, D9). Commits: `15b2ca1c` (domain), `07a536fe` (candidate pool), `f815d545` (in play, saves), `a9ca527c` (Bio tab and Work tab).

## What changed

### Domain packages (`internal/domain/pawn/...`)
- **`skill`:** 11 skills (Shooting, Melee, Construction, Mining, Cooking, Plants, Animals, Crafting, Medicine, Social, Intellectual), levels 0 to 20, `Neutral = 7`. Work speed `SpeedPermil = 1000 + 60 x (level - 7)`, so level 7 is exactly 1000 permille and level 20 outworks level 0 by 1780 to 580. Experience to the next level is `20000 + 6000 x level`. Passions: none learns at 35 %, minor 100 %, major 150 %. Decay: above level 10 a skill loses `500 x (level - 9)` experience a day and drops a level only when the deficit passes half the previous level's cost, landing at 3/4 of it (a no-flap band, `TestSkillXPAndDecayNoFlap`).
- **`stat`:** a stat sheet of 14 stats (work speed, move speed, hunger, rest, mood offset, break threshold, cold and heat comfort, shooting and melee accuracy, aim cooldown, pain tolerance, learn rate, recreation rate) built in a fixed layer order: base, body, age, conditions, traits, then the difficulty layer the game appends.
- **`trait`:** 19 original traits in conflict groups (pace, outlook, nerve, gait, appetite, standards, trigger): Tireless, Unhurried, Sunny, Brooding, Steady, Brittle, Fleet-footed, Plodding, Hearty eater, Light eater, Roughneck, Fussy, Deliberate aim, Quick draw, plus Green thumb, Bookish, Gentle (cannot do violence, +3 Social), Cold-hardy, Sturdy. Traits declare stat mods, thought scaling, work-speed or learning bonuses, and which backstories they cannot pair with; a trait is never paired with a backstory that makes it pointless.
- **`backstory`:** 59 original backstories (childhood and adulthood) with skill changes and incapabilities (violence, caring, dumb labour, intellectual, animals, cooking, plants, mining, crafting, construction), tagged by tech level (neolithic, industrial, spacer). Written in Go (`catalog.go`), not yet data files.
- **`look`:** sex, one of three builds (thin, average, heavy), hair colour and style (short, crop, bun, long, braid, shaved, curls, tail), beard (short, full, moustache), skin and clothes as palette data. `look.Legacy(id)` reproduces the renderer's historic look exactly (`TestLegacyLookMatchesRenderer`).
- **`pawngen`:** deterministic generator from (seed, slot, roll): names (first, nickname in 35 % of cases, last, unique nicknames), age 14 to 68 (mode 30; 8 % of colonists are teens of 14 to 17, who have only a childhood backstory), sex, build, backstories (10 % of non-clan colonists may draw a clan backstory), traits, skills, passions (30 % of picks major), conditions (12 % chance of one of old fracture, nearsighted, hand tremor, exhausted), temperament, beard on 45 % of men. Scenario `Rules` constrain age, forced traits and conditions, tech tag, no clothes. Body and age change stats: thin eats less and is faster but colder; heavy the reverse; under 18 learns faster but works slower; 50 and 65 slow work and walking.

### In play (`internal/application/game/skills.go`, `bio_build.go`, `bio_view.go`)
- Every colonist (and any raider SETUP gives a bio) holds a bio, a live skill set and a derived stat sheet; the sheet rebuilds on a birthday or a difficulty change.
- **Work:** a job's progress per tick follows the skill level, the WorkSpeed stat and trait or condition factors; fractional progress carries between ticks. **Learning:** each skilled tick grants experience scaled by passion, learn rate and trait multipliers, with a saturation of 4000 experience per skill per day; shooting or striking grants 120, a finished chat 40 Social. **Yields** (crops, mining, butchering) scale with the skill and the difficulty; **fumbles:** harvesting below Plants level 4 may fail; **accuracy:** shooting and melee accuracy follow skill and stats.
- **Traits and passions:** mood and break threshold offsets, thought scaling and extra thoughts (Fussy doubles the thoughts for sleeping outside, on the ground and eating raw food and likes a nice room; Roughneck cuts them to a quarter), hunger, rest, recreation and comfort rates, pain tolerance; working at a passion gives joy, and a major passion adds a "loves their work" thought.
- **Incapability:** work a colonist cannot do is never assigned or ordered: its priority is forced to 0, a `SetWorkPriority` on it is ignored with the toast "Incapable: <reason>", and a pacifist told to attack does not strike (`TestIncapablePriorityIgnoredAndOrderRefused`, `TestViolenceIncapableNeverStrikes`).
- **Neutral legacy pawns:** `SpawnNamedPawn` and old saves make a neutral pawn: all skills 7 (exactly 1000 permille), no traits, no incapabilities, adult, average build, `look.Legacy(id)`. Legacy timings are unchanged (`TestNeutralPawnTimingsUnchanged`, `TestLegacyLookUnchanged`).
- **Read model:** `Simulation.Bio(id)` feeds the Bio tab (backstories with pronouns, traits, skills with level, passion, experience and this week's gain, incapabilities with reasons, conditions).

### Character creation (`game/candidates.go`)
`CandidatePool` exists before any `Game`. Every candidate keeps the generation slot it was made in, so rerolls stay deterministic from (seed, slot, roll) however the roster is reordered. Operations: reroll one (keeping edited names), Randomize all, move, swap, rename (nickname required and unique, at most 16 characters, letters, spaces, apostrophe and hyphen), undo. The first `People` candidates are Selected; the pool holds `ChooseFrom` (default People + 5). `Team()` reports the best level per skill and warnings: nobody can fight, no doctor, no cook, no grower, no builder, no researcher (best level under 3 or all incapable), everyone very young or old.

### Saves
Bios and skills are saved (new state in save version 2; `TestImportBiosMigratesLegacy` migrates v1 colonists to neutral legacy bios). **Golden state hashes were re-pinned once for this** (`TestScaleGolden`): every other saved field is identical to the previous pins, checked field by field against `4d68b9aa`, so behaviour is unchanged; only the new saved state differs.

## Verification
- Recorded at commit time; the docs pass re-ran nothing. Key tests: `TestGenerateDeterministic`, `TestLegacyPawnIsNeutral`, `TestLegacyTemperamentMatchesGame`, `TestSkillXPAndDecayNoFlap`, `TestNeutralPawnTimingsUnchanged`, `TestSkillSpeedRatio`, `TestLearningSaturation`, `TestDecayNoFlapInPlay`, `TestAccuracyRisesWithSkill`, `TestTraitMoodAndBreakOffsets`, `TestBirthdayRebuildsStats`, `TestBioViewInGame`, plus `candidates_test.go`.

## Known limitations
- **Art follow-up:** the male and female builds read weakly at 1x zoom (the silhouettes differ by only a few pixels), and beards are nearly invisible when a colonist faces north.
- No Artistic skill (11 skills, not 12) and no craft quality.
- No relationships, no biological family, no per-pawn faction field.
- Backstories and traits are Go tables, not moddable data files.
- Stat layers stop at traits plus difficulty; equipment and environment layers are not part of the sheet.
- Age only changes through birthdays (60 days a year); there are no chronic conditions.
