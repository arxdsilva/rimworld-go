# What works today

The full feature list, grouped by area. Back to the [README](../README.md). The backlogs are in the [vanilla feature checklist](../.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md) and the [UI/UX checklist](../.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md).

- [Building and construction](#building-and-construction)
- [Zones, areas and storage](#zones-areas-and-storage)
- [Hauling and production](#hauling-and-production)
- [Farming and food](#farming-and-food)
- [Power, temperature and weather](#power-temperature-and-weather)
- [Colonists](#colonists)
- [Health, blood and death](#health-blood-and-death)
- [Animals](#animals)
- [Threats and combat](#threats-and-combat)
- [World and maps](#world-and-maps)
- [Research](#research)
- [Game flow, saves and new games](#game-flow-saves-and-new-games)
- [Interface, sound and options](#interface-sound-and-options)

## Building and construction

- **Building:** generated maps, cutting, mining, walls, doors, beds, multi-tile workbenches and a stove, all placed from the Architect menu.
- **Stone:**
  - Each map has two or three of five stones (sandstone, granite, limestone, slate, marble) and steel, silver and gold veins.
  - Mining drops a chunk of the rock's stone, a stonecutter cuts it into blocks, and each stone builds walls with its own strength and build time.
  - Builders can smooth rock walls and stone floors.
  - Walls, doors and rock connect into one continuous surface.
- **Blueprint inspector:**
  - Click a blueprint or frame to see its status, materials delivered, on the way and in stock, work left, haulers and builder; cancelling refunds delivered materials.
  - Blocked or unreachable work is put off so colonists move on to something else, and the colonist inspector shows why.
- **Floors, fences and forbid:**
  - Floors can go under beds and benches.
  - Wood and stone fences and gates keep animals in (tame animals use gates, colonists climb fences slowly).
  - Forbid items with the Forbid/Allow tools or F so colonists leave them alone.
  - A door or gate can be placed straight over a wall or fence: the old piece comes down first, then the new one goes up.
  - Doors and gates face along their walls and fences on their own, and painting a wall, floor or piece of furniture in another material changes it in place (the room stays sealed).
  - Walls, doors, fences and gates can't be moved.
  - The Plan tool paints layout marks that vanish once something is built on them.
- **Bridges:**
  - Research Bridges, then build wooden decks over water, marsh or mud (never the open sea).
  - A deck makes the cell walkable at full speed and holds light things like wooden walls, doors, fences and furniture, but not stone or steel walls or floors.
  - Spans grow out from the shore.
  - If a bridge is destroyed, whatever it held breaks, items over deep water sink, and anyone on it steps off.
- **Roofs:**
  - Colonists roof closed rooms on their own, or wherever you plan one within 6 tiles of a wall.
  - Unroofed rooms lose their heat, roofs keep rain and snow off, and unsupported roofs collapse.
  - Press Shift+R for the roof overlay.

## Zones, areas and storage

- **Zones:**
  - Stockpiles, growing zones and tree zones are painted tile by tile.
  - A tile belongs to one zone only, touching zones of the same kind merge under one outline, and growing zones skip bare rock.
  - The drag preview shows which tiles will be claimed.
  - Click a zone (an empty tile or its label) to manage it, as in RimWorld: change its crop, allow or stop sowing and cutting, Expand or Shrink it with a drag, delete, hide or rename it.
  - A sowing tile is claimed by one grower at a time, and planted tiles show their furrows and crops without the zone's tint over them.
- **Allowed areas:**
  - Create up to ten named, coloured areas in the area manager (rename, invert, delete), paint and clear them from Architect → Zone with a tinted overlay.
  - Pick each colonist's area in the Schedule tab or the inspector, and each pet's in the Animals tab (Unrestricted, Home or an area).
  - Pawns only pick work, food, beds, recreation and wander spots inside their area, but may walk through other ground to get there; drafted, fleeing or panicking pawns ignore it.
- **Home area:**
  - It grows around new buildings and zones unless you switch off the Auto toggle.
  - Colonists clean, repair and fight fires only inside it (and inside their own area).
  - Build-roof areas keep their roofs rebuilt.
- **Storage furniture:** 27 display cases, racks, wardrobes, safes, drums and more, each with its own size, capacity and accepted items, showing what they hold.

## Hauling and production

- **Hauling:**
  - Colonists carry up to 100 at a time.
  - One trip feeds several nearby building sites and takes the leftovers back to the nearest stockpile.
  - Haulers bring in only the colony's own goods (what it makes, brings, harvests, chops, mines or hunts); things found lying on the map, like ruin loot, stone chunks or a predator's kill, stay put until you mark them with Orders → Haul things (or the item's Mark for hauling button).
- **Production:**
  - Workbench bills, a tailoring bench and clothing (parkas, tuques, dusters).
  - Colonists change straight into the best outfit they're comfortable in, even mid-job; a Wear order you give sticks.
- **Components and industrial medicine:** fabricate components from steel or mine buried machinery, refine herbal medicine, and build tool cabinets that speed benches and hospital beds.

## Farming and food

- **Farming and food:**
  - Growing zones (growers first clear whatever else grows on a cell, then sow; ripe crops get a harvest mark), a crop picker, regrowing berries, cotton for cloth, straw from wheat, campfire cooking, drying racks and spoilage.
  - Crops share the grass and bushes' soft, muted look: each planted tile shows a few small plants in rows over tilled furrows, growing smoothly from seed specks to ripe produce.
  - Below a crop's frost line (or in great heat) it takes damage over a few hours: the leaves frost or scorch, then wilt, shrink and thin out until the plants are gone, and a damaged harvest yields less.
- **Light matters to plants:**
  - Daylight, lamps, burning campfires and glowing duskmoss light the cells around them.
  - Plants grow in sunlight or right next to a lamp (at any hour), not in the dark under a roof; click a plant to see whether it's growing and when it will be ripe.
  - Moss needs light; duskmoss needs darkness and withers in lamplight or under an opened roof.

## Power, temperature and weather

- **Electricity,** as in RimWorld (with our own numbers):
  - Research Electricity, then lay conduits (or hidden ones inside walls) to link wood-fired, solar, wind and geothermal generators (on a steam vent) to batteries and devices; each device connects to a conduit within three tiles.
  - Lamps, heaters and coolers with thermostats, and faster electric stove, tailoring and research benches run on the grid.
  - When it can't keep up, batteries help, then devices are switched off in turn and stay off a while.
  - Colonists flick switches and power toggles (V), the Shift+P power grid overlay shows each grid in its own colour, and solar flares and short circuits keep you honest.
  - Before electricity, wood torch lamps light the rooms.
- **Temperature:** seasons, room heat from campfires, and hypothermia on winter nights.
- **Weather:**
  - Rain, storms, snow and blizzards with wind, puddles and snow cover.
  - Rain douses outdoor fires, snow chills the air, and colonists get soaked.
- **Snow:**
  - Snow lies tile by tile (none under roofs), slows walkers and stops grass where it is deep.
  - Paint a snow clearing area and colonists shovel it.
- **Fire:**
  - Lightning in storms and bursting animals start fires that spread downwind through grass, plants, wooden buildings, bridges and stored goods, burn anyone caught in them and die out in rain or when nothing is left to burn, leaving ash.
  - Firefighting is on by default for fires in the Home area; draft a colonist and right-click a fire to put it out anywhere.

## Colonists

- **Create your characters:**
  - Generated candidates you can reroll, rename, reorder and swap, with backstories, traits, skills and passions, what each cannot do, health conditions and warnings when the team has no doctor, cook, grower, builder, researcher or fighter.
  - Colonists arrive in drop pods (or on foot, or standing in a ring, by scenario).
- **Colonist identity and skills:**
  - Eleven skills with passions that set work speed and learning.
  - 41 traits.
  - 59 original backstories that can rule out kinds of work.
  - Six body builds with new hair and beards.
  - A Bio tab and a Work tab that hatches the cells a colonist cannot do.
  - Older saves keep their colonists as before.
- **Colonists:**
  - Needs, mood, rooms and impressiveness, recreation, mental breaks, work priorities, schedules and a per-colonist order queue.
  - Every colonist's route and destination show on the map, and drafted colonists can be sent to a spot or ordered to attack.
- **The colonist card:**
  - Select a colonist or animal and the pane shows a card with needs, skills, health and standing, each with a tooltip, plus selectors for food, area and the current hour.
  - Edit it from its gear: sizes, sections, colours, saved presets; float it over the map or switch it off in Options.
- **Common-sense habits:**
  - Colonists stop recreation once topped off, seek the outdoors when they miss it, carry a spare meal in a pack, tidy up as they go, take what a bill needs in one trip and more.
  - Each habit has its own switch in Options.
- **Movement:** colonists and tame animals walk through each other instead of queueing; only in a fight do colonists keep off each other's tiles.
- **Event logs:** every colonist, raider and animal remembers what happened to it (bites, hits, injuries, meals, sleep, taming, breakdowns), shown in its inspector.
- **Dirt and cleaning:**
  - Colonists pick up dirt outdoors (more in rain and field work) and track it into rooms.
  - Entrance carpets of straw, wood, leather or cloth catch 30-70% of it.
  - Colonists sweep dirt and blood inside the Home area you paint, and dirty rooms lose impressiveness.

## Health, blood and death

- **Health, modelled closely on RimWorld's:**
  - Wounds per body part that bleed into blood loss, pain shock and capacity-based downing.
  - Tend quality from the doctor's skill, the medicine and the bed.
  - Wiki-rate healing and scars.
  - Infections and six diseases (flu, plague, malaria, sleeping sickness, gut worms, muscle parasites) racing against immunity.
  - Per-patient care levels and drafted emergency tending.
  - Kidneys, liver and spine with organ failure.
  - Prosthetics and a servo/implant tier.
  - Surgery with success chances, organ harvest and transplant, euthanasia and anesthetic.
  - A powered vitals monitor.
  - Painkillers, a disease-prevention tablet and overdose.
  - Chronic conditions of age (bad back, cataract, dementia, artery blockage and heart attacks, carcinoma).
  - Animals that bleed and get tended too.
  - Colonists in pain, wounded or sick rest in bed until they heal (a per-colonist "Rest when hurt" setting, on by default), and any bed can be marked Medical.
- **Blood:** hits spray blood (ichor from hive insects), wounded colonists and animals look bloodied until they heal, and the floor keeps splats, drip trails and pools that dry to brown, fade over 7 days and wash off in rain or snow.
- **Death and burial:** the dead leave bodies that rot to bones; colonists mourn, dig graves with headstones, bury their own, or cremate on a funeral pyre.

## Animals

- **Animals:**
  - 61 species (livestock, wild grazers and predators, pets, birds, reptiles, hive insects and a rare legendary stag) that spawn by season and habitat, hunt, flee, gore and kick; hunting, butchering, taming, pens, manhunter packs and hive swarms.
  - Any animal that is attacked fights back or runs (penned ones run within the pen), predators may charge anyone who walks too close, and unarmed colonists run from an animal coming for them.
  - Tame animals eat the nearest food they can (grass, berry bushes, raw food from stockpiles, carcasses), and tame predators hunt wild prey.
- **Pens:**
  - Build a pen marker inside fences, gates, walls or rock and the enclosure becomes a pen (an open pen is flagged).
  - Livestock (29 farm and grazing species) can't get through gates or doors on their own: handlers rope them and lead them to a pen that accepts their species and sex.
  - The pen's panel shows grazing against what the herd eats and can auto-cut unwanted plants.
  - Livestock without a pen roam, and after a few days may wander off the map.
  - The Animals tab sets each animal's master, whether it follows when the master is drafted, and its area or pen.

## Threats and combat

- **Threats:**
  - Raids that scale with colony wealth, sieges, sappers, winter wolf packs, and auto-pause on danger.
  - An animal that starts chasing a colonist pauses the game with a red warning, a marker and a chase line, and the camera turns to it if it's off-screen.
- **Defences and combat:** sandbags, spike traps, breaching raiders, rebuilding after breaches, ranged and melee combat with floating damage numbers.
- **Factions:**
  - Settlers, clans, drifters, freebooters and the hive swarm settle the globe; raids come from a named hostile faction, goodwill moves with raids and drifts back, and a Factions window shows goodwill history.
  - Storyteller threat points follow wealth, colonists, time and an adaptation that eases after losses; heat waves and cold snaps come with the Extreme weather setting.

## World and maps

- **A generated world:**
  - Type a seed and get a rotating hex globe with continents, climate by latitude and altitude, rain shadows, twelve biomes, hills, rivers, lakes, roads and named factions.
  - Pick a landing site from a full tile panel (temperature, rainfall, growing period, stones, water, caves, nearest settlement), choose the starting season, and open the Factions dialog.
- **Natural-looking colony maps:**
  - A staged, seed-reproducible generator builds each landing site from fields.
  - Mountains lean toward the world's high ground with irregular cliffs and winding, branching caves; stone forms broad regions; rivers meander with banks, fertile floodplains and canyons; coasts are ragged; trees grow in groves with clearings.
  - You also find ore lumps deep in rock, stone debris near cliffs, small ruins to salvage and steam vents that warm the air.
  - Roads follow the world's roads and cross rivers on straight wooden bridges; if a later step breaks a road, it is re-routed.
  - Dark caves grow clusters of blue-grey duskmoss, and lit cave mouths may grow ordinary moss.
  - The landing spot is chosen for open, fertile, reachable ground.
  - Fog of war hides sealed caves and rock until you mine into them, and deep rock overhead is permanent.
  - `go run ./cmd/mapgen -out dir` writes a PNG of every field (elevation, fertility, wetness, stone, caves, water distance, terrain, vegetation, resources with bridges and vaults, landing score, regions, roofs, fog), 12-seed contact sheets (`-grid 12`) and hilliness sweeps (`-sweep hills`); in-game, developer mode's "Map fields" shows the same layers over the live map.
- **Water and landing-site maps:**
  - The chosen tile shapes the map with hills, caves, coasts, rivers with fords, lakes, mud, wet ground and dirt tracks, always leaving the start able to walk out.
  - Water is rendered with a ripple shader and sounds; wading soaks colonists and washes their boots, and nothing can be built on water.
- **Sealed vaults:**
  - Old chambers cut into the mountains, with intact walls and hidden by fog.
  - A colonist who comes close senses something ominous.
  - Break in, and the dormant hive insects inside wake (their number scales with the threat level; none on peaceful settings).
  - The loot is good, and a row of sleeper caskets waits.
  - Opening one opens them all: a grateful sleeper may join you, an armed one may attack, or a casket may hold insects, old remains or nothing.
- **A living world:**
  - Grass is many small tufts that blend from tile to tile and grows back from shoots after grazing.
  - Trees have trunks and canopies that turn see-through when someone walks under them.
  - Brambles, moss, desert rosettes, ball cacti, elephant ears, fire lilies and dandelions fill out each biome.
  - Everything waves in gusting wind and parts around anyone walking through; trees turn in autumn and stand bare and snowy in winter.
- **Sun, moon and shadows:**
  - Every sprite has a normal map, lit by a sun that rises in the east and sets in the west (lower in winter) and by a cool moon at night; shadows swing and stretch with it, long at dawn and dusk.
  - Colonists and animals cast their own animated silhouettes, sharp at the feet and soft at the tip, plus shadows away from campfires and lamps at night.
  - Lamps are real point lights: their light fades over a few tiles, stays inside their room (or, outdoors, where it can reach), shades everything toward the lamp, and keeps colonists' lamp shadows indoors even by day.

## Research

- **Research:** a research bench with 75 projects that unlock buildings and work, including Fabrication (components, tool cabinets, hospital beds) and Pharmacology (industrial medicine).

## Game flow, saves and new games

- **Save and load,** with an autosave every in-game day (adjustable in Options). Commitment mode keeps one save per colony that the game updates itself: no reloading, Save & Quit, no developer mode.
- **A full new-game flow:** pick a scenario (Broken Hull, Ember Clan, Gilded Expedition, Bare Hands, or build your own in the Scenario Editor with validation and saved files), a storyteller (Orla Vane, Bram Holloway or Kestrel Quay, each pacing threats differently), and a difficulty (six presets or a custom one with 19 settings).

## Interface, sound and options

- **A familiar colony screen:**
  - The map fills the screen, with the stored resources top-left, the colonist bar top-centre (it shrinks and wraps for big colonies), alerts and a letter stack on the right, and time, speed buttons, weather and the temperature under the cursor at the lower-right.
  - Select something and its details sit at the lower-left, with its tabs opening upward and every action as a square button beside it (hotkeys in order; right-click a button for per-colonist choices).
  - The Architect's categories, tool buttons and description box sit in the same place, and the main windows open above the tab strip.
  - Letters stay until you dismiss them (right-click); alerts explain themselves on hover and cycle through what they are about.
  - A learning helper in the top-right teaches the basics as you play, and the whole interface uses an original slate-and-teal palette.
- **Real font and minimap:**
  - Noto Sans throughout the menus, HUD and inspectors, and a minimap (Shift+M, click to move) on big maps.
  - Map sizes from 200x200 to 500x500, holding about 60 fps or better at 3x speed on a grown 500x500 colony.
- **Sound and options:**
  - Every species has its own call, attack, hurt and death voice (generated locally with the open MOSS-SoundEffect model, reviewed by ear).
  - Code-made sound effects that pan with the camera and fade as you zoom out, ambience for day, night and rain, and composed music for the menu, colony life and raids.
  - Options cover volumes, fullscreen, a 120 fps frame cap on every display (VSync off by default; both can be changed), graphics quality (High, Medium or Low, picked for your computer at the first start, with Shadows, Lighting, Water, Grass and World detail rows you can change), a performance overlay, effects detail, autosave interval, edge scrolling, reduced motion and running in the background (off pauses the colony while the window is away), and are saved between runs.
  - The UI scales sharply with the window (Auto, or 75–200 %) and renders at full resolution on high-density screens.
