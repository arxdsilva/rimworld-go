# SIM-033 Landing-site maps and water

Part of NG-1 (`.cursor/NG1_NEW_GAME_PLAN.md` §4.3, §5.6, §7.3; decision D12). Commits: `cf293081` (generation), `f815d545` (rules in play), `f7bebeb1` (art), `6cf941f2` (ambience).

## What changed

### Terrain (`domain/world/terrain`)
Four new terrain types, appended so saved codes keep their meaning: shallow water (wadeable, walk cost 300 %, wetness 1000, unbuildable), deep water (impassable), mud (walk 140 %, wetness 800, fertility 70) and dirt track (walk 90 %, not farmable). `terrain.IsWater` is shallow or deep.

### Site generation (`domain/world/generation/site.go`)
`GenerateSite(seed, w, h, biome, site.Site)` builds the ground of the biome under the site's hills, then caves, coast, river, lakes, wet ground, a road track, a reachability safety pass and wild plants. **The zero `Site` is exactly `GenerateBiome`** (`TestZeroSiteMatchesLegacy`), so legacy maps and goldens do not move. Each pass draws from its own `randx` salt, so turning one feature on never moves another; the terrain noise and plants keep the legacy stream.

- **Hills:** per level rock and stone noise thresholds and an edge-bias weight (flat 0.88/0.80, small 0.82/0.73, large 0.75/0.66, mountainous 0.66/0.58 with a mountain ridge band across the map); the open start radius is always kept clear.
- **Caves:** tunnel systems in large hills and mountains, kept 9 tiles from the centre, with cave-mouth art.
- **Coast:** one or two edges facing the ocean: a deep band of about 18 % of the map with a jittered shoreline (plus or minus 6 tiles), 5 shallow tiles and beaches.
- **Rivers:** creek (3 tiles wide), river (9, deep core 3), large river (15, deep core 7), from the site's entry edge to its exit edge, with mud banks and **fords** (shallow crossings about every 45 plus or minus 10 tiles, 3 wide).
- **Lakes:** up to three noisy blobs of radius 6 to 14, outside the start area, ringed by marsh or mud.
- **Wet ground:** within 4 tiles of river or lake water, soft ground turns to marsh in marshy biomes (48 % next to the water, 12 points less per tile out to 12 % at 4 tiles), otherwise soil within 3 tiles becomes rich silt 40 % of the time. **Tracks:** a 2-wide packed dirt track from each road edge of the site toward the centre, stopping at the start radius; it cuts through rock but not water.
- **Reachability pass:** water is cleared from within 7 tiles (`WaterGuard`) of the centre first. Then the walkable land must let the start reach at least 70 % of it (`ReachShare` 700 permille); otherwise 3-wide crossings are carved (up to 8 tries, then 5-wide up to 32), so a player can never be walled in by rock or water. A 500x500 site generates in about 65 ms.
- **Geology:** the site's stones and buried machinery veins (see SIM-034) come from `geology.Generate`.

### Water rules in play (`game/water.go`, `placement.go`, `filth.go`)
- Nothing is built on water: placement refuses any water cell in a footprint with "Can't build on water" (`ReasonWater`). Items and spawns use `dryWalkable`: animals and raiders may cross shallow water but never appear in it, and colonists, pods and drops land on dry ground.
- **Walking:** shallow water, mud, marsh, sand and ice slow a step by their walk cost unless the tile is floored (`weatherSlow`).
- **Dirt:** wading washes half of the carried dirt off the boots and gives the Soaked thought; mud and wet ground pick up dirt faster (the wetter of weather and ground, doubled on mud).
- **Path cost:** `groundExtra` adds `(walk cost - 100) / 10` to a step into slow ground. It is applied only through `crowdCost`, that is on **crowd repaths**; an ordinary path does not avoid mud or water by cost, only the walkability rules apply. This is deliberate for performance and means colonists may wade when a dry detour is slightly longer.
- Saves keep the new terrain codes; the research list grew (old saves still load).

### Rendering and sound
- **Water** (`ebiten/water_draw.go`, `water.kage`): shallow and deep tiles through a shader with a slow UV ripple and caustic glints on the presentation clock, lit like the ground; shallow-over-deep edges and shore foam from edge masks; without the shader the tile cycles four baked frames. Reduced Motion freezes both. Mud-over-soil edges and the dirt track use 16-mask variants (4 variants per type, `terrain/<type>_v0..3`).
- **Arrival:** drop-pod fall streaks and landing dust (`effect/pod_fall`, `effect/pod_dust`).
- **Ambience** (`sound_director.go`): two loops, `amb.water_river` and `amb.water_shore`, whose volume follows the share of water and of shoreline on screen (a sixth of the view in water, a tenth in shoreline, for full volume), damped indoors.
- **Chunk bakes:** water, mud and tracks are baked into the 16 px chunk tier and the 4 px map tier (ENG-001).

## Verification
- Recorded at commit time; the docs pass re-ran nothing. `generation/site_test.go` covers legacy equality (`TestZeroSiteMatchesLegacy`, `TestFeaturelessSiteKeepsLegacyGround`), `TestRiverEdgeToEdgeAllSizes`, `TestRiverWidths`, `TestNoWaterAtStart`, `TestReachableSoftGround70`, `TestCavesOnlyInRock`, `TestCoastDepthBands`, `TestLakeRings`, `TestFordsEveryInterval`, `TestWetGroundBySiteWater`, `TestRoadTrackReachesStart` and determinism; `terrain/water_test.go`; `game/water_test.go` covers `TestWaterPlacementRefused`, `TestPathAvoidsShallow`, `TestMudDirtGain`, `TestWadingSoakedAndWashes`, `TestNothingSpawnsInWater`.
- `BenchmarkGenerateSite500` is the generation-time check (about 65 ms per the commit).

## Known limitations
- No bridges over water, no fishing, no water as a resource (out of NG-1 scope).
- Ordinary pathing ignores slow ground (see Path cost above); only crowd repaths weigh it.
- Deep water is purely impassable: nothing swims or drowns.
- Rivers run edge to edge between the site's entry and exit edges; they do not follow the world tile's exact course.
