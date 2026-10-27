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
    <td width="50%"><img src="docs/clips/melee.webp" width="100%" alt="Knife, spear, club and rifle in a close fight: weapons held across the body and aimed from it, wounds that bleed"><br><sub>Knife, spear, club and rifle in a close fight: weapons held across the body and aimed from it, wounds that bleed</sub></td>
    <td width="50%"><img src="docs/clips/combat.webp" width="100%" alt="A firefight, followed shot by shot"><br><sub>A firefight, followed shot by shot</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/raid.webp" width="100%" alt="Raiders advance on the colony"><br><sub>Raiders advance on the colony</sub></td>
    <td width="50%"><img src="docs/clips/chores.webp" width="100%" alt="Chores up close: colonists lean into the job, a progress bar under the work"><br><sub>Chores up close: colonists lean into the job, a progress bar under the work</sub></td>
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
    <td width="50%"><img src="docs/clips/shadows.webp" width="100%" alt="A colonist's own shadow from dawn to dusk, sharp at the feet and soft at the tip"><br><sub>A colonist's own shadow from dawn to dusk, sharp at the feet and soft at the tip</sub></td>
    <td width="50%"><img src="docs/clips/medical.webp" width="100%" alt="A colonist bleeding from deep cuts is carried to a hospital bed and tended; the bleed rate and death clock show until the wounds are bound"><br><sub>A colonist bleeding from deep cuts is carried to a hospital bed and tended; the bleed rate and death clock show until the wounds are bound</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/surgery.webp" width="100%" alt="Surgery: a doctor fits a peg leg in a hospital bed beside a powered vitals monitor"><br><sub>Surgery: a doctor fits a peg leg in a hospital bed beside a powered vitals monitor</sub></td>
    <td width="50%"><img src="docs/clips/outbreak.webp" width="100%" alt="A flu outbreak: the letter, the disease alert, and the sick on bed rest"><br><sub>A flu outbreak: the letter, the disease alert, and the sick on bed rest</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/clips/turret.webp" width="100%" alt="Two powered sentry turrets open fire as a raid reaches the house"><br><sub>Two powered sentry turrets open fire as a raid reaches the house</sub></td>
    <td width="50%"><img src="docs/clips/trade.webp" width="100%" alt="A trader caravan arrives; a colonist walks over and the trade dialog opens with prices set by Social and goodwill"><br><sub>A trader caravan arrives; a colonist walks over and the trade dialog opens with prices set by Social and goodwill</sub></td>
  </tr>
</table>

## Progress

Media updated 2026-10-26 · regenerate with `make media clips`

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
- Zones: stockpiles, growing zones and tree zones are painted tile by tile. A tile belongs to one zone only, touching zones of the same kind merge under one outline, and growing zones skip bare rock. The drag preview shows which tiles will be claimed. Click a zone (an empty tile or its label) to manage it, as in RimWorld: change its crop, allow or stop sowing and cutting, Expand or Shrink it with a drag, delete, hide or rename it. A sowing tile is claimed by one grower at a time, and planted tiles show their furrows and crops without the zone's tint over them.
- Hauling: colonists carry up to 100 at a time. One trip feeds several nearby building sites and takes the leftovers back to the nearest stockpile. Haulers bring in only the colony's own goods (what it makes, brings, harvests, chops, mines or hunts); things found lying on the map, like ruin loot, stone chunks or a predator's kill, stay put until you mark them with Orders → Haul things (or the item's Mark for hauling button).
- Farming and food: growing zones (growers first clear whatever else grows on a cell, then sow; ripe crops get a harvest mark), a crop picker, regrowing berries, cotton for cloth, straw from wheat, campfire cooking, drying racks and spoilage. Crops share the grass and bushes' soft, muted look: each planted tile shows a few small plants in rows over tilled furrows, growing smoothly from seed specks to ripe produce. Below a crop's frost line (or in great heat) it takes damage over a few hours: the leaves frost or scorch, then wilt, shrink and thin out until the plants are gone, and a damaged harvest yields less.
- Production: workbench bills, a tailoring bench and clothing (parkas, tuques, dusters). Colonists change straight into the best outfit they're comfortable in, even mid-job; a Wear order you give sticks.
- Colonists: needs, mood, rooms and impressiveness, recreation, mental breaks, work priorities, schedules and a per-colonist order queue. Every colonist's route and destination show on the map, and drafted colonists can be sent to a spot or ordered to attack.
- Event logs: every colonist, raider and animal remembers what happened to it (bites, hits, injuries, meals, sleep, taming, breakdowns), shown in its inspector.
- Temperature: seasons, room heat from campfires, and hypothermia on winter nights.
- Threats: raids that scale with colony wealth, sieges, sappers, winter wolf packs, and auto-pause on danger. An animal that starts chasing a colonist pauses the game with a red warning, a marker and a chase line, and the camera turns to it if it's off-screen.
- Blueprint inspector: click a blueprint or frame to see its status, materials delivered, on the way and in stock, work left, haulers and builder; cancelling refunds delivered materials. Blocked or unreachable work is put off so colonists move on to something else, and the colonist inspector shows why.
- Floors, fences and forbid: floors can go under beds and benches; wood and stone fences and gates keep animals in (tame animals use gates, colonists climb fences slowly); forbid items with the Forbid/Allow tools or F so colonists leave them alone. A door or gate can be placed straight over a wall or fence: the old piece comes down first, then the new one goes up. Doors and gates face along their walls and fences on their own, and painting a wall, floor or piece of furniture in another material changes it in place (the room stays sealed). Walls, doors, fences and gates can't be moved. The Plan tool paints layout marks that vanish once something is built on them.
- Defences and combat: sandbags, spike traps, breaching raiders, rebuilding after breaches, ranged and melee combat with floating damage numbers.
- Electricity, as in RimWorld (with our own numbers): research Electricity, then lay conduits (or hidden ones inside walls) to link wood-fired, solar, wind and geothermal generators (on a steam vent) to batteries and devices; each device connects to a conduit within three tiles. Lamps, heaters and coolers with thermostats, and faster electric stove, tailoring and research benches run on the grid; when it can't keep up, batteries help, then devices are switched off in turn and stay off a while. Colonists flick switches and power toggles (V), the Shift+P power grid overlay shows each grid in its own colour, and solar flares and short circuits keep you honest. Before electricity, wood torch lamps light the rooms.
- Health, modelled closely on RimWorld's: wounds per body part that bleed into blood loss, pain shock and capacity-based downing; tend quality from the doctor's skill, the medicine and the bed; wiki-rate healing and scars; infections and six diseases (flu, plague, malaria, sleeping sickness, gut worms, muscle parasites) racing against immunity; per-patient care levels and drafted emergency tending; kidneys, liver and spine with organ failure; prosthetics and a servo/implant tier; surgery with success chances, organ harvest and transplant, euthanasia and anesthetic; a powered vitals monitor; painkillers, a disease-prevention tablet and overdose; chronic conditions of age (bad back, cataract, dementia, artery blockage and heart attacks, carcinoma); and animals that bleed and get tended too. Colonists in pain, wounded or sick rest in bed until they heal (a per-colonist "Rest when hurt" setting, on by default), and any bed can be marked Medical.
- Blood: hits spray blood (ichor from hive insects), wounded colonists and animals look bloodied until they heal, and the floor keeps splats, drip trails and pools that dry to brown, fade over 7 days and wash off in rain or snow.
- Dirt and cleaning: colonists pick up dirt outdoors (more in rain and field work) and track it into rooms; entrance carpets of straw, wood, leather or cloth catch 30-70% of it; colonists sweep dirt and blood inside the Home area you paint, and dirty rooms lose impressiveness.
- Animals: 61 species (livestock, wild grazers and predators, pets, birds, reptiles, hive insects and a rare legendary stag) that spawn by season and habitat, hunt, flee, gore and kick; hunting, butchering, taming, pens, manhunter packs and hive swarms. Any animal that is attacked fights back or runs (penned ones run within the pen), predators may charge anyone who walks too close, and unarmed colonists run from an animal coming for them. Tame animals eat the nearest food they can (grass, berry bushes, raw food from stockpiles, carcasses), and tame predators hunt wild prey.
- Movement: colonists and tame animals walk through each other instead of queueing; only in a fight do colonists keep off each other's tiles.
- Research: a research bench with seven projects that unlock buildings and work, including Fabrication (components, tool cabinets, hospital beds) and Pharmacology (industrial medicine).
- Weather: rain, storms, snow and blizzards with wind, puddles and snow cover; rain douses outdoor fires, snow chills the air, and colonists get soaked.
- A living world: grass is many small tufts that blend from tile to tile and grows back from shoots after grazing; trees have trunks and canopies that turn see-through when someone walks under them; brambles, moss, desert rosettes, ball cacti, elephant ears, fire lilies and dandelions fill out each biome; everything waves in gusting wind and parts around anyone walking through; trees turn in autumn and stand bare and snowy in winter.
- Sun, moon and shadows: every sprite has a normal map, lit by a sun that rises in the east and sets in the west (lower in winter) and by a cool moon at night; shadows swing and stretch with it, long at dawn and dusk. Colonists and animals cast their own animated silhouettes, sharp at the feet and soft at the tip, plus shadows away from campfires and lamps at night. Lamps are real point lights: their light fades over a few tiles, stays inside their room (or, outdoors, where it can reach), shades everything toward the lamp, and keeps colonists' lamp shadows indoors even by day.
- Roofs: colonists roof closed rooms on their own, or wherever you plan one within 6 tiles of a wall; unroofed rooms lose their heat, roofs keep rain and snow off, and unsupported roofs collapse. Press Shift+R for the roof overlay.
- Death and burial: the dead leave bodies that rot to bones; colonists mourn, dig graves with headstones, bury their own, or cremate on a funeral pyre.
- Sound and options: every species has its own call, attack, hurt and death voice (generated with ElevenLabs, reviewed by ear); code-made sound effects that pan with the camera and fade as you zoom out, ambience for day, night and rain, and composed music for the menu, colony life and raids. Options cover volumes, fullscreen, a 120 fps frame cap on every display (VSync off by default; both can be changed), graphics quality (High, Medium or Low, picked for your computer at the first start, with Shadows, Lighting, Water, Grass and World detail rows you can change), a performance overlay, effects detail, autosave interval, edge scrolling, reduced motion and running in the background (off pauses the colony while the window is away), and are saved between runs. The UI scales sharply with the window (Auto, or 75–200 %) and renders at full resolution on high-density screens.
- Save and load, with an autosave every in-game day (adjustable in Options). Commitment mode keeps one save per colony that the game updates itself: no reloading, Save & Quit, no developer mode.
- A full new-game flow: pick a scenario (Broken Hull, Ember Clan, Gilded Expedition, Bare Hands, or build your own in the Scenario Editor with validation and saved files), a storyteller (Orla Vane, Bram Holloway or Kestrel Quay, each pacing threats differently), and a difficulty (six presets or a custom one with 19 settings).
- A generated world: type a seed and get a rotating hex globe with continents, climate by latitude and altitude, rain shadows, twelve biomes, hills, rivers, lakes, roads and named factions. Pick a landing site from a full tile panel (temperature, rainfall, growing period, stones, water, caves, nearest settlement), choose the starting season, and open the Factions dialog.
- Natural-looking colony maps: a staged, seed-reproducible generator builds each landing site from fields. Mountains lean toward the world's high ground with irregular cliffs and winding, branching caves; stone forms broad regions; rivers meander with banks, fertile floodplains and canyons; coasts are ragged; trees grow in groves with clearings. You also find ore lumps deep in rock, stone debris near cliffs, small ruins to salvage and steam vents that warm the air. Roads follow the world's roads and cross rivers on straight wooden bridges; if a later step breaks a road, it is re-routed. Dark caves grow clusters of blue-grey duskmoss, and lit cave mouths may grow ordinary moss. The landing spot is chosen for open, fertile, reachable ground. Fog of war hides sealed caves and rock until you mine into them, and deep rock overhead is permanent. `go run ./cmd/mapgen -out dir` writes a PNG of every field (elevation, fertility, wetness, stone, caves, water distance, terrain, vegetation, resources with bridges and vaults, landing score, regions, roofs, fog), 12-seed contact sheets (`-grid 12`) and hilliness sweeps (`-sweep hills`); in-game, developer mode's "Map fields" shows the same layers over the live map.
- Bridges: research Bridges, then build wooden decks over water, marsh or mud (never the open sea). A deck makes the cell walkable at full speed and holds light things like wooden walls, doors, fences and furniture, but not stone or steel walls or floors. Spans grow out from the shore. If a bridge is destroyed, whatever it held breaks, items over deep water sink, and anyone on it steps off.
- Sealed vaults: old chambers cut into the mountains, with intact walls and hidden by fog. A colonist who comes close senses something ominous. Break in, and the dormant hive insects inside wake (their number scales with the threat level; none on peaceful settings). The loot is good, and a row of sleeper caskets waits. Opening one opens them all: a grateful sleeper may join you, an armed one may attack, or a casket may hold insects, old remains or nothing.
- Light matters to plants: daylight, lamps, burning campfires and glowing duskmoss light the cells around them. Plants grow in sunlight or right next to a lamp (at any hour), not in the dark under a roof; click a plant to see whether it's growing and when it will be ripe. Moss needs light; duskmoss needs darkness and withers in lamplight or under an opened roof.
- Allowed areas: create up to ten named, coloured areas in the area manager (rename, invert, delete), paint and clear them from Architect → Zone with a tinted overlay, and pick each colonist's area in the Schedule tab or the inspector, and each pet's in the Animals tab (Unrestricted, Home or an area). Pawns only pick work, food, beds, recreation and wander spots inside their area, but may walk through other ground to get there; drafted, fleeing or panicking pawns ignore it.
- Pens: build a pen marker inside fences, gates, walls or rock and the enclosure becomes a pen (an open pen is flagged). Livestock (29 farm and grazing species) can't get through gates or doors on their own: handlers rope them and lead them to a pen that accepts their species and sex. The pen's panel shows grazing against what the herd eats and can auto-cut unwanted plants. Livestock without a pen roam, and after a few days may wander off the map. The Animals tab sets each animal's master, whether it follows when the master is drafted, and its area or pen.
- Home area: it grows around new buildings and zones unless you switch off the Auto toggle; colonists clean, repair and fight fires only inside it (and inside their own area). Build-roof areas keep their roofs rebuilt.
- Snow lies tile by tile (none under roofs), slows walkers and stops grass where it is deep; paint a snow clearing area and colonists shovel it.
- Fire: lightning in storms and bursting animals start fires that spread downwind through grass, plants, wooden buildings, bridges and stored goods, burn anyone caught in them and die out in rain or when nothing is left to burn, leaving ash. Firefighting is on by default for fires in the Home area; draft a colonist and right-click a fire to put it out anywhere.
- Create your characters: generated candidates you can reroll, rename, reorder and swap, with backstories, traits, skills and passions, what each cannot do, health conditions and warnings when the team has no doctor, cook, grower, builder, researcher or fighter. Colonists arrive in drop pods (or on foot, or standing in a ring, by scenario).
- Colonist identity and skills: eleven skills with passions that set work speed and learning, 19 traits, 59 original backstories that can rule out kinds of work, six body builds with new hair and beards, and a Bio tab and a Work tab that hatches the cells a colonist cannot do. Older saves keep their colonists as before.
- Factions: settlers, clans, drifters, freebooters and the hive swarm settle the globe; raids come from a named hostile faction, goodwill moves with raids and drifts back, and a Factions window shows goodwill history. Storyteller threat points follow wealth, colonists, time and an adaptation that eases after losses; heat waves and cold snaps come with the Extreme weather setting.
- Water and landing-site maps: the chosen tile shapes the map with hills, caves, coasts, rivers with fords, lakes, mud, wet ground and dirt tracks, always leaving the start able to walk out. Water is rendered with a ripple shader and sounds; wading soaks colonists and washes their boots, and nothing can be built on water.
- Components and industrial medicine: fabricate components from steel or mine buried machinery, refine herbal medicine, and build tool cabinets that speed benches and hospital beds.
- A familiar colony screen: the map fills the screen, with the stored resources top-left, the colonist bar top-centre (it shrinks and wraps for big colonies), alerts and a letter stack on the right, and time, speed buttons, weather and the temperature under the cursor at the lower-right. Select something and its details sit at the lower-left, with its tabs opening upward and every action as a square button beside it (hotkeys in order; right-click a button for per-colonist choices). The Architect's categories, tool buttons and description box sit in the same place, and the main windows open above the tab strip. Letters stay until you dismiss them (right-click); alerts explain themselves on hover and cycle through what they are about. A learning helper in the top-right teaches the basics as you play, and the whole interface uses an original slate-and-teal palette.
- Real font and minimap: Noto Sans throughout the menus, HUD and inspectors, and a minimap (Shift+M, click to move) on big maps. Map sizes from 200x200 to 500x500, holding about 60 fps or better at 3x speed on a grown 500x500 colony.

The full backlogs are in the [vanilla feature checklist](.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md) and the [UI/UX checklist](.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md).

## Run it

Ready-made builds for Windows, macOS (Apple Silicon and Intel) and Linux are on
the [Releases](https://github.com/arxdsilva/rimworld-go/releases) page: unpack
the archive and run `rimworld-go`, keeping the `assets` folder next to it. The
macOS build is unsigned, so open it the first time with right-click → Open.

**System requirements:** 64-bit Windows, macOS or Linux, 6 GB of RAM or more (8 GB recommended), and a GPU with Metal (macOS), DirectX 11 (Windows) or OpenGL (Linux) drivers.
- The game uses about 1.1 GB of resident memory, and its peak footprint is 2.2 GB on a 250×250 map and 3.4 GB on a grown 500×500 colony.
- At the first start it reads your RAM and cores and picks a graphics tier: High from 12 GB and 6 cores, Medium from 8 GB, Low below that. You can change it in Options › Graphics, where "Detect again" re-applies the tier.
- With less than 6 GB, the main menu warns once. Map sizes whose estimate exceeds 70% of your RAM say so in the new-game wizard; nothing is blocked.
Pushing a `v*` tag builds and publishes a new release
([`release.yml`](.github/workflows/release.yml)).

To build from source you need Go 1.26.5 or later (the version in [`go.mod`](go.mod)).

```sh
go run ./cmd/game   # start the game
make build          # build every command into bin/
make test           # unit tests with the race detector and coverage
make simulate       # headless simulation run for quick checks
```

Developer flags for `cmd/game`: `-scenario <name>` starts a set piece (for example `-scenario grown -width 250 -height 250` is the large-map benchmark colony), `-fpsprobe N` runs the colony for N seconds of real frames at `-capture-speed` and `-capture-zoom` and logs fps, 1% low, update/draw times and the CPU work per frame, and `-cpuprofile`, `-memprofile` and `-trace` write Go profiles. `make fps` is the repeatable probe: the grown 500×500 colony at 3× speed, zoom 0.1 and 0.8, in a floating display-sized window with vsync off (`-vsync=false`), logged to `bin/fps-<tag>.log`; a 30 Hz or occluded display caps the fps number, so compare the `work` line there. `-capture-ui-scale 1.5` (or 2) takes a `-capture` at that UI scale, to review scaled layouts. `-graphics low` (or `medium`, `high`) applies that graphics tier for one run, captures and `-fpsprobe` included, without saving it, and `-render-scale 0.75` (or `0.5`) draws the map at that share of the screen's pixels for the run; the probe's report ends with the Go heap and the art memory (sprites freed, prewarm, tint bakes). `go run ./cmd/benchmark` prints per-tick percentiles for the grown colony at 120, 250 and 500 tiles a side, with a `views` row for the read model. In the game, Options › Graphics › Performance overlay shows fps, 1% low, frame, update and draw time, the sim rate and memory in a small box by the resources.

Sound plays through your default audio device; `go run ./cmd/game -noaudio`
(or `RIMWORLD_GO_NOAUDIO=1`) runs silently. On Linux, audio needs cgo and
the ALSA headers (`sudo apt install libasound2-dev`). Options (volumes,
fullscreen, graphics quality and detail, effects, autosave, edge scrolling,
reduced motion) are saved to
`settings.json` in your user config folder, next to the saves.

### Controls

Every key can be changed in Options › Controls (conflicts ask before
swapping; Reset to defaults puts them back).

| What | Keys |
|---|---|
| Camera | WASD or arrows pan (Shift: faster), PgDn / PgUp or the wheel zoom, middle-drag pans, edge scrolling in Options |
| Time | Space pause, 1 / 2 / 3 speeds, Home / End one speed up / down, or the speed buttons at the lower-right |
| Tabs | Tab Architect, F1 Work, F2 Schedule, F4 Animals, F5 Wildlife, F6 Research (F3, F7-F9 not yet) |
| View | F10 screenshot (saved under `screenshots` next to the settings), F11 hide the interface |
| Selection | click, Shift+click add/remove, drag a box, double-click for all of a kind on screen (in the colonist bar: centre on that colonist), `,` / `.` previous / next colonist, Numpad * next thing in the cell, Z search |
| Selected | R draft / undraft (all selected), F forbid / allow, V power on / off (a colonist flicks it), B H Y N J U M K I L O P the command buttons in order (right-click one for its options) |
| Orders | right-click (drafted: the group spreads around the tile; right-drag lines them up), Shift queues |
| Architect | Q / E rotate, or change the shape of drag tools (line, rectangle, oval, filled or empty); Ctrl squares the shape; C Cancel tool, X Deconstruct tool |
| Overlays | Shift+R roofs, Shift+P power grid, G room stats, Shift+M minimap on maps over 150 tiles, hold Alt for cell details |
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
