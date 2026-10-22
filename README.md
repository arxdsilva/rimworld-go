# rimworld-go

A RimWorld-inspired colony simulation written in Go with Ebitengine, built in public on YouTube. Original Blender-rendered art; no proprietary RimWorld code, art or data.

<p align="center"><img src="docs/clips/lategame.webp" width="100%" alt="Late game: a stone compound in autumn rain, fields at every stage, a raid met at the sandbags, and a graveyard"></p>
<p align="center"><sub>Late game: a stone compound in autumn rain, fields at every stage, a raid met at the sandbags, and a graveyard</sub></p>

## See it move

<table>
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

Media updated 2026-10-21 · regenerate with `make media clips`

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
- Zones: stockpiles, growing zones, tree zones and pens are painted tile by tile. A tile belongs to one zone only, touching zones of the same kind merge under one outline, and growing zones skip bare rock. The drag preview shows which tiles will be claimed.
- Hauling: colonists carry up to 100 at a time. One trip feeds several nearby building sites and takes the leftovers back to the nearest stockpile.
- Farming and food: growing zones, a crop picker, regrowing berries, cotton for cloth, straw from wheat, campfire cooking, drying racks, spoilage and frost.
- Production: workbench bills, a tailoring bench and clothing (parkas, tuques, dusters).
- Colonists: needs, mood, rooms and impressiveness, recreation, mental breaks, work priorities, schedules and a per-colonist order queue. Every colonist's route and destination show on the map, and drafted colonists can be sent to a spot or ordered to attack.
- Event logs: every colonist, raider and animal remembers what happened to it (bites, hits, injuries, meals, sleep, taming, breakdowns), shown in its inspector.
- Temperature: seasons, room heat from campfires, and hypothermia on winter nights.
- Threats: raids that scale with colony wealth, sieges, sappers, winter wolf packs, and auto-pause on danger.
- Blueprint inspector: click a blueprint or frame to see its status, materials delivered, on the way and in stock, work left, haulers and builder; cancelling refunds delivered materials. Blocked or unreachable work is put off so colonists move on to something else, and the colonist inspector shows why.
- Floors, fences and forbid: floors can go under beds and benches; wood and stone fences and gates keep animals in (tame animals use gates, colonists climb fences slowly); forbid items with the Forbid/Allow tools or F so colonists leave them alone.
- Defences and combat: sandbags, spike traps, breaching raiders, rebuilding after breaches, ranged and melee combat with floating damage numbers.
- Health: injuries, rescue and tending, infections, herbal medicine and a healroot crop.
- Blood: hits spray blood (ichor from hive insects), wounded colonists and animals look bloodied until they heal, and the floor keeps splats, drip trails and pools that dry to brown, fade over 7 days and wash off in rain or snow.
- Dirt and cleaning: colonists pick up dirt outdoors (more in rain and field work) and track it into rooms; entrance carpets of straw, wood, leather or cloth catch 30-70% of it; colonists sweep dirt and blood inside the Home area you paint, and dirty rooms lose impressiveness.
- Animals: 61 species (livestock, wild grazers and predators, pets, birds, reptiles, hive insects and a rare legendary stag) that spawn by season and habitat, hunt, flee, gore and kick; hunting, butchering, taming, pens, manhunter packs and hive swarms.
- Research: a research bench with five projects that unlock buildings and work.
- Weather: rain, storms, snow and blizzards with wind, puddles and snow cover; rain douses outdoor fires, snow chills the air, and colonists get soaked.
- A living world: grass, crops and trees wave in gusting wind and part around anyone walking through; trees turn in autumn and stand bare and snowy in winter.
- Sun, moon and shadows: every sprite has a normal map, lit by a sun that rises in the east and sets in the west (lower in winter) and by a cool moon at night; shadows swing and stretch with it, long at dawn and dusk. Colonists and animals cast their own animated silhouettes, sharp at the feet and soft at the tip, plus shadows away from campfires and lamps at night.
- Roofs: colonists roof closed rooms on their own, or wherever you plan one within 6 tiles of a wall; unroofed rooms lose their heat, roofs keep rain and snow off, and unsupported roofs collapse. Press Shift+R for the roof overlay.
- Death and burial: the dead leave bodies that rot to bones; colonists mourn, dig graves with headstones, bury their own, or cremate on a funeral pyre.
- Sound and options: animal calls, cries, attacks and deaths for every species; code-made sound effects that pan with the camera and fade as you zoom out, ambience for day, night and rain, and composed music for the menu, colony life and raids. Options cover volumes, fullscreen, effects detail, autosave interval, edge scrolling and reduced motion, and are saved between runs.
- Save and load, with an autosave every in-game day (adjustable in Options).

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
| Overlays | Shift+R roofs, G room stats, hold Alt for cell details |
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
