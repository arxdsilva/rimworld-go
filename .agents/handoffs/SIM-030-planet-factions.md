# SIM-030 Planet generator, landing-site parameters and factions

Part of NG-1 (`.cursor/NG1_NEW_GAME_PLAN.md` §3.4, §4.1, §4.2, §5.5). Commits: `a85704f2` (planet, site parameters), `37f2935d` (factions), `15b2ca1c`..`f815d545` (raid attribution in play).

## What changed

### Planet (`internal/domain/world/planet`)
- **Mesh:** Goldberg hex globe with 10f²+2 tiles (23,042 at full coverage), 12 pentagons. Coverage 30, 50 or 100 % keeps the tiles inside a spherical cap around a seed-chosen pole. Struct-of-arrays storage with CSR neighbour and corner lists.
- **Builder:** `NewBuilder(Params)`, `Step(units)` in budgeted work units across seven phases (shape, elevation, climate, biomes, rivers, factions, roads), each with its own `randx` salt. Output is identical for every step size (`TestStepBudgetDeterministic`) and across machines; a full globe builds in about 35 ms. `TestPlanetGoldenHash` pins seed "amber-hollow" at 30 %.
- **Elevation:** 6-octave sphere noise plus continent blobs; sea level is a quantile giving about 38 % land (`TestLandShareInRange` allows 31.5 to 44.5 % over 100 seeds). Ocean down to -4000 m, land up to 6000 m with an exponent of 2.4 so most land is lowland.
- **Climate (numbers changed from the plan):**

  | Item | Plan | Built |
  |---|---|---|
  | Latitude falloff | 28 - 52·\|sin lat\|^1.25 | 28 - 52·\|sin lat\|^**3.0** (`tempLatExp`), -6.5 °C per km of height, setting offsets -12/-6/0/+6/+12 |
  | Rain band | 2400 mm equator, 300 at 25°, 900 at 50°, 200 poles | knots at 0°/12°/25°/38°/50°/90° = 2400/**2100**/300/**800**/900/200 mm, smoothstepped |
  | Moisture | 0.35 + 0.65·exp(-d/9) | same, but the decay length is up to **3x longer inside 20° of the equator** (forest recycles rain), plus a smooth ±25 % rain noise field |
  | Rain shadow | -35 % under a 2500 m upwind peak | x0.65 with a peak within 3 tiles upwind (westerlies outside 30° latitude, easterlies inside) |
  | Tropical biomes | from 20 °C | from **18 °C**, where the temperate rows end, so no temperature falls between them |
  | River sources | rain > 1000, elevation > 400 m | rain > **500**, elevation > **200 m**, 6 % per tile |

  Sea ice: ocean colder than -8 °C is stored as land with biome Sea Ice.
- **Biomes:** the 12 biomes are classified on mean temperature, rain, hills and elevation in table order (`Classify`, tested by `TestBiomeClassificationTable`). Hills come from the largest neighbour height difference plus ridge noise (flat under 150, small under 400, large under 900, else mountainous; above 3000 m at least large). Caves: 30 % of mountainous and 12 % of large-hill tiles. Each tile gets 2 to 3 of five stones from smooth noise fields, so neighbours share stones.
- **Rivers and lakes:** sources flow down the steepest descent to water or ice, joining rivers on the way; a basin of 2 or more tiles becomes a lake, a single-tile pit spills over its lowest rim. Flow sets creek, river (4) or large river (12).
- **Roads:** minimum spanning tree over each landmass's settlements plus extra loops, A* over tiles with hill and biome costs (`TestRoadsConnectSettlements`).
- **Queries** (`query.go`): `TileAt`, `Site`, `GrowingDays`, `Hops`, `Nearest`, `ValidLanding(i) (ok, reason)`, `RandomLanding(n)`, `SettlementFaction`, `SettlementName`.
- **Tuning follow-up (flag):** over 100 seeds arid shrubland covers about **40 % of land**, far more than any other biome (figure measured by the lead, not re-measured here). Cause, read from `Classify`: the table falls through to arid shrubland for every tile at 18 °C or warmer with under 1900 mm of rain (and for cool tiles with 250 to 550 mm), and the plan's table has the same fall-through. The classification itself matches its tests; the world just reads dry. Candidates: lower the 1900 mm rainforest line, or add a warm seasonal-forest rule for 550 to 1900 mm. Not changed here.

### Landing-site parameters (`world/site`)
`site.Site` carries biome, climate, hills, river (size, in and out edge), coast edges, lakes (0 to 3), caves, stones (2 to 3), road edges and latitude. The tile's climate gives amplitude from 6 °C at the equator to 22 °C at 60°, plus up to 4 °C of continentality; a day swing of 12 °C in dry places down to 5 °C in wet ones; the temperature peak day is 15 in the north and 45 in the south (`Climate.SeasonAt`, hemisphere-aware). River edges and coast edges come from neighbour bearings. The zero `Site` is the legacy map.

### Factions (`internal/domain/faction`)
- **Types** (`Def` table): Settlers (industrial, up to 4), Clans (neolithic, 4), Drifters (industrial, 3), Freebooters (industrial, 3, raid points x1.2), Swarm (hidden, insect). All original; the player is a hidden type.
- **Names:** original grammars (adjective + noun, clan body-part compounds, two-syllable place names, 660 settlement names, leader names and titles per type); unique within a world (`uniqueName` numbers a clash with a roman numeral).
- **Goodwill:** start inside each type's range; Freebooters and Swarm are fixed at -100 (`Fixed`, permanent hostility); `Adjust` clamps -100..100 and keeps the last five events; `DailyDrift` moves one point toward the type's baseline and keeps 60 days of history.
- **Hysteresis (no flap):** hostile at or below -50 and calm again only at or above -10; friendly at or above +50 and neutral again only below +30 (`RelationFor`, `TestFactionHostilityHysteresis`). Types that cannot be friendly stay neutral.
- **Settlements:** Poisson-disc over land with spacing 8 (sparse), 6 (normal) or 4 (crowded); count = land x density x population; capital first, the rest tried within 18 tiles of it; never on ice sheet, sea ice or extreme desert; Clans prefer forest, swamp, bog and tundra, Settlers prefer temperate forest, shrubland and desert, Freebooters prefer hills (`place.go`). No landing within one tile of a settlement (`ValidLanding`).
- **In play** (`game/factions.go`, `story.go`): a setup colony's raids come from a hostile human faction chosen by `faction.PickRaider` (weighted by settlement count); with none hostile the raid is replaced by a pack, manhunters or a hive. The raid group carries its faction: the letter names it, raiders get generated names, Neolithic factions field no rifles. When a raid ends (no raider left standing) the faction loses 5 goodwill (`raidRepelled`) with an event line. Goodwill drifts at each day rollover. Read model: `Simulation.Factions()` for the Factions window.
- **Legacy colonies** have no factions; their raids stay "Raiders".

## Verification
- Recorded at commit time; the docs pass re-ran nothing. Domain tests exist for tile count, pentagons, neighbour symmetry, coverage fraction, land share, biome table, river descent, road connectivity, golden hash, step-size independence, site parameters, landing rules, faction placement, hysteresis and names.
- Planet build time (about 35 ms for a full globe) is from the commit message; `BenchmarkPlanetBuild100` is the check for the plan's 1.5 s budget.

## Known limitations
- Arid shrubland is about 40 % of land (see Tuning follow-up).
- Movement difficulty is informational (no world travel).
- Goodwill changes only by raid outcomes and daily drift: no gifts, quests, trade or aid.
- No faction territory beyond settlement placement, no settlement attack maps, no destroying settlements.
- No mechanoid or ancient factions.
- Out of scope per plan §0.5: trade, caravans, world travel, quests.
