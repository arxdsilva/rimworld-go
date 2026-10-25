# rimworld-go

A RimWorld-inspired colony simulation written in Go with Ebitengine, built in public on YouTube. Original Blender-rendered art; no proprietary RimWorld code, art or data.

**Maps up to 500×500 tiles at 60+ fps.** A grown 500×500 colony held 117 fps on average (1% low 68) over a ten-minute run at 3× speed on an Apple M3 Pro, at about 0.7 ms per simulation tick.

<p align="center"><img src="docs/clips/lategame.webp" width="100%" alt="Late game: a stone compound in autumn rain, fields at every stage, a raid met at the sandbags, and a graveyard"></p>
<p align="center"><sub>Late game: a stone compound in autumn rain, fields at every stage, a raid met at the sandbags, and a graveyard</sub></p>

## See it move

<table>
  <tr>
    <td width="50%"><img src="docs/clips/bigmap.webp" width="100%" alt="A 500x500 map at 3x speed, zoomed all the way out: a grown colony in the middle, the minimap showing how much land is left"><br><sub>A 500×500 map at 3× speed, zoomed all the way out: a grown colony in the middle, the minimap showing how much land is left</sub></td>
    <td width="50%"><img src="docs/clips/bigmap_colony.webp" width="100%" alt="The same 500x500 colony closer in: a full colony at work, still at 3x speed"><br><sub>The same 500×500 colony closer in: a full colony at work, still at 3× speed</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/daycycle.webp" width="100%" alt="A summer day in a minute: the sun rises in the east, shadows swing west to east, then moonlight"><br><sub>A summer day in a minute: the sun rises in the east, shadows swing west to east, then moonlight</sub></td>
    <td width="50%"><img src="docs/clips/roofs.webp" width="100%" alt="Snow at dusk: a roofed room warm by its fire, its unroofed twin dark and cold"><br><sub>Snow at dusk: a roofed room warm by its fire, its unroofed twin dark and cold</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/colony_life.webp" width="100%" alt="Colonists going about their work"><br><sub>Colonists going about their work</sub></td>
    <td width="50%"><img src="docs/clips/building.webp" width="100%" alt="The first hut going up from its blueprint"><br><sub>The first hut going up from its blueprint</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/melee.webp" width="100%" alt="Knife, spear, club and rifle in a close fight"><br><sub>Knife, spear, club and rifle in a close fight</sub></td>
    <td width="50%"><img src="docs/clips/combat.webp" width="100%" alt="A firefight, followed shot by shot"><br><sub>A firefight, followed shot by shot</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/raid.webp" width="100%" alt="Raiders advance on the colony"><br><sub>Raiders advance on the colony</sub></td>
    <td width="50%"><img src="docs/clips/chores.webp" width="100%" alt="Chopping with a two-handed axe, then on to the next tree"><br><sub>Chopping with a two-handed axe, then on to the next tree</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/rain.webp" width="100%" alt="A fall storm: wind-slanted rain, puddles and swaying trees"><br><sub>A fall storm: wind-slanted rain, puddles and swaying trees</sub></td>
    <td width="50%"><img src="docs/clips/snow.webp" width="100%" alt="Snow drifting in on the wind and settling on the ground"><br><sub>Snow drifting in on the wind and settling on the ground</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/grass.webp" width="100%" alt="Grass and flowers waving in the wind as a colonist walks by"><br><sub>Grass and flowers waving in the wind as a colonist walks by</sub></td>
    <td width="50%"><img src="docs/clips/burial.webp" width="100%" alt="Laying the dead to rest: graves, headstones and a funeral pyre"><br><sub>Laying the dead to rest: graves, headstones and a funeral pyre</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/animals.webp" width="100%" alt="Tamed animals in their pen"><br><sub>Tamed animals in their pen</sub></td>
    <td width="50%"><img src="docs/clips/zoo.webp" width="100%" alt="All 61 animal species, penned and named"><br><sub>All 61 animal species, penned and named</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/shadows.webp" width="100%" alt="A colonist's own shadow from dawn to dusk: legs, clothes and tools, sharp at the feet and soft at the tip"><br><sub>A colonist's own shadow from dawn to dusk: legs, clothes and tools, sharp at the feet and soft at the tip</sub></td>
    <td width="50%"></td>
  </tr>
</table>

## Progress

Media updated 2026-10-23 · regenerate with `make media clips`

### Building

<table>
  <tr>
    <td width="50%"><img src="docs/media/colony.webp" width="100%" alt="A young colony: colonists, a hut, stockpile and wild land"><br><sub>A young colony: colonists, a hut, stockpile and wild land</sub></td>
    <td width="50%"><img src="docs/media/architect.webp" width="100%" alt="Architect menu: placing stone walls"><br><sub>Architect menu: placing stone walls</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/media/farm.webp" width="100%" alt="Growing zones and the crop picker"><br><sub>Growing zones and the crop picker</sub></td>
    <td width="50%"><img src="docs/media/bills.webp" width="100%" alt="Workbenches and production bills"><br><sub>Workbenches and production bills</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/media/rock.webp" width="100%" alt="Linked walls and rock: every wall shape, five stones, smoothed rock, ore veins and a mined-out hole"><br><sub>Linked walls and rock: every wall shape, five stones, smoothed rock, ore veins and a mined-out hole</sub></td>
    <td width="50%"><img src="docs/media/rock_close.webp" width="100%" alt="Close up: rock masses read as one surface, and walls join the outcrop"><br><sub>Close up: rock masses read as one surface, and walls join the outcrop</sub></td>
  </tr>
</table>

### Threats

<table>
  <tr>
    <td width="50%"><img src="docs/media/siege.webp" width="100%" alt="Raiders dig in for a siege"><br><sub>Raiders dig in for a siege</sub></td>
    <td width="50%"><img src="docs/media/defense.webp" width="100%" alt="A sandbag ring and spike traps as a raid arrives"><br><sub>A sandbag ring and spike traps as a raid arrives</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/media/combat.webp" width="100%" alt="Armed colonists hold the line against a raid"><br><sub>Armed colonists hold the line against a raid</sub></td>
    <td width="50%"></td>
  </tr>
</table>

### Colony life

<table>
  <tr>
    <td width="50%"><img src="docs/media/animals.webp" width="100%" alt="A pen of tamed animals: a husky, a cow, a horse and a goat"><br><sub>A pen of tamed animals: a husky, a cow, a horse and a goat</sub></td>
    <td width="50%"><img src="docs/media/research.webp" width="100%" alt="Research bench and the project list"><br><sub>Research bench and the project list</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/media/medic.webp" width="100%" alt="A tended infection and a healroot crop at each growth stage"><br><sub>A tended infection and a healroot crop at each growth stage</sub></td>
    <td width="50%"><img src="docs/media/winter.webp" width="100%" alt="Winter night: a heated hut vs a cold one"><br><sub>Winter night: a heated hut vs a cold one</sub></td>
  </tr>
</table>

## What works today

- Building: generated maps, cutting, mining, walls, doors, beds, multi-tile workbenches and a stove, all placed from the Architect menu.
- Stone: each map has two or three of five stones (sandstone, granite, limestone, slate, marble) and steel, silver and gold veins. Mining drops a chunk of the rock's stone, a stonecutter cuts it into blocks, and each stone builds walls with its own strength and build time. Builders can smooth rock walls and stone floors. Walls, doors and rock connect into one continuous surface.
- Zones: stockpiles, growing zones, tree zones and pens are painted tile by tile. A tile belongs to one zone only, touching zones of the same kind merge under one outline, and growing zones skip bare rock. The drag preview shows which tiles will be claimed.
- Hauling: colonists carry up to 100 at a time. One trip feeds several nearby building sites and takes the leftovers back to the nearest stockpile.
- Farming and food: growing zones, a crop picker, regrowing berries, cotton for cloth, straw from wheat, campfire cooking, drying racks, spoilage and frost.
- Production: workbench bills, a tailoring bench and clothing (parkas, tuques, dusters).
- Colonists: needs, mood, rooms and impressiveness, recreation, mental breaks, work priorities, schedules and a per-colonist order queue. Every colonist's route and destination show on the map, and drafted colonists can be sent to a spot or ordered to attack.
- Event logs: every colonist, raider and animal remembers what happened to it (bites, hits, injuries, meals, sleep, taming, breakdowns), shown in its inspector.
- Temperature: seasons, room heat from campfires, and hypothermia on winter nights.
- Threats: raids that scale with colony wealth, sieges, sappers, winter wolf packs, and auto-pause on danger.
- Blueprint inspector: click a blueprint or frame to see its status, materials delivered, on the way and in stock, work left, haulers and builder; cancelling refunds delivered materials. Blocked or unreachable work is put off so colonists move on to something else, and the colonist inspector shows why.
- Floors, fences and forbid: floors can go under beds and benches; wood and stone fences and gates keep animals in (tame animals use gates, colonists climb fences slowly); forbid items with the Forbid/Allow tools or F so colonists leave them alone. A door or gate can be placed straight over a wall or fence: the old piece comes down first, then the new one goes up. Doors and gates face along their walls and fences on their own, and painting a wall, floor or piece of furniture in another material changes it in place (the room stays sealed). Walls, doors, fences and gates can't be moved. The Plan tool paints layout marks that vanish once something is built on them.
- Defences and combat: sandbags, spike traps, breaching raiders, rebuilding after breaches, ranged and melee combat with floating damage numbers.
- Health: injuries, rescue and tending, infections, herbal medicine and a healroot crop, industrial medicine for severe wounds, and hospital beds that heal faster and cut infections in half.
- Blood: hits spray blood (ichor from hive insects), wounded colonists and animals look bloodied until they heal, and the floor keeps splats, drip trails and pools that dry to brown, fade over 7 days and wash off in rain or snow.
- Dirt and cleaning: colonists pick up dirt outdoors (more in rain and field work) and track it into rooms; entrance carpets of straw, wood, leather or cloth catch 30-70% of it; colonists sweep dirt and blood inside the Home area you paint, and dirty rooms lose impressiveness.
- Animals: 61 species (livestock, wild grazers and predators, pets, birds, reptiles, hive insects and a rare legendary stag) that spawn by season and habitat, hunt, flee, gore and kick; hunting, butchering, taming, pens, manhunter packs and hive swarms. Any animal that is attacked fights back or runs (penned ones run within the pen), predators may charge anyone who walks too close, and unarmed colonists run from an animal coming for them.
- Movement: colonists and tame animals walk through each other instead of queueing; only in a fight do colonists keep off each other's tiles.
- Research: a research bench with seven projects that unlock buildings and work, including Fabrication (components, tool cabinets, hospital beds) and Pharmacology (industrial medicine).
- Weather: rain, storms, snow and blizzards with wind, puddles and snow cover; rain douses outdoor fires, snow chills the air, and colonists get soaked.
- A living world: grass is many small tufts that blend from tile to tile and grows back from shoots after grazing; trees have trunks and canopies that turn see-through when someone walks under them; brambles, moss, desert rosettes, ball cacti, elephant ears, fire lilies and dandelions fill out each biome; everything waves in gusting wind and parts around anyone walking through; trees turn in autumn and stand bare and snowy in winter.
- Sun, moon and shadows: every sprite has a normal map, lit by a sun that rises in the east and sets in the west (lower in winter) and by a cool moon at night; shadows swing and stretch with it, long at dawn and dusk. Colonists and animals cast their own animated silhouettes, sharp at the feet and soft at the tip, plus shadows away from campfires and lamps at night.
- Roofs: colonists roof closed rooms on their own, or wherever you plan one within 6 tiles of a wall; unroofed rooms lose their heat, roofs keep rain and snow off, and unsupported roofs collapse. Press Shift+R for the roof overlay.
- Death and burial: the dead leave bodies that rot to bones; colonists mourn, dig graves with headstones, bury their own, or cremate on a funeral pyre.
- Sound and options: every species has its own call, attack, hurt and death voice (generated with ElevenLabs, reviewed by ear); code-made sound effects that pan with the camera and fade as you zoom out, ambience for day, night and rain, and composed music for the menu, colony life and raids. Options cover volumes, fullscreen, effects detail, autosave interval, edge scrolling and reduced motion, and are saved between runs. The UI scales sharply with the window (Auto, or 75–200 %) and renders at full resolution on high-density screens.
- Save and load, with an autosave every in-game day (adjustable in Options). Commitment mode keeps one save per colony that the game updates itself: no reloading, Save & Quit, no developer mode.
- A full new-game flow: pick a scenario (Broken Hull, Ember Clan, Gilded Expedition, Bare Hands, or build your own in the Scenario Editor with validation and saved files), a storyteller (Orla Vane, Bram Holloway or Kestrel Quay, each pacing threats differently), and a difficulty (six presets or a custom one with 19 settings).
- A generated world: type a seed and get a rotating hex globe with continents, climate by latitude and altitude, rain shadows, twelve biomes, hills, rivers, lakes, roads and named factions. Pick a landing site from a full tile panel (temperature, rainfall, growing period, stones, water, caves, nearest settlement), choose the starting season, and open the Factions dialog.
- Natural-looking colony maps: a staged, seed-reproducible generator builds each landing site from fields. Mountains lean toward the world's high ground with irregular cliffs and winding, branching caves; stone forms broad regions; rivers meander with banks, fertile floodplains, canyons and fords; coasts are ragged; trees grow in groves with clearings. You also find ore lumps deep in rock, stone debris near cliffs, smoothed roads, small ruins to salvage and steam vents that warm the air. The landing spot is chosen for open, fertile, reachable ground. Fog of war hides sealed caves and rock until you mine into them, and deep rock overhead is permanent. `go run ./cmd/mapgen -out dir` writes a PNG of every field (elevation, fertility, wetness, stone, caves, water distance, terrain, vegetation, resources, landing score, regions, roofs, fog), 12-seed contact sheets (`-grid 12`) and hilliness sweeps (`-sweep hills`); in-game, developer mode's "Map fields" shows the same layers over the live map.
- Create your characters: generated candidates you can reroll, rename, reorder and swap, with backstories, traits, skills and passions, what each cannot do, health conditions and warnings when the team has no doctor, cook, grower, builder, researcher or fighter. Colonists arrive in drop pods (or on foot, or standing in a ring, by scenario).
- Colonist identity and skills: eleven skills with passions that set work speed and learning, 19 traits, 59 original backstories that can rule out kinds of work, six body builds with new hair and beards, and a Bio tab and a Work tab that hatches the cells a colonist cannot do. Older saves keep their colonists as before.
- Factions: settlers, clans, drifters, freebooters and the hive swarm settle the globe; raids come from a named hostile faction, goodwill moves with raids and drifts back, and a Factions window shows goodwill history. Storyteller threat points follow wealth, colonists, time and an adaptation that eases after losses; heat waves and cold snaps come with the Extreme weather setting.
- Water and landing-site maps: the chosen tile shapes the map with hills, caves, coasts, rivers with fords, lakes, mud, wet ground and dirt tracks, always leaving the start able to walk out. Water is rendered with a ripple shader and sounds; wading soaks colonists and washes their boots, and nothing can be built on water.
- Components and industrial medicine: fabricate components from steel or mine buried machinery, refine herbal medicine, and build tool cabinets that speed benches and hospital beds.
- Real font and minimap: Noto Sans throughout the menus, HUD and inspectors, and a minimap (Shift+M, click to move) on big maps. Map sizes from 200x200 to 500x500, holding about 60 fps or better at 3x speed on a grown 500x500 colony.

The full backlogs are in the [vanilla feature checklist](.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md) and the [UI/UX checklist](.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md).

## Run it

Ready-made builds for Windows, macOS (Apple Silicon and Intel) and Linux are on
the [Releases](https://github.com/arxdsilva/rimworld-go/releases) page: unpack
the archive and run `rimworld-go`, keeping the `assets` folder next to it. The
macOS build is unsigned, so open it the first time with right-click → Open.
Pushing a `v*` tag builds and publishes a new release
([`release.yml`](.github/workflows/release.yml)).

To build from source you need Go 1.26.5 or later (the version in [`go.mod`](go.mod)).

```sh
go run ./cmd/game   # start the game
make build          # build every command into bin/
make test           # unit tests with the race detector and coverage
make simulate       # headless simulation run for quick checks
```

Developer flags for `cmd/game`: `-scenario <name>` starts a set piece (for example `-scenario grown -width 250 -height 250` is the large-map benchmark colony), `-fpsprobe N` runs the colony for N seconds of real frames at `-capture-speed` and `-capture-zoom` and logs fps, 1% low and update/draw times, and `-cpuprofile`, `-memprofile` and `-trace` write Go profiles. `go run ./cmd/benchmark` prints per-tick percentiles for the grown colony at 120, 250 and 500 tiles a side.

Sound plays through your default audio device; `go run ./cmd/game -noaudio`
(or `RIMWORLD_GO_NOAUDIO=1`) runs silently. On Linux, audio needs cgo and
the ALSA headers (`sudo apt install libasound2-dev`). Options (volumes,
fullscreen, effects, autosave, edge scrolling, reduced motion) are saved to
`settings.json` in your user config folder, next to the saves.

### Controls

Every key can be changed in Options › Controls (conflicts ask before
swapping; Reset to defaults puts them back).

| What | Keys |
|---|---|
| Camera | WASD or arrows pan (Shift: faster), PgDn / PgUp or the wheel zoom, middle-drag pans, edge scrolling in Options |
| Time | Space pause, 1 / 2 / 3 speeds, Home / End one speed up / down |
| Tabs | Tab Architect, F1 Work, F2 Schedule, F4 Animals, F5 Wildlife, F6 Research (F3, F7-F9 not yet) |
| View | F10 screenshot (saved under `screenshots` next to the settings), F11 hide the interface |
| Selection | click, Shift+click add/remove, drag a box, double-click for all of a kind on screen, `,` / `.` previous / next colonist, Numpad * next thing in the cell, Z search |
| Selected | R draft / undraft (all selected), F forbid / allow, B H Y N J U M K I L O P the panel's buttons in order |
| Orders | right-click (drafted: the group spreads around the tile; right-drag lines them up), Shift queues |
| Architect | Q / E rotate, or change the shape of drag tools (line, rectangle, oval, filled or empty); Ctrl squares the shape; C Cancel tool, X Deconstruct tool |
| Overlays | Shift+R roofs, G room stats, Shift+M minimap on maps over 150 tiles, hold Alt for cell details |
| Amounts | Ctrl x10, Shift x100, Ctrl+Shift x1000 on bill +/- |
| Developer mode (Options › Gameplay) | 4 ultrafast, Numpad + one tick while paused, / debug actions, ` debug log, ' debug inspector |

### Refreshing the README media

- `make media`: records the gallery as short animated WebPs through the game's `-capture` flag.
- `make clips`: captures frame sequences and packs them into animated WebP with `scripts/frames_to_webp.py`. Needs python3 and Pillow.
- `make art` re-renders the sprites in Blender, then `make normals` writes their normal maps.
- To let an AI agent drive a live Blender (Blender MCP), see [`tools/blender-mcp`](tools/blender-mcp/README.md). `.mcp.json` and `.cursor/mcp.json` register the server for Claude Code and Cursor.

### Sound

- `make audio` renders the sound effects from code (`cmd/audiogen`, recipes in `cmd/audiogen/recipes.go`) and updates the sound list `assets/metadata/audio/audio.json`; `make audio-check` validates it.
- `make music` composes the music in code (`cmd/audiogen/music.go`, `pkg/musicsynth`): the main theme, a calm colony track and a raid loop, encoded to Ogg with ffmpeg.
- Ambience comes from ElevenLabs through its MCP server, registered in `.mcp.json`. Put your key in your shell profile as `export ELEVEN_LABS_API_KEY=...` (the config maps it to the server's `ELEVENLABS_API_KEY`), then restart your editor so it inherits it. Prompts and the credit budget are in `assets/source/audio/prompts.json`; import an approved take with `scripts/audio_import.sh <raw> <file> <credits> "<license>"`.

## Credits

Normal maps are generated with a Go port of [spritefusion-sprite-normals](https://github.com/Hugo-Dz/spritefusion-sprite-normals) by Hugo Duprez (MIT); see [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).

## Project layout & contributing

Read [`AGENTS.md`](AGENTS.md) before changing code. It sets the rules: hexagonal architecture, and a headless, deterministic domain. Specs live in [`.cursor/DOCUMENT_INDEX.md`](.cursor/DOCUMENT_INDEX.md), and the art pipeline is documented in [`assets/AGENTS.md`](assets/AGENTS.md).
