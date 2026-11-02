# Architect gap analysis (STK-AG)

Scope: every Architect category and item in core vanilla RimWorld 1.5 (no Royalty, Ideology, Biotech, Anomaly or Odyssey content), compared with our catalog (`internal/adapters/inbound/ebiten/architect/catalog.go`, placeholders in `planned.go`). Load it for Architect catalog work only. Sources: rimworldwiki.com pages Architect, Menus, Orders, Zone/Area, Structure, Floors, Ship, Campfire, Ornate door and Carpet (2026-11-01). Names and prose here are ours; the wiki only fixes which items exist and where they sit (ENG-003, user decision STK 2026-11-01).

## Decisions

| Item | Decision |
|---|---|
| Categories | The panel shows all twelve vanilla categories, in vanilla order: Orders, Zones, Structure, Production, Furniture, Power, Security, Misc, Floors, Recreation, Ship, Temperature. Six rows of two (`hudlayout.ArchCatsH` = 204). |
| Placeholders | Every missing item is a greyed card in its vanilla category (`architect.Planned`, id `planned_<slug>`, icon `ui_icon/planned`). It cannot be armed, placed or dragged and never emits a command. Its tooltip is the label, one original sentence and "Unavailable: Not in this game yet"; clicking it or pressing its key shows "Not in this game yet". |
| Card order | Within a category, working tools come first and placeholders follow, both in rough vanilla order. A small screen's grid holds fewer cells (34 at 1113x626, 46 at 1280x720), so a greyed card must never push a working tool out of the grid, and working tools keep the first command keys. |
| Moved tools | Heater, cooler, passive cooler, vent and campfire go to Temperature (the wiki's Campfire infobox types it "Building – Temperature"). Wood floor, carpet and Smooth go to Floors. Horseshoe pin goes to Recreation. Grave, sarcophagus, funeral pyre, vitals monitor, tool cabinet and multi-analyzer go to Misc. |
| New working cards | Hydroponics basin (Production) and sun lamp (Furniture) were built kinds with art and panes but no card (FOOD-2/FOOD-5); they get one. |
| One card per family | Vanilla groups carpets by colour, tables by size, and flagstone and stone tile by stone type in drop-down families. One card stands for each family here (also the two bookcases), so Furniture fits the smallest grid (33 cards, 34 cells at 1113x626); every member is still a row below, pointing at that card. |
| Our extras | Haul urgently (Orders), feeding trough (Furniture), drying rack (Production) and funeral pyre (Misc) are ours, not vanilla core; they stay. |
| Excluded | DLC items, burned floors (not buildable), Rearm trap (removed from vanilla in Beta 19), fishing zone (Odyssey). |

## Counts

| | Count |
|---|---|
| Vanilla core items (rows below) | 191 |
| Done or partial (a working card) | 169 |
| Missing (a placeholder card) | 22 rows on 21 placeholder cards (the two bookcases share one card) |

## Items

Columns: category id, vanilla item, the card that stands for it (`planned_*` is a placeholder), status, and notes (what a partial lacks, or what blocks a missing item). `TestEveryGapRowHasACard` (`architect/catalog_test.go`) parses this table: every row's card must exist in that category, a `missing` row's card must be a placeholder, and every placeholder must have a row.

### Orders

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `orders` | Cancel | `cancel` | done | |
| `orders` | Deconstruct | `deconstruct` | done | |
| `orders` | Uninstall | `uninstall` | done | AX-1: a builder packs the building into a crated building (`item.MinifiedBuilding`: stuff, quality and wear kept; bills, fuel and owners are not); the crate's Reinstall gizmo lays a blueprint and a builder carries the crate there and installs it. Work: the deconstruct work to pack, the move order's to install (wiki silent, original tuning). |
| `orders` | Mine | `mine` | done | |
| `orders` | Haul things | `haul` | done | |
| `orders` | Cut plants | `cut` | done | |
| `orders` | Harvest | `harvest` | done | |
| `orders` | Chop wood | `chop` | done | AX-1: trees only (trunked plants, the saguaro included), from 40 % grown (wiki). |
| `orders` | Hunt | `hunt` | done | |
| `orders` | Slaughter | `slaughter` | done | AX-1: marks tame animals only, through the FOOD-3 slaughter job (herd/slaughter.go). |
| `orders` | Tame | `tame` | done | |
| `orders` | Claim | `claim` | done | AX-1: ruins (`Building.Ruin`) and ancient or raider-built pieces (`Building.Ancient`) become the colony's at once, no work (wiki), and join the Home area. |
| `orders` | Strip | `strip` | done | AX-1: marks downed people; a hauler strips each with the SIM-065 strip job. Our corpses already shed what they wore at death (PRD-2), so there is nothing left on a body to strip. |
| `orders` | Open | `open` | done | AX-1: sealed sleeper caskets (the casket pane's open order) and filled graves or sarcophagi (a hauler lifts the body out). The stasis casket ejects through A9's own order. |
| `orders` | Forbid | `forbid` | done | |
| `orders` | Allow | `allow` | done | |
| `orders` | Plan | `plan` | done | |
| `orders` | Remove plans | `unplan` | done | |

### Zones

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `zones` | Stockpile zone | `stockpile` | done | |
| `zones` | Dumping stockpile zone | `dump` | done | Vanilla parity in `STORAGE_GAP_ANALYSIS.md` (ST). |
| `zones` | Growing zone | `grow` | done | |
| `zones` | Delete zone | `delete_zone` | done | AX-1: removes every zone's cells under the drag (stockpiles, growing zones, pens); a zone cut in two splits. |
| `zones` | Expand home area | `home` | done | |
| `zones` | Clear home area | `unhome` | done | |
| `zones` | Build roof area | `roof` | done | |
| `zones` | Clear build roof area | `clear_roof_area` | done | AX-1: erases the build roof area and its plans; roofs already up stay. |
| `zones` | Remove roof area | `noroof` | done | |
| `zones` | Clear remove roof area | `clear_noroof_area` | done | AX-1: erases the remove roof area and its plans, so a closed room there is auto-roofed again. |
| `zones` | Expand snow clear area | `snow_clear` | done | |
| `zones` | Clear snow clear area | `unsnow_clear` | done | |
| `zones` | Expand allowed area | `allowed_area` | done | |
| `zones` | Clear allowed area | `allowed_area_clear` | done | |
| `zones` | Manage allowed areas | `allowed_area` | done | Through the area submenu of the Expand card. |

### Structure

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `structure` | Wall | `wall` | done | Wood, stone blocks, steel, and since AX-2 silver, gold, plasteel, uranium and jade (silver, gold and jade cost ten times the stuff, wiki: Walls). |
| `structure` | Door | `door` | done | |
| `structure` | Autodoor | `autodoor` | done | Our "Powered door". |
| `structure` | Ornate door | `ornate_door` | done | AX-2: a 2-wide door (research Upholstery). Its 50 gold secondary material waits on Def.Extra delivery. |
| `structure` | Column | `column` | done | AX-2: holds a roof like a wall, bounds no room. |
| `structure` | Fence | `fence` | done | |
| `structure` | Fence gate | `fence_gate` | done | |
| `structure` | Animal flap | `animal_flap` | done | AX-2: tame animals pass; people and wild animals don't (path layers). |
| `structure` | Bridge | `bridge` | done | |

### Production

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `production` | Butcher spot | `butcher_spot` | done | |
| `production` | Butcher table | `butcher` | done | |
| `production` | Crafting spot | `crafting_spot` | done | |
| `production` | Fueled stove | `stove` | done | |
| `production` | Electric stove | `electric_stove` | done | |
| `production` | Brewery | `brewing_bench` | done | |
| `production` | Fermenting barrel | `fermenting_barrel` | done | |
| `production` | Art bench | `art_bench` | done | Our "Sculpting bench". |
| `production` | Simple research bench | `research` | done | |
| `production` | Hi-tech research bench | `electric_research` | done | |
| `production` | Fueled smithy | `fueled_smithy` | done | Our "Wood-fired forge". |
| `production` | Electric smithy | `electric_smithy` | done | Our "Electric forge". |
| `production` | Hand tailor bench | `tailor` | done | |
| `production` | Electric tailor bench | `electric_tailor` | done | |
| `production` | Stonecutter's table | `stonecutter` | done | |
| `production` | Drug lab | `drug_table` | done | |
| `production` | Machining table | `machining_table` | done | |
| `production` | Fabrication bench | `fabrication_bench` | done | |
| `production` | Electric smelter | `electric_smelter` | done | |
| `production` | Biofuel refinery | `biofuel_refinery` | done | Our "Fuel refinery". |
| `production` | Nutrient paste dispenser | `nutrient_paste_dispenser` | done | |
| `production` | Hopper | `hopper` | done | |
| `production` | Hydroponics basin | `hydroponics` | done | Card added by STK-AG (the kind existed since FOOD-2). |
| `production` | Deep drill | `deep_drill` | done | AX-4: a miner works it on power; it brings up the deep ore a ground scan found within 2 cells (wiki 14000-work cycle; the portions per cycle are original tuning), then stone chunks once the ore runs out. No insect infestation yet. |
| `production` | Electric crematorium | `crematorium` | done | AX-4: the Cremate bill on power, no wood, 180 work a corpse (wiki). Apparel, weapon and drug burning bills and its heat are not modelled. |

### Furniture

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `furniture` | Sleeping spot | `spot` | done | |
| `furniture` | Double sleeping spot | `double_sleeping_spot` | done | AX-2. |
| `furniture` | Bedroll | `bedroll` | done | |
| `furniture` | Double bedroll | `double_bedroll` | done | |
| `furniture` | Bed | `bed` | done | |
| `furniture` | Double bed | `double_bed` | done | |
| `furniture` | Hospital bed | `hospital_bed` | done | |
| `furniture` | Animal sleeping spot | `animal_spot` | done | |
| `furniture` | Animal sleeping box | `animal_sleeping_box` | done | AX-2. |
| `furniture` | Animal bed | `animal_bed` | done | |
| `furniture` | End table | `end_table` | done | |
| `furniture` | Dresser | `dresser` | done | |
| `furniture` | Table (1x2) | `table` | done | AX-2: older 1x1 tables migrate (save v11); seats along each side cell. |
| `furniture` | Table (2x2) | `table` | done | AX-2: the table card's family (click the card again for the next size). |
| `furniture` | Table (2x4) | `table` | done | As above. |
| `furniture` | Table (3x3) | `table` | done | As above. |
| `furniture` | Stool | `stool` | done | |
| `furniture` | Dining chair | `chair` | done | |
| `furniture` | Armchair | `armchair` | done | |
| `furniture` | Couch | `couch` | done | AX-2: two seats. |
| `furniture` | Small sculpture | `small_sculpture` | done | |
| `furniture` | Large sculpture | `large_sculpture` | done | |
| `furniture` | Grand sculpture | `grand_sculpture` | done | |
| `furniture` | Plant pot | `plant_pot` | done | AX-2: pick a rose or daylily in its pane; a grower sows it, frost kills it and it is sown again; it grows in light and adds beauty. |
| `furniture` | Bookcase | `planned_bookcase` | missing | Needs books (vanilla 1.5 reading). |
| `furniture` | Small bookcase | `planned_bookcase` | missing | As above. |
| `furniture` | Shelf | `shelf` | done | |
| `furniture` | Small shelf | `small_shelf` | done | AX-2. |
| `furniture` | Torch lamp | `torch_lamp` | done | |
| `furniture` | Wall torch lamp | `wall_torch` | done | AX-2 (our "wall torch"): hung on the cell in front of a wall, facing out (it takes that cell's object slot). |
| `furniture` | Wall lamp | `wall_lamp` | done | As above. |
| `furniture` | Standing lamp | `lamp` | done | Our "Lamp". |
| `furniture` | Sun lamp | `sun_lamp` | done | Card added by STK-AG (the kind existed since FOOD-2). |
| `furniture` | Flood light | `flood_light` | done | AX-2. |
| `furniture` | Urn | `urn` | done | AX-2: stands in generated shed ruins (claim to own); the card is locked. No trader stock yet. |

### Power

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `power` | Power conduit | `conduit` | done | |
| `power` | Hidden conduit | `hidden_conduit` | done | |
| `power` | Waterproof conduit | `waterproof_conduit` | done | AX-4: a conduit that also goes over shallow water (never deep water). |
| `power` | Power switch | `power_switch` | done | |
| `power` | Wood-fired generator | `wood_generator` | done | |
| `power` | Chemfuel powered generator | `chemfuel_generator` | done | AX-4: 1000 W, 30 chemfuel a tank, 4.5 a day while running; haulers refuel it with chemfuel. Its explosion when badly damaged is not modelled. |
| `power` | Wind turbine | `wind_turbine` | done | |
| `power` | Solar generator | `solar_panel` | done | |
| `power` | Geothermal generator | `geothermal` | done | |
| `power` | Watermill generator | `watermill` | done | AX-4: map generation keeps the river's flow; the 5x6 mill's wheel row must sit in moving water; 1100 W, 330 W with another mill in its 13x7 exclusion area. Older saves and river-less maps have no flowing water. |
| `power` | Battery | `battery` | done | |

### Security

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `security` | Sandbags | `sandbags` | done | |
| `security` | Barricade | `barricade` | done | |
| `security` | Spike trap | `spike_trap` | done | |
| `security` | IED trap | `ied_blast` | done | Our "Blast charge". |
| `security` | IED incendiary trap | `ied_fire` | done | Our "Fire charge". |
| `security` | IED EMP trap | `ied_pulse` | done | Our "Pulse charge". |
| `security` | IED smoke trap | `ied_smoke` | done | Our "Smoke charge". |
| `security` | IED firefoam trap | `ied_foam` | done | AX-5: our "Foam charge" (2 foam shells, foam burst r 9.9, re-laid after it goes off). |
| `security` | IED antigrain warhead trap | `ied_warhead` | done | AX-5: our "Warhead charge" (antimatter warhead: 550 bomb r 14.9 falling to 110, ~22 % of cells alight). The warhead is found in ancient vaults (2 % of vaults); the wiki's other source, quest rewards, waits for quests. |
| `security` | Mini-turret | `sentry_turret` | done | Our "Sentry turret". |
| `security` | Autocannon turret | `autocannon` | done | |
| `security` | Uranium slug turret | `slug_turret` | done | |
| `security` | Foam turret | `foam_turret` | done | AX-5: 80 W, r 19.9, 41 blobs a spray, 10 charges refilled with chemfuel (7.2 a charge), 3600-tick wait; bursts in foam when smashed. |
| `security` | Rocketswarm launcher | `rocket_battery` | partial | AX-5: our "Rocket battery" (2x2, 150 W, 12 rockets of 24 bomb, r 29.9, full 180-chemfuel load only, 7200-tick recharge). It fires on its own at raiders like the other turrets; vanilla's manual activation by a colonist and per-rocket tile spread are not modelled. |
| `security` | Mortar | `mortar` | done | |

### Misc

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `misc` | Caravan hitching spot | `planned_hitching_spot` | missing | Needs caravan-forming wait spots. |
| `misc` | Egg box | `egg_box` | done | AX-2: tame layers within 20 tiles lay in it; it stores eggs. |
| `misc` | Grave | `grave` | done | Moved from Furniture. |
| `misc` | Sarcophagus | `sarcophagus` | done | Moved from Furniture. |
| `misc` | Marriage spot | `marriage_spot` | done | |
| `misc` | Party spot | `party_spot` | done | |
| `misc` | Pen marker | `pen_marker` | done | |
| `misc` | Tool cabinet | `tool_cabinet` | done | Moved from Production. |
| `misc` | Comms console | `radio_console` | done | Our "Radio console". |
| `misc` | Orbital trade beacon | `trade_beacon` | done | Our "Trade beacon". |
| `misc` | Cryptosleep casket | `planned_cryptosleep_casket` | missing | The sealed vault casket kind exists (not buildable); needs a cost, research and an enter job. |
| `misc` | Firefoam popper | `foam_popper` | done | AX-5: our "Foam popper" (fire within 3.9 tiles, 150-tick spark, foam burst r 9.9; spent, then re-laid as a blueprint, the wiki's rebuild option; no manual trigger). |
| `misc` | Ground-penetrating scanner | `deep_scanner` | done | AX-4: a researcher works it on power; a find every 180000 work on average (360000 at most) lays a lump of deep ore (wiki sizes and yields), shown by the deep ore overlay. |
| `misc` | Long-range mineral scanner | `planned_mineral_scanner` | missing | Needs world-map mining sites. |
| `misc` | Moisture pump | `moisture_pump` | done | AX-4: on power it dries marsh and mud to soil and shallow water to gravel, one cell of radius every 8.7 days up to 6.9; moving water and the sea stay. |
| `misc` | Multi-analyzer | `multi_analyzer` | done | Moved from Production. |
| `misc` | Pod launcher | `planned_pod_launcher` | missing | Needs transport pod travel on the world map. |
| `misc` | Transport pod | `planned_transport_pod` | missing | As above. |
| `misc` | Vitals monitor | `vitals_monitor` | done | Moved from Furniture. |
| `misc` | Large stele | `large_stele` | done | AX-2. |
| `misc` | Grand stele | `grand_stele` | done | AX-2. |

### Floors

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `floors` | Wood floor | `floor` | done | Moved from Structure. |
| `floors` | Straw matting | `straw_matting` | done | 2 hay a tile (1 here), beauty -1, cleanliness -0.1, walk speed 93 % (other floors 100 %, soil 87 %), burns readily. |
| `floors` | Concrete | `concrete` | done | 1 steel, beauty -1 (research Stonecutting). |
| `floors` | Paved tile | `paved_tile` | done | Steel, beauty 0 (research Stonecutting). |
| `floors` | Steel tile | `steel_tile` | done | Steel, cleanliness +0.2 (research Metalworking, as silver and gold tile). |
| `floors` | Silver tile | `silver_tile` | done | Silver, beauty +4, cleanliness +0.2. |
| `floors` | Gold tile | `gold_tile` | done | Gold, beauty +11, cleanliness +0.2. |
| `floors` | Sterile tile | `sterile_tile` | done | Steel and silver, cleanliness +0.6: lowers wound infection in a clean room (x0.3 against x0.5), and raises surgery and research speed through room cleanliness (research Sterile surfaces). |
| `floors` | Carpet | `carpet_floor` | done | A colour-picked floor: 63 colours under our own names, 1 cloth whatever the colour, beauty +2 (research Carpet making); laying another colour over it repaints it, and any floor laid over another replaces it once built. Our one-tile entrance carpet (wipes boots, from a crafted carpet item) stays as its own `carpet` card. |
| `floors` | Sandstone tile | `stone_tile` | done | The Stone tile card's sandstone blocks, with that stone's own sprite. Beauty +1. Wood floors of stone blocks from older saves become stone tiles. |
| `floors` | Granite tile | `stone_tile` | done | The Stone tile card's granite blocks, with that stone's own sprite. Beauty +1. Wood floors of stone blocks from older saves become stone tiles. |
| `floors` | Limestone tile | `stone_tile` | done | The Stone tile card's limestone blocks, with that stone's own sprite. Beauty +1. Wood floors of stone blocks from older saves become stone tiles. |
| `floors` | Slate tile | `stone_tile` | done | The Stone tile card's slate blocks, with that stone's own sprite. Beauty +1. Wood floors of stone blocks from older saves become stone tiles. |
| `floors` | Marble tile | `stone_tile` | done | The Stone tile card's marble blocks, with that stone's own sprite. Beauty +1. Wood floors of stone blocks from older saves become stone tiles. |
| `floors` | Sandstone flagstone | `flagstone` | done | The Flagstone card's sandstone; 4 blocks a tile per the lead's ruling (1 here), beauty 0, no refund. |
| `floors` | Granite flagstone | `flagstone` | done | The Flagstone card's granite; 4 blocks a tile per the lead's ruling (1 here), beauty 0, no refund. |
| `floors` | Limestone flagstone | `flagstone` | done | The Flagstone card's limestone; 4 blocks a tile per the lead's ruling (1 here), beauty 0, no refund. |
| `floors` | Slate flagstone | `flagstone` | done | The Flagstone card's slate; 4 blocks a tile per the lead's ruling (1 here), beauty 0, no refund. |
| `floors` | Marble flagstone | `flagstone` | done | The Flagstone card's marble; 4 blocks a tile per the lead's ruling (1 here), beauty 0, no refund. |
| `floors` | Smooth surface | `smooth` | done | Our "Smooth stone" (walls and floors). Moved from Orders. |
| `floors` | Remove floor | `remove_floor` | done | AX-1: marks floors only; half the material (and of any secondary material) back, nothing for flagstone (wiki, agreed with A3). Deconstruct still takes floors down too (kept for older habits and tests). |

### Recreation

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `recreation` | Horseshoes pin | `horseshoes` | done | Moved from Furniture. |
| `recreation` | Hoopstone ring | `planned_hoopstone` | missing | Needs recreation kinds beyond horseshoes. |
| `recreation` | Game of Ur board | `planned_game_of_ur` | missing | As above. |
| `recreation` | Chess table | `planned_chess_table` | missing | As above; a social game for two. |
| `recreation` | Billiards table | `planned_billiards` | missing | As above. |
| `recreation` | Poker table | `planned_poker_table` | missing | As above. |
| `recreation` | Tube television | `planned_tube_tv` | missing | Needs watched recreation from seats. |
| `recreation` | Flatscreen television | `planned_flatscreen_tv` | missing | As above. |
| `recreation` | Megascreen television | `planned_megascreen_tv` | missing | As above; traded, not researched. |
| `recreation` | Telescope | `planned_telescope` | missing | Needs night-sky recreation. |

### Ship

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `ship` | Ship structural beam | `planned_ship_beam` | missing | Needs the escape-ship victory (MASTER_SPEC endgame). |
| `ship` | Ship cryptosleep casket | `planned_ship_casket` | missing | As above. |
| `ship` | Ship computer core | `planned_ship_core` | missing | As above. |
| `ship` | Ship reactor | `planned_ship_reactor` | missing | As above, plus the reactor's raid countdown. |
| `ship` | Ship engine | `planned_ship_engine` | missing | As above. |
| `ship` | Sensor cluster | `planned_ship_sensor` | missing | As above. |

### Temperature

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `temperature` | Campfire | `campfire` | done | Moved from Furniture. |
| `temperature` | Heater | `heater` | done | Moved from Misc. |
| `temperature` | Cooler | `cooler` | done | Moved from Misc. |
| `temperature` | Passive cooler | `passive_cooler` | done | Moved from Misc. |
| `temperature` | Vent | `vent` | done | Moved from Misc. |

## Follow-ups

- Each missing row becomes real by replacing its placeholder with a working `ToolDef` in `Tools` and flipping the row to done. Its art follows `rimworld-go-art-assets`.
- Grouped families: stone tiles and flagstones have a stone picker on one card, and the carpet floor a colour picker (AX-3); the tables still want one.
