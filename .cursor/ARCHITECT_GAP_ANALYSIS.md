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
| Done or partial (a working card) | 118 |
| Missing (a placeholder card) | 73 rows on 66 placeholder cards (the large tables, the two bookcases and the five flagstones each share one card) |

## Items

Columns: category id, vanilla item, the card that stands for it (`planned_*` is a placeholder), status, and notes (what a partial lacks, or what blocks a missing item). `TestEveryGapRowHasACard` (`architect/catalog_test.go`) parses this table: every row's card must exist in that category, a `missing` row's card must be a placeholder, and every placeholder must have a row.

### Orders

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `orders` | Cancel | `cancel` | done | |
| `orders` | Deconstruct | `deconstruct` | done | |
| `orders` | Uninstall | `planned_uninstall` | missing | Needs a minified-building item; the building pane's Move mode covers relocating for now. |
| `orders` | Mine | `mine` | done | |
| `orders` | Haul things | `haul` | done | |
| `orders` | Cut plants | `cut` | done | |
| `orders` | Harvest | `harvest` | done | |
| `orders` | Chop wood | `cut` | partial | Cut plants also fells trees; no trees-only order. |
| `orders` | Hunt | `hunt` | done | |
| `orders` | Slaughter | `planned_slaughter` | missing | Needs a slaughter designation and job for tame animals (the animal pane lists it). |
| `orders` | Tame | `tame` | done | |
| `orders` | Claim | `planned_claim` | missing | Needs building ownership by faction. |
| `orders` | Strip | `planned_strip` | missing | Needs a strip job; the allow programme (AL) may cover drafted pick-up only. |
| `orders` | Open | `planned_open` | missing | The casket pane opens one casket; no area order. |
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
| `zones` | Delete zone | `planned_delete_zone` | missing | Needs an any-zone cell-removal command; Cancel and a zone's Shrink cover part of it. |
| `zones` | Expand home area | `home` | done | |
| `zones` | Clear home area | `unhome` | done | |
| `zones` | Build roof area | `roof` | done | |
| `zones` | Clear build roof area | `planned_clear_roof_area` | missing | Needs a clear mode on the roof-area command. |
| `zones` | Remove roof area | `noroof` | done | |
| `zones` | Clear remove roof area | `planned_clear_noroof_area` | missing | Needs a clear mode on the remove-roof command. |
| `zones` | Expand snow clear area | `snow_clear` | done | |
| `zones` | Clear snow clear area | `unsnow_clear` | done | |
| `zones` | Expand allowed area | `allowed_area` | done | |
| `zones` | Clear allowed area | `allowed_area_clear` | done | |
| `zones` | Manage allowed areas | `allowed_area` | done | Through the area submenu of the Expand card. |

### Structure

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `structure` | Wall | `wall` | done | Wood, stone blocks, steel; no plasteel, silver, gold, jade or uranium walls. |
| `structure` | Door | `door` | done | |
| `structure` | Autodoor | `autodoor` | done | Our "Powered door". |
| `structure` | Ornate door | `planned_ornate_door` | missing | 1.5 core, research Complex furniture; needs a 2-wide door kind. |
| `structure` | Column | `planned_column` | missing | Needs a roof-holding, non-walling kind. |
| `structure` | Fence | `fence` | done | |
| `structure` | Fence gate | `fence_gate` | done | |
| `structure` | Animal flap | `planned_animal_flap` | missing | Needs an animal-only door rule. |
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
| `production` | Deep drill | `planned_deep_drill` | missing | Needs deep ore deposits and the ground-penetrating scanner. |
| `production` | Electric crematorium | `planned_crematorium` | missing | Needs a corpse-burning bill on a powered bench; the funeral pyre (ours) burns bodies. |

### Furniture

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `furniture` | Sleeping spot | `spot` | done | |
| `furniture` | Double sleeping spot | `planned_double_sleeping_spot` | missing | Needs a two-sleeper spot kind (the double bed's rules exist). |
| `furniture` | Bedroll | `bedroll` | done | |
| `furniture` | Double bedroll | `double_bedroll` | done | |
| `furniture` | Bed | `bed` | done | |
| `furniture` | Double bed | `double_bed` | done | |
| `furniture` | Hospital bed | `hospital_bed` | done | |
| `furniture` | Animal sleeping spot | `animal_spot` | done | |
| `furniture` | Animal sleeping box | `planned_animal_sleeping_box` | missing | Needs a kind; animal bed rules exist. |
| `furniture` | Animal bed | `animal_bed` | done | |
| `furniture` | End table | `end_table` | done | |
| `furniture` | Dresser | `dresser` | done | |
| `furniture` | Table (1x2) | `table` | partial | Ours is a single-tile table. |
| `furniture` | Table (2x2) | `planned_tables` | missing | Needs multi-tile tables and seat counting. |
| `furniture` | Table (2x4) | `planned_tables` | missing | As above. |
| `furniture` | Table (3x3) | `planned_tables` | missing | As above. |
| `furniture` | Stool | `stool` | done | |
| `furniture` | Dining chair | `chair` | done | |
| `furniture` | Armchair | `armchair` | done | |
| `furniture` | Couch | `planned_couch` | missing | Needs a two-seat kind. |
| `furniture` | Small sculpture | `small_sculpture` | done | |
| `furniture` | Large sculpture | `large_sculpture` | done | |
| `furniture` | Grand sculpture | `grand_sculpture` | done | |
| `furniture` | Plant pot | `planned_plant_pot` | missing | Needs a decorative plant container. |
| `furniture` | Bookcase | `planned_bookcase` | missing | Needs books (vanilla 1.5 reading). |
| `furniture` | Small bookcase | `planned_bookcase` | missing | As above. |
| `furniture` | Shelf | `shelf` | done | |
| `furniture` | Small shelf | `planned_small_shelf` | missing | Needs a 1x1 shelf kind; shelf storage rules exist. |
| `furniture` | Torch lamp | `torch_lamp` | done | |
| `furniture` | Wall torch lamp | `planned_wall_torch_lamp` | missing | Needs wall-mounted placement. |
| `furniture` | Wall lamp | `planned_wall_lamp` | missing | As above. |
| `furniture` | Standing lamp | `lamp` | done | Our "Lamp". |
| `furniture` | Sun lamp | `sun_lamp` | done | Card added by STK-AG (the kind existed since FOOD-2). |
| `furniture` | Flood light | `planned_flood_light` | missing | Needs a wide-radius light kind. |
| `furniture` | Urn | `planned_urn` | missing | Not craftable in vanilla; needs a trade or map-gen source. |

### Power

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `power` | Power conduit | `conduit` | done | |
| `power` | Hidden conduit | `hidden_conduit` | done | |
| `power` | Waterproof conduit | `planned_waterproof_conduit` | missing | Needs conduits over shallow water. |
| `power` | Power switch | `power_switch` | done | |
| `power` | Wood-fired generator | `wood_generator` | done | |
| `power` | Chemfuel powered generator | `planned_chemfuel_generator` | missing | Needs chemfuel as a burnable fuel. |
| `power` | Wind turbine | `wind_turbine` | done | |
| `power` | Solar generator | `solar_panel` | done | |
| `power` | Geothermal generator | `geothermal` | done | |
| `power` | Watermill generator | `planned_watermill` | missing | Needs rivers with flow direction. |
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
| `security` | IED firefoam trap | `planned_ied_firefoam` | missing | Needs firefoam (fire suppression foam). |
| `security` | IED antigrain warhead trap | `planned_ied_antigrain` | missing | Needs the antigrain warhead item. |
| `security` | Mini-turret | `sentry_turret` | done | Our "Sentry turret". |
| `security` | Autocannon turret | `autocannon` | done | |
| `security` | Uranium slug turret | `slug_turret` | done | |
| `security` | Foam turret | `planned_foam_turret` | missing | Needs firefoam. |
| `security` | Rocketswarm launcher | `planned_rocketswarm` | missing | Needs a rocket projectile and its ammunition. |
| `security` | Mortar | `mortar` | done | |

### Misc

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `misc` | Caravan hitching spot | `planned_hitching_spot` | missing | Needs caravan-forming wait spots. |
| `misc` | Egg box | `planned_egg_box` | missing | Needs a laying-target container for animals. |
| `misc` | Grave | `grave` | done | Moved from Furniture. |
| `misc` | Sarcophagus | `sarcophagus` | done | Moved from Furniture. |
| `misc` | Marriage spot | `marriage_spot` | done | |
| `misc` | Party spot | `party_spot` | done | |
| `misc` | Pen marker | `pen_marker` | done | |
| `misc` | Tool cabinet | `tool_cabinet` | done | Moved from Production. |
| `misc` | Comms console | `radio_console` | done | Our "Radio console". |
| `misc` | Orbital trade beacon | `trade_beacon` | done | Our "Trade beacon". |
| `misc` | Cryptosleep casket | `planned_cryptosleep_casket` | missing | The sealed vault casket kind exists (not buildable); needs a cost, research and an enter job. |
| `misc` | Firefoam popper | `planned_firefoam_popper` | missing | Needs firefoam. |
| `misc` | Ground-penetrating scanner | `planned_deep_scanner` | missing | Needs deep ore deposits. |
| `misc` | Long-range mineral scanner | `planned_mineral_scanner` | missing | Needs world-map mining sites. |
| `misc` | Moisture pump | `planned_moisture_pump` | missing | Needs terrain drying over time. |
| `misc` | Multi-analyzer | `multi_analyzer` | done | Moved from Production. |
| `misc` | Pod launcher | `planned_pod_launcher` | missing | Needs transport pod travel on the world map. |
| `misc` | Transport pod | `planned_transport_pod` | missing | As above. |
| `misc` | Vitals monitor | `vitals_monitor` | done | Moved from Furniture. |
| `misc` | Large stele | `planned_large_stele` | missing | Needs a kind; beauty rules exist. |
| `misc` | Grand stele | `planned_grand_stele` | missing | As above. |

### Floors

| Category | Vanilla item | Card | Status | Notes |
|---|---|---|---|---|
| `floors` | Wood floor | `floor` | done | Moved from Structure. |
| `floors` | Straw matting | `planned_straw_matting` | missing | Needs floor kinds per material beyond wood and stone blocks. |
| `floors` | Concrete | `planned_concrete` | missing | As above. |
| `floors` | Paved tile | `planned_paved_tile` | missing | As above. |
| `floors` | Steel tile | `planned_steel_tile` | missing | As above. |
| `floors` | Silver tile | `planned_silver_tile` | missing | As above. |
| `floors` | Gold tile | `planned_gold_tile` | missing | As above. |
| `floors` | Sterile tile | `planned_sterile_tile` | missing | Also needs the infection and cleanliness bonus. |
| `floors` | Carpet | `carpet` | partial | Ours is a one-tile entrance carpet laid from a crafted carpet item, not a colour-picked floor (research Carpet making). Moved from Furniture. |
| `floors` | Sandstone tile | `floor` | partial | The Wood floor card's sandstone blocks material; no separate stone-tile name or look per stone. |
| `floors` | Granite tile | `floor` | partial | As above, granite blocks. |
| `floors` | Limestone tile | `floor` | partial | As above, limestone blocks. |
| `floors` | Slate tile | `floor` | partial | As above, slate blocks. |
| `floors` | Marble tile | `floor` | partial | As above, marble blocks. |
| `floors` | Sandstone flagstone | `planned_flagstone` | missing | Needs a floor laid from stone chunks. |
| `floors` | Granite flagstone | `planned_flagstone` | missing | As above. |
| `floors` | Limestone flagstone | `planned_flagstone` | missing | As above. |
| `floors` | Slate flagstone | `planned_flagstone` | missing | As above. |
| `floors` | Marble flagstone | `planned_flagstone` | missing | As above. |
| `floors` | Smooth surface | `smooth` | done | Our "Smooth stone" (walls and floors). Moved from Orders. |
| `floors` | Remove floor | `planned_remove_floor` | missing | Needs a floor-removal designation (Deconstruct takes our floors down). |

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
- Grouped families (carpet colours, stone tiles, flagstones) want a material or colour picker on one card, like the wall's material row.
