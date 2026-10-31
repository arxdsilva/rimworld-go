# Temperature management gap analysis (TEMP-0..TEMP-5)

Scope: outdoor climate and events, room thermodynamics, heat sources and sinks, pawn and animal comfort and exposure, clothing, temperature-driven effects, and the UI, vanilla core only (no DLC). Load only for TEMP-* tasks. Phase handoffs go in `.cursor/handoffs/TEMP-N-*.md`. The pieces exist (biome climates, per-room temperature, heater and cooler, campfire, hypothermia and heatstroke, warmth-seeking, auto-dressing), but the model is coarse and most numbers are original. Sister documents: `FOOD_GAP_ANALYSIS.md`, `MEDICAL_GAP_ANALYSIS.md`, `BEDS_GAP_ANALYSIS.md`, `COMBAT_GAP_ANALYSIS.md`.

## Status (2026-10-31)

TEMP-0 is in progress (this analysis, the ENG-003 extension, the inert contract and the craft temperature-units fix). TEMP-1 to TEMP-5 are missing. Out of scope by decision (section 1): water freezing, DLC climate content.

| Phase | Delivers | State |
|---|---|---|
| TEMP-0 | This analysis; ENG-003 extended; inert contract (constructions, command, item kinds, layer slots, conditions, moods, incidents, view fields); craft temperature-units fix with a regression test; stale checklist rows corrected | in progress |
| TEMP-1 | Room thermodynamics (new `game/thermo`): 120-tick equalisation, wall and roof exposure, doors and vents, heater, cooler, passive cooler, campfire and torch caps, production and body heat | missing |
| TEMP-2 | Outdoor and events: biome extremes checked, heat wave, cold snap and volcanic winter as storyteller incidents, Extreme weather weighting, per-plant cold death | missing |
| TEMP-3 | Exposure: 16-26 C base range, wiki gain and recovery formulas, stage effects, frostbite, 4-stage moods, prisoners and guests, animal comfort ranges and death, insect slowdown | missing |
| TEMP-4 | Layered clothing: layer slots, seven new garments, wiki apparel factors, tailoring recipes, naked thought, layered auto-dressing, worn-layer art | missing |
| TEMP-5 | UI: temperature overlay, vent and passive cooler panes, heat lines, health stages and frostbite, animal temperature, alerts, climate event chip, gear layers | missing |

## 1. Decision record

| Item | Decision |
|---|---|
| Goal | Make temperature management as similar to RimWorld as possible, carried through until complete, ranked by how wrongly the colony stays warm or cool. |
| Numbers | ENG-003 in `AGENTS.md` is extended to temperature management (2026-10-31) at the user's request ("just like rimworld has"): published wiki numbers are tuning data; names and prose stay original. Each constant gets `// wiki: <page>`, or `// original tuning` where the wiki gives none. |
| Original tuning (wiki silent) | Daily swing and seasonal formula, door and vent exchange rates, frostbite chance and body parts, mood trigger offsets, fire heat, event weights and storyteller MTB, the vent exchange chain penalty. Each is recorded in its phase handoff. |
| Layered clothing | The user chose layered core clothing: skin, middle, outer and head layers with wiki apparel factors, replacing the four single slots (Body, Head, Legs, Belt) for insulation. Outfit saves stay a slice. |
| Normal events | Heat wave, cold snap and volcanic winter become normal storyteller incidents; the wizard's Extreme weather option raises their weight instead of gating them. |
| Out of scope | Water freezing (not core vanilla either) and DLC climate content. Not a gap. |
| Wiki gaps | The daily swing and seasonal formula, door and vent rates, fire heat, event weights, frostbite chance and parts, mood trigger temperatures are unpublished. Heater idle draw differs between pages (17 or 18 W) and campfire max differs (28 or 30 C); the Heater and Campfire pages win. The "-9555 C difference" equalisation table is marked [Check] in the notes. Where a number is missing the row says so and the phase picks an original tuning. |
| Clean room | No wiki text, code, art or sound is copied. New buildings and garments get original art through `rimworld-go-art-assets`. No new sound-making events exist, so no new sounds (vent and door cues exist). |
| Budget | `internal/application/game` is at its SOLID ceiling (54,017 lines, 178 files, `Game` 1912 methods and 147 fields; `body.Body` 99 methods). New behaviour goes in sub-packages (new `game/thermo`, existing care, rest, craft) behind consumer-owned roles; hypothermia and frostbite conditions are free functions. Never raise a `solid_budget.txt` line. |
| Determinism and hysteresis | Rolls use `rng_streams.go`, no global RNG or wall clock, no per-tick global scans. Room data is cached per room and invalidated on structure, roof and door changes. Every threshold behaviour (thermostat, warmth-seeking, dress plan, campfire bank, vent) starts and stops at different thresholds or has a cooldown, with a no-flap test. |
| Saves | New saved state keeps gob names, gets Export/Import and a round-trip test, and changed units get a migration (`save.go` `migrateSave`). Goldens are re-pinned only in behaviour phases, and the handoff says so. |
| Player visibility | Each phase ships read-model fields and the HUD/UI representation (TEMP-5 collects the screens) or states in the handoff why none fits. |

## 2. Reading the tables

State legend: `done`, `partial`, `missing`. `Today` evidence is `file:line` from the 2026-10-31 inventory, spot-checked by grep (section 4); line numbers drift, so find the function. Paths are under `internal/`; `G` is `application/game/`, `T` is `domain/world/temperature/temperature.go`, `item.go` is `domain/economy/item/item.go`. Wiki pages are rimworldwiki.com pages; temperatures are in C and heat in "heat/s" as the wiki gives them. Local temperatures are tenths of a degree (`temperature.C(deg)`).

## 3. Gap tables

### 3.0 Contract and regressions (TEMP-0)

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Workbenches are comfortable at 10-35 C and run at 70 % speed outside; the fermenting barrel works at -1 to 32 C (wiki: Temperature, Fermenting barrel). | `craftBenches.TemperatureAt` (`G craft_host.go:120`) returns `temperatureAt` in tenths, but `craft/deps.go:45` documents whole degrees. `craft/station.go:35-43` (good 10/35) and `craft/ferment.go:27-30` (FermentMinC 7, MaxC 32) then read the wrong scale: every bench in a 21 C room runs at the 70 % speed (210 > 35) and barrels only work at 0.7-3.2 C. Unit tests use whole degrees, so they miss it. | Convert to whole degrees at the host; barrel range -1 to 32 C; regression test through the real host. | TEMP-0 | partial (host conversion edit in the working tree; lead owns the test and golden re-pin) |
| Contract: PassiveCooler and Vent constructions, ToggleVent command, seven garment kinds, layer slots, Frostbite and hypothermic-slowdown conditions, four-stage mood kinds and Naked, HeatWave/ColdSnap/VolcanicWinter incidents, view fields (RoomView heat, BuildingView Open and HeatRate, PawnView Frostbite, AnimalView ComfortMin/Max/Hypothermia/Heatstroke, overlay grid source, active climate event). | None. | Inert names and fields so waves can work in disjoint files. | TEMP-0 | missing |
| Stale checklist rows (vanilla L1098, L1107, MAIN_UI L2177). | Outdoor row described a flat calendar; heat sources `[x]` with no passive cooler; time-controls row said "no temperature/weather". | Corrected in this change. | TEMP-0 | done |

### 3.1 Outdoor climate and events

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Outdoor temperature depends on latitude, time of day, day of year, biome and events; daily swing and seasonal formula unpublished (wiki: Temperature). | `Climate{YearMean, Amplitude, PeakDay, DaySwing}`: a yearly triangle wave plus a linear day/night swing, coldest 05:00, warmest 15:00 (`T:30-103`). Biome climates in `domain/world/biome/biome.go:74-110`, planet tile climates in `domain/world/planet/query.go:199-209`, applied at `G setup.go:179-183`. | Shape is plausible; swing stays original tuning. | TEMP-2 | partial |
| Biome summer max / winter min: temperate forest and swamp 35/-25; tropical and arid shrubland 35/0; desert 35/-25; extreme desert 35/-5; boreal forest and cold bog 25/-35; tundra 25/-50; ice sheet 10/-70; sea ice 10/-60 (wiki: Biomes). | Original per-biome YearMean and Amplitude (`biome.go:74-110`), not checked against the table. | Check every biome's extremes against the table and retune. | TEMP-2 | partial |
| Heat wave: +17 C ramped over 12,000 ticks, lasts 1.5-3.5 days, needs summer at or above 20 C, 30-day cooldown (wiki: Events). | +12 C, 2-4 days, 6 %/day, 3-day cooldown, only with the wizard's Extreme weather option; shifts YearMean (`G weather_ng1.go:16-79`). | Wiki offset, ramp, duration, condition and cooldown as a storyteller incident. | TEMP-2 | partial |
| Cold snap: -20 C over 12,000 ticks, 1.5-3.5 days, summer temperature condition, 30-day cooldown (wiki: Events). | -14 C, same gating as heat wave (`weather_ng1.go:16-79`). | Same. | TEMP-2 | partial |
| Volcanic winter: -7 C over 50,000 ticks, 7.5-40 days, light -30 %, wildlife halved, earliest day 60, 140-day cooldown (wiki: Events). | None. | Incident, temperature offset, light and wildlife hooks. | TEMP-2 | missing |
| Heat wave, cold snap and volcanic winter block one another while active; event weights and MTB unconfirmed (wiki: Events). | No exclusion. Extreme weather gates the events. | Mutual exclusion; original weights; Extreme weather scales weight instead of gating. | TEMP-2 | missing |
| Snow and blizzard chill outdoors; rain none; rain or snow puts out campfires (wiki: Weather, Campfire). | Snow -2, blizzard -5, outdoors only (`G weather.go:143-157`); rain or snow extinguishes outdoor campfires (`weather.go:160-167`). | None beyond tuning notes. | none | done |
| Per-tile air temperature: room temperature, else outdoor plus modifiers. | `temperatureAt` (`G climate.go:38-47`): room temp, breached-room residual, else outdoor + weather + steam vent (+6 within 2 cells, `G vents.go`) + fire (`G fire.go:477-490`). | Fold into the equalisation model; steam vent stays as the geothermal feature. | TEMP-1 | partial |
| Endless chill and long summer scenarios. | `G setup.go:225-236`. | None; keep as scenario offsets. | none | done |

### 3.2 Room thermodynamics

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Equalisation every 120 ticks (wiki: Temperature). | Every 250 ticks (`G climate.go:94-104`, phase order power then climate); drift toward outdoor + 3 C at roomLeak = 120/sqrt(cells) per mille, clamped 10-80 (`G heat.go:92-147`). | 120-tick cadence and wiki loss model. | TEMP-1 | partial |
| Loss through walls scales with perimeter / area; per 120 ticks at the table difference, single wall 1x1 -161.47, 2x2 -81.24, 3x3 -54.16, 5x5 -32.49, 7x7 -23.21; double wall roughly half (-81.23, -40.62, -27.08, -16.25, -11.61); a second layer halves, more adds nothing (wiki: Temperature; table marked [Check]). | Walls give no insulation and exposure is not counted; loss depends only on cell count. | Wall exposure counted per room at rebuild (outdoor cells 1 and 2 beyond the wall); double wall halves. | TEMP-1 | missing |
| Thin roof (constructed or thin rock): -57.331 per 120 ticks at the table difference whatever the room size (wiki: Temperature). | The roof is just roofed or not (`G roof.go:118-126`); thin, thick and mountain are treated the same. | Roof exchange term. | TEMP-1 | missing |
| Overhead mountain (thick rock) does not exchange with outdoors, only slight passive cooling at 15 C or more; no fixed mountain-room temperature (wiki: Temperature). | Mountain is "roofed", same leak as any roof. | Thick-roof rule and cooling. | TEMP-1 | missing |
| A room at least 25 % unroofed takes the outdoor temperature next tick; OutdoorsForWork above 25 % unroofed or over 100 unroofed tiles; psychologically outdoors at 300 or more unroofed tiles or open to the map edge with 50 % unroofed (wiki: Rooms). | A partly unroofed room loses the indoor bonus and leaks up to 5x faster (`heat.go:92-147`); an area touching the map edge is outdoors (`domain/world/room/room.go`). | The 25 % rule and the two outdoors definitions. | TEMP-1 | partial |
| Indoor equals outdoor unless heated or cooled (wiki: Temperature). | `Indoor` is the daily mean + IndoorBonus 3 C (`T`). | Remove IndoorBonus. | TEMP-1 | partial |
| Open doors and open vents exchange between rooms and outdoors, doors weaker than vents; no numbers published (wiki: Temperature, Vent). | Doors always act as walls; boundaries are rock, walls, doors and coolers (`G rooms.go:58-73`). No room-to-room flow. | Open-door and open-vent exchange, original rates. | TEMP-1 | missing |
| Rooms cap at 36 regions or 5184 tiles (wiki: Rooms). | Flood fill without that cap (`domain/world/room/room.go`). | Check cap behaviour for large rooms. | TEMP-1 | missing |
| Room rebuild keeps temperature; per-room cache invalidated on structure, roof and door changes. | A new room starts at the average of its cells' old values (`heat.go:44-88`, `G rooms_index.go:261-293`); breached air kept per cell in `g.residual`, fading about 6 % per interval. Rebuilds at `rooms.go:91` and incrementally at `rooms_index.go:192-194`. | Keep; add roof and door invalidation for the new terms. | TEMP-1 | partial |
| Performance: a heat slice is allocated every interval (`heat.go:115`); `temperatureAt` computed per call. | As stated. | Reuse buffers; no per-tick global scans. | TEMP-1 | partial |

### 3.3 Heat sources and sinks

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Heater: +21 heat/s, target 21 C default, 175 W heating, 17 W idle (Heater page; the Temperature page says 18), 50 steel + 1 component, Construction 5; a 9x9 room at -17 C holds about 20 C, 11x11 about 10 C (wiki: Heater). | +220 tenth-degree-cells per interval (`G power_heat.go:23,51-58`); heater 180 W, idle 20 W (`domain/building/power/power.go:107-110`). | Wiki output, watts and cost; the 9x9 and 11x11 proof. | TEMP-1 | partial |
| Cooler: -21 heat/s on the cold side, the hot side heats by an unstated amount; 200 W, 20 W idle, target 21 C, 90 steel + 3 components; one holds a 10x10 at 10-16 C when it is 27-38 C outside (wiki: Cooler). | 220 moved from the cold to the hot side, lost if the hot side is outdoors (`power_heat.go:59-70`); 220 W, idle 20 W. | Wiki output and cost; hot-side amount is original tuning; proof test. | TEMP-1 | partial |
| Thermostat per heater and cooler. | Default 21 C, plus or minus 5 C; on below target -1, off above target +0.5 (`G power.go:40-42`, `power_heat.go:31-32,79-93`). Hysteresis exists. | Keep; re-check range against wiki. | TEMP-1 | done |
| Passive cooler: -11 heat/s, floor 17 C, holds 50 wood, burns 10 per day (about 5 days), cost 50 wood (wiki: Passive cooler). | Not built; the checklist row was `[x]` without it. | Construction, fuel, floor rule. | TEMP-1 | missing |
| Vent: 30 steel, open/closed, impassable, chained vents less efficient; rate unpublished (wiki: Vent). | None (steam vents are a different, geothermal feature, `G vents.go`). | Wall-mounted vent construction, ToggleVent, original rate. | TEMP-1 | missing |
| Campfire: +21 heat/s up to 28 C (Campfire page; Temperature page says 30), 20 wood, burns 10/day (46/day unroofed in rain), light radius 7.17 (wiki: Campfire). | 160 tenths x cells per interval (`G heat.go:25`); banks at 20 C and relights below 16 C (`G fuel.go:152-178`). | Wiki output and 28 C cap; keep a start/stop gap. | TEMP-1 | partial |
| Torch lamp: +3.5 heat/s up to 23 C; 2 wood/day (38 in rain), capacity 20. Standing lamp: no heat (wiki: Torch lamp). | Torch lamps give no heat. | Heat source with cap. | TEMP-1 | missing |
| Wood-fired generator +6; electric smelter +9; electric crematorium +12; fueled stove +4; electric stove +3 heat/s (wiki: Temperature). | Stoves, smelters and generators give no heat. | Production heat sources. | TEMP-1 | missing |
| Fire heat unpublished (wiki: Temperature stub); spontaneous ignition about 235 C; burn damage above 150 C over the comfortable max. | Fire adds 160 x size / 50 (`G fire.go:467-473`). | Original tuning kept; ignition and burn rules checked. | TEMP-1 | partial |
| Body heat: humans 0.3 x BodySize x 4.1666665 (about 1.25 x BodySize) at ambient 40 C or less; other creatures x0.6 (wiki: Temperature). | None. | Pawn and animal heat into the room. | TEMP-1 | missing |
| Steam vent geothermal heat. | +1.5 to the target of its room (`vents.go:119-131`). | Keep as geothermal feature. | none | done |
| Production heat and the heat sources need heat in the room (not outdoors). | Heat divided by room size (tenths x cells / size). | Heat per cell count in heat/s units. | TEMP-1 | partial |

### 3.4 Pawn comfort and exposure

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Human base comfortable range 16-26 C; min = base - cold insulation, max = base + heat insulation; apparel never moves a limit the wrong way (wiki: Temperature). | Base 8-28 C (`domain/pawn/exposure/exposure.go:12`); insulation widens it (`:16-18`). Stats ColdComfort and HeatComfort (`pawn/stat/stat.go:18-19`) via `comfortShift` (`G skills.go:593`); cold-hardy -6 floor; build Thin +2, Heavy -3. | Base range 16-26; stat adjustments reviewed against wiki. | TEMP-3 | partial |
| Hypothermia gain per 60 ticks: max(0.00075, (degrees below min - 10) x 0.0000645); about 32 h to death at 10 C past, about 4.1 h at 100 C past; onset more than 10 C outside the range (wiki: Hypothermia). | Builds 8 C below the floor, rate 5 + degrees/5 per mille per 250 ticks (`exposure.go:21-82`). | Wiki formula and cadence. | TEMP-3 | partial |
| Heatstroke gain: 0.0000645 x x, x = degrees above max + 10 through a curve (1:1 up to 25, 50->40, 100->60, 400->100), floor 0.000375 per 60 ticks; +25 takes about 24.8 h, +100 about 7.4 h (wiki: Heatstroke). | Builds 10 C above the ceiling, same rate shape as cold. | Wiki curve. | TEMP-3 | partial |
| Recovery per 60 ticks: max(0.0015, min(0.015, 0.027 x severity)), about 3.6 h from near 100 %; needs only the temperature back inside the range; both can be present (wiki: Hypothermia). | Recovery 30 per mille inside the range, 10 in the margin; thresholds Mild 200, Serious 600, Recover 400, Fatal 1000 (`exposure.go:21-82`). | Wiki recovery. | TEMP-3 | partial |
| Hypothermia stages: above 0.04 Consciousness -5 %, Manipulation -8 %; above 0.20 -10 %, -20 %, Moving -10 %; above 0.35 -20 %, -50 %, Moving -30 %, Pain +15 %; above 0.62 Consciousness max 10 %, Pain +30 %; 1.0 death. Heatstroke has the same stages with no Manipulation term (wiki: Hypothermia, Heatstroke). | No stage effects: Mild 200 only triggers warmth-seeking and Serious 600 a collapse (`exposure.go`). State in `g.exposure`. | Stage capacity effects through the body; downing through consciousness. | TEMP-3 | missing |
| Frostbite: possible at hypothermia 0.37 or more and 0 C or less, chance rising with severity (unpublished); pain 1.25, 25 % infection untreated, 1 % permanent, never internal organs; parts unpublished (wiki: Frostbite). | None. | Condition on extremities; chance curve is original tuning. | TEMP-3 | missing |
| Moods: Chilly -4, Cold -8, Numbing cold -12, Absolutely freezing -16; Sweaty -4, Hot -8, Sweltering hot -12, Blistering hot -16; do not stack; trigger temperatures unpublished (wiki: Thoughts). | Two stages a side: Cold -4, Freezing -10 (13 C below the floor), Hot -4, Sweltering -10 (6 C above the ceiling) (`domain/pawn/mood/mood.go:20-23,182-185`; picked at `G rooms.go:222-233`). | Four stages a side; original offsets. | TEMP-3 | partial |
| Slept in the cold -4 and slept in the heat -4, one day each, judged on temperature before apparel (wiki: Thoughts). | Slept in cold/heat -4 for a day (`rest/waking.go:62-73`). | Check "before apparel". | TEMP-3 | partial |
| Colonists seek warmth or coolness and choose apparel to suit; a spot is safe within 90 C of the range for rescue (wiki: Temperature). | `needsComfort`/`warmSpot` (`G heat.go:195-282`): job.Warm capped at 2500 ticks, 1000-tick retry, mild line 200; tick loop interrupts (`G tick.go:482-496`); sleepers wake at 400 or more. `warmSpot` scans the room grid. | Keep hysteresis; cache the spot search; re-tune on the new thresholds with a no-flap test. | TEMP-3 | partial |
| Prisoners, guests and other non-colonist humans suffer exposure. | Only colonists (`g.pawnIDs`); prisoners and visitors live in `raiderIDs` and are skipped. | Exposure for prisoners and guests. | TEMP-3 | missing |
| Exposure runs per pawn at a regular cadence. | `phaseClimate` -> `expose()` every 250 ticks (`heat.go:153-185`). | 60-tick equivalent through the cadence, no per-pawn global scans. | TEMP-3 | partial |

### 3.5 Animals

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Per-species comfortable min / max C; defaults 0..40 when unset: alpaca -20/45, dromedary -22/60, thrumbo -65/50, penguin min -70, arctic wolf min -55, capybara min -10, cobra 0/60, cat max 40, chinchilla max 55, insectoids and iguana max 60 (wiki: species pages). | `Roster.Habitat` (Temperate/Cold/Hot) used only for spawning and seasons. No comfort ranges. | Roster fields, wiki where published, default 0..40 otherwise. | TEMP-3 | missing |
| Animals suffer hypothermia and heatstroke by the same formulas and can die. | No exposure for animals. | Exposure in the 60-tick animal bucket decision through a new sub-package. | TEMP-3 | missing |
| Insectoids: "hypothermic slowdown" instead of hypothermia; same formula and stages, no death at 100 %, no frostbite (wiki: Hypothermia). Hunger multipliers -10 / -40 / -95 % by stage (wiki: Saturation). | Absent; noted at `domain/pawn/needs/hunger_rate.go:8-11` (FOOD-1 left the hunger hook unused). | Condition and hunger hook. | TEMP-3 | missing |
| Animals seek shelter. | Not described by the wiki; none. | Original: animals leave extremes only through existing bucket decisions; no new scan. | TEMP-3 | missing |
| Animal and eggs: fertilised eggs fine at 0-50 C. | Egg hatching 0-50 C (`G animal_life.go:434`). | None. | none | done |

### 3.6 Clothing and layers

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Insulation = stuff insulation x apparel factor x quality (legendary x1.8, poor x0.9) (wiki: Temperature, Apparel). | Formula in `economy/item/condition.go:216-232`; quality scales positive values only. | Quality factors per wiki. | TEMP-4 | partial |
| Apparel factors, cold / heat %: parka 200/0; jacket 80/30; duster 60/85; cape 60/85; tuque 50/0; cowboy hat 10/50; bowler hat 10/40; button-down 26/10; T-shirt 22/10; pants 20/8; tribalwear 55/55; simple helmet 15/0 (wiki: Apparel). | Fixed garment insulation (cold/heat in tenths): Parka 200/-30, Tuque 60/0, Duster 40/80, WoolParka 240/-20 (`item.go:423-437`). | Retune parka, duster and tuque; add the others. | TEMP-4 | partial |
| Fixed values, cold / heat: flak vest -1/0; flak jacket -14.4/+3; flak pants -3.5/+1; marine armor -68/+10; marine helmet -4/+2; recon armor -64/+9 (wiki: Apparel). | Flak and marine armor per the wiki (`item.go:423-437`). | Verify recon armor. | TEMP-4 | partial |
| Stuff, cold / heat: cloth 18/18; muffalo wool 28/12; alpaca wool 30/16; megasloth wool 34/12; devilstrand 20/24; hyperweave 26/26; synthread 22/22; plainleather 16/16; thrumbofur 34/22; wolfskin 24/16; bearskin 20/20; camelhide 16/24; patchleather 9/9; wood 8/4; metals 3/0 (wiki: Apparel). | Cloth 180/180, Leather 160/160, Wool 260/100, Wood 80/40, metals 30/0 in tenths (`item/stuff.go:50-72`). | Stuffs that exist retuned; new stuffs only if they exist in the game. | TEMP-4 | partial |
| Layers: skin, middle, outer and headgear worn together; coverage of body parts (wiki: Apparel layers). | Four single slots, Body ("over the shirt"), Head, Legs, Belt (`item.go:393-412`); `craft.Outfit{On [Slots]Piece; Forced}` saved as `OutfitSave{On []Piece}`; outfit sum `craft/garment.go:259-268`. Armor already has plies Skin/Middle/Outer (`domain/combat/armor/apparel.go:31-38`). | Layer slots (skin torso, skin legs, middle, outer, head, belt); sums by layer. | TEMP-4 | missing |
| Garments: T-shirt, button-down shirt, pants, jacket, cowboy hat, bowler hat, tribalwear; tailoring recipes; start kits and traders (wiki: Apparel). | Parka, wool parka, duster, tuque and armor only. | Seven garments, recipes, start kits, traders, art. | TEMP-4 | missing |
| Naked thought (wiki: Thoughts). | None. | Mood and rule. | TEMP-4 | missing |
| Colonists dress to suit the temperature. | `G apparel.go:185-398`: dress plan within 10 C of the day's mean, dress gain, 2 h slot cooldown, urgent dressing; bands in `apparel.go:46-49`. | Layered plan keeping the hysteresis; re-run the dress no-flap tests. | TEMP-4 | partial |

### 3.7 Temperature-driven effects

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Benches comfortable at 10-35 C, x0.7 work speed outside (wiki: Temperature). | See 3.0: reads the wrong scale. | Units fix. | TEMP-0 | partial |
| Fermenting barrel works at -1 to 32 C (wiki: Fermenting barrel). | FermentMinC 7 / MaxC 32 (`craft/ferment.go:27-30,80-85`), read in tenths. | -1 to 32 C and the units fix. | TEMP-0 | partial |
| Food rot: none at 0 C or below; between 0 and 10 C the rate is x(T/10) (wiki: Temperature, Food). | Rot 0 at or below 0 C, linear to full at 10 C (`item.go:743-753`). Matches. | None. | none | done |
| Freezer rooms keep meat frozen. | Works by room temperature (rot rule above). | Proof test with a cooler-fed room. | TEMP-1 | partial |
| Plant growth x(T/6) below 6 C, full at 6-42 C, x(58-T)/16 above 42; stops below 0 C and above 58 C (cactus 75) (wiki: Plants). | `domain/world/plant/plant.go:256-273` matches. | None. | none | done |
| Cold kills plants at a random per-plant threshold between -18 and -10 C, nothing survives below -19; cold-resistant plants (healroot, nutrifungus, fibercorn); leaves regrow in 24 h; growth resumes above 0 C (wiki: Plants). | Fixed per-species frost kill (`plant/farm.go:60-72`, `plant/damage.go`). | Per-plant random threshold, resistant set, leaf loss. | TEMP-2 | partial |
| Drying rack. | Temperature rule at `item.go:630-639`. | Check against wiki drying rule if published. | none | partial |
| Snow melt and fall use the outdoor curve (`G snow.go:36,244`). | Outdoor curve only. | Include event offsets. | TEMP-2 | partial |
| Volcanic winter: light -30 %, wildlife halved. | None. | Hooks. | TEMP-2 | missing |
| No water freezing in vanilla core. | None. | Not a gap. | none | done |

### 3.8 UI

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Temperature readouts: outdoor, room, cursor. | `WorldView` OutdoorTemp (without the snow chill) and IndoorTemp (`G game.go:571`); `PawnView.Temp`, `RoomView.Temp` (`rooms.go:31`); HUD cursor temperature (`hud/global.go:431-474`), biome chip (`hud/biome.go`), cell info (`hud/cellinfo.go:75-78`), rooms panel (`hud/rooms.go:80`). | Room stats heat/exchange. | TEMP-5 | partial |
| Temperature overlay (UI/UX 16.3, MAIN_UI overlay). | None; all five checklist rows `[ ]`. | Overlay fed by a WorldView grid source. | TEMP-5 | missing |
| Thermostat buttons; heater, cooler and vent panes. | `inspector/cmds_power.go:108-113,219-227` (thermostat); `BuildingView.ThermoTarget/Active`. | Vent toggle, passive cooler pane, heat output lines. | TEMP-5 | partial |
| Health tab: hypothermia and heatstroke stages, frostbite. | Status lines "Hypothermia/Heatstroke N%" (`inspector/cmds_pawn.go:254-259`); comfort range (`inspector/inspector.go:224-248`). | Stages and frostbite lines. | TEMP-5 | partial |
| Alerts: hypothermia/heatstroke, "Need warm clothes", hypothermic animals. | Hypothermia/heatstroke grouped (`hud/alerts.go:218-225`). | Two new alerts. | TEMP-5 | partial |
| Climate event chips. | Heat wave and cold snap chips (`hud/chips.go:37-39`). | Volcanic winter chip, active event field. | TEMP-5 | partial |
| Zone warnings. | "Too cold/hot" in `architect/zone_select.go:295-324`. | Retune to the 6-42 C crop range. | TEMP-5 | partial |
| Animal temperature lines; gear tab showing layers. | None. | Both. | TEMP-5 | missing |

## 4. Stale or wrong evidence

Spot-checked by grep on 2026-10-31: `craft_host.go:120` (tenths), `craft/deps.go:45` (whole degrees), `ferment.go:27-30` (7/32), `station.go:35-43` (10/35) and `exposure.go:12` (`Everyday` 80..280, i.e. 8-28 C) are confirmed. `item.go` is `internal/domain/economy/item/item.go`. Checklist rows corrected in this change: vanilla "Outdoor temperature" (the description was a flat calendar with no weather), vanilla "Heat sources/sinks" (`[x]` to `[-]`: no passive cooler, vent, production or body heat, original numbers), MAIN_UI time controls (the HUD shows cursor temperature, biome and weather chips). Still to update by phases: vanilla L721, L1101, L1111, L1166-1167, L1716; UI/UX L783, L951-955, L1886, L1938, L2119; MAIN_UI L842, L2339.

## 5. Phase order

TEMP-0 first (contract and units fix), then TEMP-1 to TEMP-4 in one wave (disjoint files: thermo, events and plants, exposure, clothing sim and art), then TEMP-5 (UI) on the new views. See the plan for the wave and worker split.
