# Neat Storage gap analysis (NST)

Scope: the 27 storage furniture pieces of the RimWorld mod "[sbz] Neat Storage" (Steam Workshop 3416243474), built on the mod's "Adaptive Storage" framework, compared with our storage (shelf, small shelf, egg box, bookcases, hopper, stockpile zones). Load it for NST work only. Source (read 2026-11-03): the mod's Workshop description, saved as data in the session scratchpad. It gives the table below (name, max items per cell, size, notes) and says the pieces "work the same way as vanilla shelves" with "a wider variety of items", custom display of stored items and caches. The mod's source is not available. Everything the description does not give is **unknown** here, and the value in the table is **proposed by us**, derived from our own similar furniture (shelf, small shelf, bookcase, hopper, dresser, sculpture). Names stay plain furniture nouns, card text and descriptions are original prose, art is original (never traced).

Columns of the main table: id, piece, cells (w x h, facing south), per-cell capacity (stacks of one kind; see Decisions), research, accepted items (proposed), cost (proposed), beauty and admire (proposed), status, work needed.

## Decisions

| Item | Decision |
|---|---|
| Scope | All 27 pieces are built: new building kinds, costs, art, Architect cards, research gates, item filters, rendering of stored items, a catalog test, and one row each below. Status vocabulary is the Architect doc's: `missing`, `partial`, `done`. |
| Exception (ENG-003 style) | **New exception, user decision (2026-11-03): the sizes and per-cell capacities in the table follow the mod's published table.** Everything else (costs, stuff, beauty, hit points, filters, work) is original tuning. The lead adds "NST-*" to the exception list in the root `AGENTS.md` next to ENG-003 (`...2026-11-02 and 2026-11-03`), adding "the Neat Storage furniture set (NST-*, `.cursor/NEAT_STORAGE_GAP_ANALYSIS.md`)". |
| "Max items per cell" means stacks | Our shelf holds `zone.ShelfStacks` = 3 stacks per cell (`zone/map.go:44`), each up to `item.StackLimit` = 1000, and the mod calls its shelf-like pieces "slightly more per cell than vanilla" at 4 and 6 against vanilla's 3. So the mod's number is read as **stacks per cell**. Corpses, weapons and apparel are one-item stacks, so the corpse dump spot's 12 is 12 bodies a cell. |
| Per-building capacity | Today capacity is per zone **kind** (`zone.Kind.Stacks()`: shelf 3, bookcase 5, others 1). 27 capacities and 12 filters cannot be new zone kinds without a switch per kind across the game. Decision: all 27 lay a `KindShelf` zone carrying a new saved field `Zone.Profile` (type `zone.Profile`, uint8, 0 = plain shelf), which selects slots and the filter ceiling. See Framework. |
| Walkability | None of the 27 has `BlocksMove`, like the shelf and the hopper: haulers step onto a storage cell (`store.Search` `Blocked`, `TileTaken`) and a blocking building would make its cells unreachable. Pallets, crates and cabinets are therefore walk-through, as shelves are. The room cuts no space for them (`room/tally.go:185` `cutsSpace` reads `BlocksMove`). |
| Research | Follows the mod's notes: Safe, Drum, Horizontal weapon rack, Vertical weapon rack need Machining (ours `research.MachineTools`, label "Machine tools"); Textile rack and Meat hook need Smithing (ours `research.Metalworking`, label "Metalworking"). The other 21 need none, as the mod's table says. Our own Shelf and Small shelf stay behind Upholstery (Complex furniture); so an ungated Open shelf makes the Shelf less needed. That is the mod's table and is accepted. Gates are `Unlocks: "build:<name>"` keys on those two projects (`domain/research/research.go:273`, `:286`); unlisted keys are free (`Prog.Locks`). |
| Material | Wood by default; `StoneOK`/`MetalOK` on the furniture that vanilla builds from any stuff; `MetalOnly` (steel) for the Safe, Drum, both metal racks and the Meat hook. All 27 must pass `TestAxContractKinds` (non-empty `Stuffs`, `StuffOK(DefaultStuff)`, not `Placed`). |
| Architect placement | **Decision needed from the lead before wave 1 (recommended A).** Furniture has 33 cards and the smallest grid holds 34 (`ARCHITECT_GAP_ANALYSIS.md` Decisions); `architect_hud_test.go:603` fails any category with more cards than cells. 27 more cannot go there. **A (recommended):** a thirteenth category `storage` ("Storage", after Furniture) holding shelf, small shelf, egg box, the bookcase family (these four leave Furniture) and 27 single cards: 31 cards, fits all grids. Cost: `hudlayout.ArchCatsH` 204 becomes 7 rows = 236 and every small-screen layout test must pass. **B (fallback, no layout change):** seven family cards (`neat_display`, `neat_wardrobe`, `neat_weapons`, `neat_shelves`, `neat_special`, `neat_pallets`, `neat_crates`; re-click cycles the members, `architect.NextInFamily`) in Misc (22 cards + 7 = 29, fits). The doc below assumes A; B changes only the Architect package work. |
| Admiring | The display case and the round pedestal "can be admired like art". We have **no art-viewing joy today**: sculptures give room beauty only (`rest/beauty.go`, `furniture.go:49`), and `recreate.SpecOf` (`recreate/spec.go:42`) lists only games, televisions and the telescope. Decision: a new recreation mode `recreate.Admire` (see Beauty). It also covers the three sculptures at no extra cost (optional, listed as NST-S1). |
| Save | Additive. Kinds are appended before `kindCount`, nothing renumbered. `SaveVersion` 12 to 13 so an older binary refuses a save that holds kinds it cannot read (MAP-0 did the same for v12); no migration step. The new gob field `Zone.Profile` is zero-safe. |
| Not in scope | The mod's companion mods (Fridge, Gravship Storage, Workbench Shelf, Global Settings, Vanilla Stacks) and its Korean text. A fridge-like cold storage is not one of the 27. |

## Counts

| | Count |
|---|---|
| Pieces (rows) | 27 |
| Done / partial | 27 / 0 |
| Missing | 0 |
| Footprints | 12 of 1x1, 12 of 2x1, 3 of 2x2 (48 cells in all) |
| Capacity | 1 stack/cell: 2; 3: 3; 4: 4; 6: 17; 12: 1 |
| Research-gated | 6 (4 Machining, 2 Smithing); 21 free |
| Building kinds now / after | about 158 / about 185 of 256 (uint8 `Kind`; `TestAxContractKinds` checks the last kind still fits) |
| Sprites | 81 world sprites (18 rotatable pieces x 4 facings + 9 fixed) and 27 Architect icons |
| Architect cards | 27 (plan A) |

## Pieces

Costs are in our units (`Def.Wood` = stuff units, `Def.Work` = construct ticks, scaled like the shelf: Shelf = 2 stuff, 50 work for 2x1; Small shelf = 1, 25). Beauty is in points (the table stores thousandths; Shelf 0.5, Dresser 5, End table 3, Small sculpture 50). "Shelf set" is the shelf's ceiling (`zone.Universe(KindShelf)`: everything storable but chunks and corpses). Sets are named by the filter-tree paths in `zone/filter.go:110`. Priority defaults: Preferred for furniture like the shelf; Normal for pallets, crates and the dump spot, so they do not out-rank stockpiles.

| id | Piece (build name) | Cells | Per cell | Research | Accepted items (proposed) | Cost (proposed) | Beauty / admire (proposed) | Status | Where it lives |
|---|---|---|---|---|---|---|---|---|---|
| NST-001 | Display case (`display case`) | 1x1 | 1 | none | Display set: Weapons, Apparel, Items, Manufactured, Raw resources/Metals (no food, corpses, chunks, bulk materials) | 3 stuff (wood, steel or stone), 60 work | +4; admired when it holds a stack | done | domain/building/construction/construction.go:DisplayCase |
| NST-002 | Round pedestal (`round pedestal`) | 1x1 | 1 | none | Display set | 2 stuff (wood or stone), 50 work | +6; admired when it holds a stack | done | domain/building/construction/construction.go:RoundPedestal |
| NST-003 | Small display shelf (`small display shelf`) | 1x1 | 3 | none | Display set | 1 stuff, 30 work | +0.8 | done | domain/building/construction/construction.go:SmallDisplayShelf |
| NST-004 | Display shelf (`display shelf`) | 2x1 | 3 | none | Display set | 2 stuff, 55 work | +1 | done | domain/building/construction/construction.go:DisplayShelf |
| NST-005 | Weapon rack (`weapon rack`) | 2x1 | 3 | none | Weapons (melee, ranged, grenades) | 2 stuff (wood or steel), 55 work | +0.5 | done | domain/building/construction/construction.go:WeaponRack |
| NST-006 | Narrow wardrobe (`narrow wardrobe`) | 1x1 | 4 | none | Apparel, Manufactured/Textiles | 2 stuff, 45 work | +0.5 | done | domain/building/construction/construction.go:NarrowWardrobe |
| NST-007 | Wardrobe (`wardrobe`) | 2x1 | 4 | none | Apparel, Manufactured/Textiles | 3 stuff, 70 work | +1 | done | domain/building/construction/construction.go:Wardrobe |
| NST-008 | Narrow display cabinet (`narrow display cabinet`) | 1x1 | 4 | none | Cabinet set: Display set plus Foods/Meals and Medicine | 2 stuff, 50 work | +1.5 | done | domain/building/construction/construction.go:NarrowDisplayCabinet |
| NST-009 | Display cabinet (`display cabinet`) | 2x1 | 4 | none | Cabinet set | 4 stuff, 90 work | +2 | done | domain/building/construction/construction.go:DisplayCabinet |
| NST-010 | Open shelf (`open shelf`) | 2x1 | 6 | none | Shelf set | 3 stuff, 60 work | +0.5 | done | domain/building/construction/construction.go:OpenShelf |
| NST-011 | 2-tier display shelf (`two tier display shelf`) | 2x1 | 6 | none | Display set | 4 stuff, 80 work | +1 | done | domain/building/construction/construction.go:TwoTierDisplayShelf |
| NST-012 | Narrow tall shelf (`narrow tall shelf`) | 1x1 | 6 | none | Shelf set | 2 stuff, 50 work | +0.5 | done | domain/building/construction/construction.go:NarrowTallShelf |
| NST-013 | Tall shelf (`tall shelf`) | 2x1 | 6 | none | Shelf set | 4 stuff, 95 work | +0.5 | done | domain/building/construction/construction.go:TallShelf |
| NST-014 | Safe (`safe`) | 1x1 | 6 | Machining | Valuables: Raw resources/Metals, Manufactured/Components, Items/Parts, Medicine, Drugs, Prosthetics | 5 steel, 150 work | 0 | done | domain/building/construction/construction.go:Safe |
| NST-015 | Drum (`drum`) | 1x1 | 6 | Machining | Raw resources/Chemicals, Brewing and leaves, Milk, Beer | 3 steel, 80 work | -1 | done | domain/building/construction/construction.go:Drum |
| NST-016 | Horizontal weapon rack (`horizontal weapon rack`) | 2x1 | 6 | Machining | Weapons | 5 steel, 110 work | +0.5 | done | domain/building/construction/construction.go:HorizontalWeaponRack |
| NST-017 | Vertical weapon rack (`vertical weapon rack`) | 2x1 | 6 | Machining | Weapons | 5 steel, 110 work | +0.5 | done | domain/building/construction/construction.go:VerticalWeaponRack |
| NST-018 | Textile rack (`textile rack`) | 1x1 | 6 | Smithing | Manufactured/Textiles, Items/Carpets | 2 stuff (wood or steel), 50 work | +0.5 | done | domain/building/construction/construction.go:TextileRack |
| NST-019 | Meat hook (`meat hook`) | 2x1 | 6 | Smithing | Foods/Raw food/Meat (meat, human meat, insect meat, fish) | 3 steel, 70 work | -2 | done | domain/building/construction/construction.go:MeatHook |
| NST-020 | Mini pallet (`mini pallet`) | 1x1 | 6 | none | Shelf set | 1 wood, 25 work | -0.5 | done | domain/building/construction/construction.go:MiniPallet |
| NST-021 | Half pallet (`half pallet`) | 2x1 | 6 | none | Shelf set | 2 wood, 45 work | -0.5 | done | domain/building/construction/construction.go:HalfPallet |
| NST-022 | Pallet (`pallet`) | 2x2 | 6 | none | Shelf set | 4 wood, 80 work | -1 | done | domain/building/construction/construction.go:Pallet |
| NST-023 | Small crate (`small crate`) | 1x1 | 6 | none | Shelf set | 2 stuff, 40 work | 0 | done | domain/building/construction/construction.go:SmallCrate |
| NST-024 | Crate (`crate`) | 2x1 | 6 | none | Shelf set | 3 stuff, 60 work | 0 | done | domain/building/construction/construction.go:Crate |
| NST-025 | Large crate (`large crate`) | 2x2 | 6 | none | Shelf set | 6 stuff, 110 work | -0.5 | done | domain/building/construction/construction.go:LargeCrate |
| NST-026 | Tall crate (`tall crate`) | 2x2 | 6 | none | Shelf set | 7 stuff, 120 work | -0.5 | done | domain/building/construction/construction.go:TallCrate |
| NST-027 | Corpse dump spot (`corpse dump spot`) | 1x1 | 12 | none | Corpses (human, animal), Bones, Chunks; rotten allowed (like `zone.NewDump`) | 2 wood, 30 work | -3 | done | domain/building/construction/construction.go:CorpseDumpSpot |

Unknown in the mod, so ours: every cost, stuff, work, beauty and hit-point figure; which item filter each piece uses (the description only says "a wider variety of items"); whether the weapon racks differ in what they take (here they differ in looks only); whether Safe and Drum are locked to certain items; the default priority; whether the display case and pedestal need an item inside to be admired. Hit points follow our small shelf (50) for 1x1 pieces and the bookcase (100) for 2x1 and 2x2 ones; metal pieces use the steel multiplier already in `StuffStats`.

## Proposed card text (original prose)

| id | Architect description |
|---|---|
| NST-001 | A glass-fronted box for one fine thing. Colonists who walk past stop to look at it. |
| NST-002 | A turned stand that holds one piece up for everyone to see; looking at it calms people. |
| NST-003 | A one-tile rack with three slots, for nice things you want in sight. |
| NST-004 | A wide display rack, three slots a tile, kept tidy for goods worth showing. |
| NST-005 | A rack with pegs for three weapons a tile. |
| NST-006 | A slim cupboard with four slots a tile for clothes and cloth. |
| NST-007 | A two-tile cupboard with four slots a tile for clothes and cloth. |
| NST-008 | A slim glass cabinet, four slots a tile, for goods, meals and medicine. |
| NST-009 | A two-tile glass cabinet, four slots a tile, for goods, meals and medicine. |
| NST-010 | Plain open boards, six slots a tile, for anything that is not a corpse or a chunk. |
| NST-011 | Two shelves stacked on one frame: six slots a tile for goods worth showing. |
| NST-012 | A tall, narrow unit with six slots a tile. |
| NST-013 | A tall two-tile unit with six slots a tile. |
| NST-014 | A steel strongbox with six slots, for metals, parts, drugs and medicine. |
| NST-015 | A steel drum with six slots for fuel, chemicals, milk and drink. |
| NST-016 | A steel rack that lays six weapons a tile flat. |
| NST-017 | A steel rack that stands six weapons a tile on end. |
| NST-018 | A frame with six slots a tile for cloth, hides and carpets. |
| NST-019 | Steel hooks with six slots a tile, to keep raw meat off the floor. |
| NST-020 | A tiny board platform with six slots; goods keep off the bare ground. |
| NST-021 | A two-tile board platform with six slots a tile. |
| NST-022 | A big board platform, four tiles with six slots each. |
| NST-023 | A small slatted crate with six slots. |
| NST-024 | A two-tile crate with six slots a tile. |
| NST-025 | A big crate, four tiles with six slots each. |
| NST-026 | A tall crate on a four-tile base, six slots a tile. |
| NST-027 | One tile for up to twelve bodies, bones and chunks, out of the way. |

## Framework behaviours

What the "Adaptive Storage" framework gives, and what is true in our code today. Evidence is file:line at HEAD (2026-11-03).

| # | Mod behaviour | Our evidence | Gap | Work |
|---|---|---|---|---|
| F1 | Works like a vanilla shelf: haulers fill it, a filter and priority per piece, linked groups. | A shelf lays a `KindShelf` zone over its footprint (`game/store/buildings.go:20` `ZoneKind`, `:72` `Lay`); the filter, priority and group are the zone's (`zone/map.go:136`); `Link` joins zones (`zone/group.go`). The storage index is the only way to find storage: `store.Search` walks `c.Stores(priority)` (`store/search.go:36`) and the per-cell stacks come from `haul.Ground` (`haul/ground.go:90`). | `ZoneKind` knows 3 shelf-like kinds; every new building must return its zone. | Extend `ZoneKind`; keep the search untouched. |
| F2 | A piece's own capacity per cell. | `zone.Kind.Stacks()` (`zone/map.go:84`) is by zone kind; 5 call sites read it: `game/tick.go:949`, `game/zones.go:591` and `:599`, `store/search.go:74` and `:75`, `store/buildings.go:149`. | No per-building capacity. | `Zone.Profile` plus `Zone.Slots()`; the five sites read `z.Slots()`. A one-stack piece (display case, pedestal) must still count as a building zone: `onShelf` (`zones.go:591`) uses `Stacks() > 1`, which would stop sheltering a one-slot piece, so it becomes `Kind.Building()`-based. |
| F3 | Wider range of items. | `Universe(KindShelf)` is storable minus chunks and corpses (`zone/filter.go:313`); the filter tree has about 150 kinds in 9 top categories (`:110`). `Zone.Effective()`, `SetAllowed`, `PasteFrom`, `RowsFor` read `Universe(z.Kind)` (`:381`, `:391`, `:422`). | The ceiling is per kind. Narrower sets (wardrobe, weapons, meat) and a wider one for the dump spot (corpses, bones, chunks) are needed. | `zone.UniverseOf(k, p)`; the Universe callers pass the zone's profile; sets are built once in `sets` (`filter.go:265`, already a read-only package var). |
| F4 | Linked settings. | `Buildings.Link` links any two zones of equal kind (`zone.Map.Link`; `store/buildings.go:123`). The inspector offers link only on the plain shelf (`inspector/storage_ui.go:59` `b.Kind != shelfKind`). | A weapon rack could link to a shelf and then paste a wider filter (`PasteFrom` clamps to the target's ceiling, but the group shares one filter). | Link only zones with the same profile; the inspector offers the buttons to every storage building (see UI). |
| F5 | Shelter and no deterioration for stored items. | `game/zones.go:589` `onShelf`; `craft_host.go:249` (`Exposed` false on a shelf); the checklist's "shelves stop deterioration" (PRD-4). | Same, for all 27 including the dump spot. | Part of F2. |
| F6 | Custom display of stored items. | Every ground stack is drawn alone at its cell centre (`render/world/objects.go:220` `drawItemObj`); two or more stacks on one cell lie on top of each other. The bookcase swaps its sprite by fill (`render/world/book_keys.go:19`, sprite metadata `capacity`/`fill`). The drying rack lays items along fixed slats (`objects.go:238` `drawRackLoad`). The plant pot reads a sprite attachment (`objects.go:265`). | No slot layout. | See "Display of stored items". |
| F7 | "Can be admired like art" (display case, pedestal). | No admire anywhere (repository search: only unrelated health tests). Beauty enters the room average (`rest/beauty.go:44` `TileBeauty`). Recreation pieces are `recreate.SpecOf` with modes Toss, Play, Watch, Gaze (`recreate/spec.go:42`). | New. | See "Beauty and admire". |
| F8 | Performance: custom rendering and caches. | Storage lookup is indexed (STO-1: `zone.Stores` by priority, `haul.Ground` per cell; `STO-1-storage-index.md`). The draw is culled to the visible tiles and allocation-free (`render/world/AGENTS.md` Hot paths). | The risk is the number of zones, not their kind: one zone per building, and `Search` visits each zone of a priority that takes the stack, with early exit by box distance (`search.go:45`). | See "Performance". |
| F9 | Stack size per slot. | A slot holds one stack up to `item.StackLimit` = 1000 (`STORAGE_GAP_ANALYSIS.md` S1); weapons, apparel, corpses one item. | A 6-slot cell holds up to 6 000 of a kind. | None; the mod's "items" are stacks (Decisions). |
| F10 | Corpse dump spot. | Corpses are stored today in a painted dumping stockpile (`zone.KindDump`, `zone/defaults.go:19` `NewDump`: human and animal corpses, bones, chunks; one stack per tile: `STORAGE_GAP_ANALYSIS.md` D10), in a grave or sarcophagus, or in a plain stockpile; a shelf refuses corpses and chunks. A corpse stack is a single item. Rot is the stack's `Rot` against `Shelf()` (`item.go:743`), unchanged by storage. | A building-based dump with 12 stacks a tile. | Profile `Dump`: ceiling corpses plus bones plus chunks, Special unset (rotten allowed, `NewDump`), Normal priority, 12 slots. |
| F11 | Rotation and footprint. | A 2x1 shelf exists: `Shelf` `Size {2,1}`, `Rotatable` (`construction.go:636`). 2x2 exists (`DoubleBed`, `Table2x2`). `Footprint` rotates the footprint with the facing, origin at the top-left of the rotated shape (`STK-BN` handoff); `Rotatable` false means always south (`SmallFootprint`, `furniture.go:142`). | None. | Rows use `Size` and `Rotatable`; the three 2x2s are not rotatable. |

### Display of stored items

- **Today:** a stored stack is a plain ground `ItemView` at the cell. Three stacks on a shelf cell overlap. There is no slot or layout in the read model.
- **Proposed read model (lead, wave 0):** `game.ItemView` gains `Store uint8` (0 loose; otherwise the stored-in building's `construction.Kind`, as a byte) and `Slot uint8` (this stack's index among the cell's stacks, 0 based, in stack id order). The game fills them where the item views are built, in the one pass that builds `WorldView.Items`, with a zone lookup per item (`zones.At`, O(1)) and a counter per cell kept from the previous item of the same cell (items arrive in id order; no map, no global scan). They are not in the saved state.
- **Draw:** `render/world.Objects.drawItemObj` checks `it.Store != 0` and places the stack at the building sprite's attachment `slot<N>` (canvas px, as the plant pot's `plant`, `objects.go:265`), scaled by `Meta.SlotScale`. Attachments are given for the first cell of the footprint only; the renderer adds the item's cell offset to its position (all cells of a piece share one layout). Per-cell layouts: 1 slot: centred; 3: a row on the shelf boards; 4: two by two; 6: two rows of three (shelves, cabinets) or a 3x2 heap (pallets and crates); 12: a heap of up to 4 drawn bodies plus the stack-count label, because 12 sprites overlap into noise.
- **Order:** the item draws after its building sprite and before the next row's objects, in the existing draw order (objects by footprint bottom row). A shelf, rack or cabinet front is lower than the items on it, so the sprite is drawn first and the items on top. A closed Safe, Drum and any crate draw no stored items (only the fill state: the cell label count shows in the inspector). The display case and cabinets are glass: items drawn through them.
- **Count labels:** the existing stack-count label (`render/world` `Labels`) is drawn per stack; at 6 slots it is hidden below the closest zoom tier. Not new: `Labels` must be checked for the item's `Store` field so it does not stack labels on a heap.
- **Art contract:** `assets/metadata/furniture/<slug>[_<facing>].yaml` is generated and gains `attachments: {slot0: [x, y], ...}` (cell 0 only; `slot_scale: <f>` as a new top-level key read by `art.Meta`). `capacity` (already used by the bookcase) is written for tooling. No fill variants: the items are the fill.
- **Plants and grass:** none of this touches plants; the no-outline rule is for them (and must not be broken).

### Beauty and admire

- **Beauty:** `construction.furniture[kind].Beauty` (`furniture.go:33`) has one row per kind, in thousandths of a point, scaled by quality and stuff by `room.BuildingBeauty` (`room/tally.go:174`). The table's beauty column is in points. Pieces are built (not crafted), so quality is the default.
- **Admire (proposed, ours):** `recreate.Admire` is a fifth `Mode` beside Toss, Play, Watch and Gaze (`recreate/spec.go:11`), reached by `SpecOf` for `DisplayCase` and `RoundPedestal` (and, optional NST-S1, `SmallSculpture`, `LargeSculpture`, `GrandSculpture`): Joy `JoySolitary` (reusing the existing kind avoids growing `recreation.NumJoyKinds`, the saved tolerance array in `recreation/tolerance.go:29`; if a distinct kind is wanted later it is an appended, saved change), Power 700 permille, Users 1, Quality false, Learns false (art does not teach), Verb "admire". The colonist stands on a free cell next to the piece and looks at it (the `Play` cell search in `recreate/seek.go`), and only if the piece holds at least one stack. It runs as `job.Recreate` (`JobOf`, `Uses` already cover it) so no new job kind is saved. Joy gain is the existing `recreation.Gain` per `GainPeriod`; the hysteresis is the recreation module's (start below `Entertained`, linger `WorkLeft`), so the no-flap test is `recreate_test.go`'s with an Admire case.
- **Unknown in the mod:** whether the item inside changes how admired it is, the number of viewers and the joy value. We propose none of those beyond the fixed power above.

### Performance

- Storage lookup stays the storage index: `zone.Stores(priority)` and `haul.Ground`. No code added for these pieces scans the map, the item list or all buildings per tick. The 5 `Stacks()` sites become `z.Slots()` (a table lookup).
- Zones grow with buildings: 300 crates are 300 zones (the author's own pain). `Search` is bounded by priority, the filter test `z.AllowsStack` first, and `z.Box.Dist` pruning once a candidate exists (`search.go:45`); a many-zone benchmark is a work item (below). The recommended default priorities keep pallets and crates at Normal, so a map of crates and a few stockpiles do not all compete at Preferred.
- The renderer reads `ItemView.Store`/`Slot`; one attachment lookup per drawn stored item, culled to the view (`Objects` runs per frame over visible tiles). No per-frame allocation: slot offsets are copied once into the sprite's `Meta` when the catalog loads.
- `ItemView` building: one extra field copy per item and one zone lookup; guarded by the existing `BenchmarkDrawArt500` A/B rule (`ebiten/AGENTS.md` Benchmarks).
- Budget: pieces are not rendered with custom shaders; the only shared hot path touched is `drawItemObj`.

## Save, kinds and determinism

| Question | Answer (checked) |
|---|---|
| How is `Kind` enumerated? | `type Kind uint8` with `iota` and a trailing `kindCount` in `construction.go:12`/`:315` (about 158 kinds). Adding 27 gives about 185, below 256. `TestAxContractKinds` (`ax_contract_test.go:18`) fails if `kindCount-1` is not `Valid` or `last+1` is. |
| Where must a new kind be listed? | `defs [kindCount]Def` (`construction.go:429`, a contiguous table: `TestAxContractKinds` demands `d.Kind == Kind(len(names)-1)`, so append defs in the enum order); `furniture [kindCount]` (`furniture.go:33`); `axMaxHP` (`ax_hp.go`); `Movable` (default true: nothing to add); `Stuffs`/`StuffOK` through the Def flags. `plannedA*` hooks are not needed: leave them out so the kinds are live from the first commit (`Planned()` is false). |
| Name rules | `defs[k].Name` is the build name, unique (`TestAxContractKinds`), and the research key and the Architect `Build` field. No collision: `crate`, `pallet`, `safe`, `drum`, `wardrobe` are free; item "crated building" is another table. |
| `ax_contract_test` | New kinds are `> MultiAnalyzer`, so `ax` is true: they need a non-empty `Stuffs`, a valid `DefaultStuff`, not `Placed`, `Unbuildable` false. Extra costs must be `(Item==0)==(N==0)`. Nothing else changes; the file needs no edit unless the lead wants a dedicated `TestNeatStorageKinds` in a new file. |
| Do saves hold `Kind` by number? | Yes (building records carry the byte). Appended numbers do not shift any existing kind. |
| Golden and state hashes | Adding kinds at the end does not change a hash. `Zone.Profile` zero is skipped by the state hash (`game/zones.go:112` note: "zero values are skipped by the state hash"). The grown golden and any scenario that builds shelves are unchanged because the scenarios place none of the new kinds. What does move: `internal/application/game/testdata/save_schema.txt` (the saved-struct field list gains `Zone.Profile`; `save_schema_test.go` regenerates with its update flag), and `SaveVersion` 12 to 13 (`TestSupportedSaveVersions` expects 12; update to 13). The lead re-pins; agents never touch the golden. Do not renumber, do not insert kinds before `FishTank`. |
| Research hash | `Unlocks` key lists are definitions, not state. No migration. |
| An old save | Loads unchanged: no new kinds in it, zones have `Profile` 0. |

## Package budgets (`internal/arch/solid_budget.txt`, `SOLID_GO.md` §8, §10)

Limits: 40 methods per type, 25 fields per type with behaviour, 8 methods per interface, 12 000 lines and 60 files per package (non-test counted). Measured 2026-11-03.

| Package | Non-test lines / files | State | Consequence |
|---|---|---|---|
| `application/game` | 53 224 / 176 | **At its ceiling** (budget lines `files ... 176`, `lines ... 53224`; the type `Game` is also over 40 methods and 25 fields, budgeted at 1837 and 64) | No new files and no net new lines: every edit here is a replacement (the 5 `Stacks()` sites, `ItemView`, the `ZoneView` field). New logic goes to `game/store` (701 lines, 7 files), `game/recreate` (696, 6) and `domain/building/zone` as sub-packages behind their consumer-owned hosts (§10). If a net `+N` lines is unavoidable the agent deletes N dead lines elsewhere in the same change and says so. No new `Game` method (1837 is the ceiling): use functions or the sub-packages' own types. |
| `domain/building/construction` | 2 208 / 13 | Far below. `Kind` has 17 methods. `Def` has 28 fields but no methods, so it is not a "type with behaviour" (the 25-field rule). Do not give `Def` a method, and add no field. | About 200 new lines in `neat.go` and the rows. |
| `domain/building/zone` | 1 838 / 7 | Below. `zone.Map` has 36 methods (ceiling 40), `Zone` 5, `Kind` 5 | New methods go on `Zone`, `Profile` and package functions, not on `Map`. A new file `profile.go` (about 150 lines). |
| `adapters/inbound/ebiten/inspector` | 10 206 / 47 | Close to the 12 000-line ceiling (1 794 lines left) | Keep the inspector change under about 80 lines; reuse the shelf's storage button and window. |
| `adapters/inbound/ebiten` (root) | 10 862 | Near the ceiling | The root is not touched. `App` (57 fields, 244 methods) and `Renderer` (71 methods, 26 fields) are budgeted: add nothing. |
| `.../architect` | 4 401 / 18 | Well below | 27 cards are about 60 lines of `catalog.go`. |
| `.../render/world` | 3 564 / 22 | Well below | Keys and slot layout in a new file `neat_keys.go`. |
| `domain/research` | 738 | Well below | A few `Unlocks` entries. |

## Tests the work must add

| Test | Where | Checks |
|---|---|---|
| `TestNeatStorageKinds` | `construction/neat_test.go` | All 27 kinds exist in the doc's order, sizes and `Rotatable` match the table, none `BlocksMove`, each passes the AX contract, hit points set, beauty set, names unique, `kindCount` below 256. |
| `TestProfileTable` | `zone/profile_test.go` | Slots per profile equal the table; each Universe is a subset of `Storable`; the dump spot's contains corpses and chunks and no profile of the others does; `Slots()` of a plain shelf is 3 (unchanged). |
| `TestNoFlapStorage` | `game/store` or `game/haul` | A re-haul between two pieces of different priority does not bounce a stack (the 1 h minimum stay holds). Also required by the no-flap rule for the new Admire loop. |
| `TestNeatStorageHauling` | `game` journey in the game package tests (not a new file in `game`: tests are not counted, but follow `store_test.go`) | Each of the 27 accepts its set and refuses the rest; a stack fills to its slots and the next goes elsewhere; the dump spot takes 12 corpses and a 13th spills. |
| `TestNeatStorageRoundTrip` | `game/save_test.go` style | Build 27 pieces, fill them, save and load: same stacks, same `Zone.Profile`; a v12 save loads. |
| `TestEveryNeatStorageRowHasACard` | `architect/neat_test.go` | Parses this file like `TestEveryGapRowHasACard` (`catalog_test.go:246`): column 1 is `NST-nnn`, column 2 ends with the build name in backticks; the card exists with that `Build`, in category `storage`, status `done`/`partial` means not `Planned`; the count table equals 27; every card with a neat build name has a row. |
| `TestNeatStorageResearchGates` | `research/neat_test.go` | The 4 Machining and 2 Smithing keys are unlocked by `MachineTools` and `Metalworking`; the other 21 by nothing. |
| `TestNeatArtCoverage` | `render/world` and `art-check` | Every piece has a sprite in each facing it declares, `slot0..slotN-1` in its metadata (N = per-cell capacity; 12 corpse slots become the heap points), an icon, and `make art-check` reports no orphan. |
| `TestJourneyNeatStorage` | `ebiten/journey_neat_test.go` | Open the Architect's Storage category, arm a crate, place it, a hauler fills it, the inspector shows `n/m stacks`, the Storage button opens the filter window. Plus a journey for Admire: a colonist with low joy admires a stocked display case. |
| Benchmarks | `store` and root | `BenchmarkSearchManyShelves` (300 zones, mixed priorities) must not exceed shelf-only time per search by more than the zone count scales; `BenchmarkDrawArt500` A/B within noise. |

## Checklists and docs to update in the implementing changes

`RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` (storage items, line 513 onwards: add NST), `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` (Architect category), `ART_DIRECTION_AND_ASSET_PIPELINE.md` §13 (storage furniture), `ARCHITECT_GAP_ANALYSIS.md` (Decisions: categories; the four moved cards), `STORAGE_GAP_ANALYSIS.md` (a pointer here), `DOCUMENT_INDEX.md`, root `AGENTS.md` (exception), `architect/AGENTS.md`, a handoff `.cursor/handoffs/NST-*.md` per agent.

## Proposed split

Parallel workers (PARALLEL_WORK §0). Areas: domain, application, renderer/UI adapter, art pipeline, docs. Run `make agent-budget` first; art agents are about 3 GiB each (Blender), so at most the number it prints, started a few minutes apart. Agents run `GOFLAGS=-p=2` and their own targeted tests; only the lead runs the whole ebiten package. Every agent leaves a handoff in `.cursor/handoffs/NST-<id>.md`.

### Wave 0: the lead, alone (the contract)

Lead-only files: `construction.go` enum and a stub `defs` row per kind (zero cost, so the tree compiles), `zone/profile.go` stub, `game.go` (`ItemView`, `ZoneView`), `assets/source/blender/build.py` (`FAMILIES` gains `neatdisplay`, `neatracks`, `neatbulk`), `hudlayout/layout.go` (plan A) or nothing (plan B), `save.go` (`SaveVersion` 13), `internal/application/game/testdata/save_schema.txt`, root `AGENTS.md`, checklists, `DOCUMENT_INDEX.md`, `go.mod` (untouched).

The contract the lead writes:

```go
// construction (append before kindCount, this order)
DisplayCase, RoundPedestal, SmallDisplayShelf, DisplayShelf, WeaponRack,
NarrowWardrobe, Wardrobe, NarrowDisplayCabinet, DisplayCabinet, OpenShelf,
TwoTierDisplayShelf, NarrowTallShelf, TallShelf, Safe, Drum,
HorizontalWeaponRack, VerticalWeaponRack, TextileRack, MeatHook,
MiniPallet, HalfPallet, Pallet, SmallCrate, Crate, LargeCrate, TallCrate,
CorpseDumpSpot
func (k Kind) IsNeatStorage() bool          // neat.go (WP-A)
func NeatProfile(k Kind) (p int, ok bool)   // 1..27 in the order above; WP-A returns it, zone converts
// zone
type Profile uint8                           // 0 = plain shelf; 1..27 as above; append only, saved
type Zone struct { ...; Profile Profile }    // new gob field, zero-safe
func (p Profile) Stacks() int                // per cell: the "Per cell" column
func (z Zone) Slots() int                    // KindShelf: Profile.Stacks() or 3; else Kind.Stacks()
func UniverseOf(k Kind, p Profile) Filter    // Universe(k) when p == 0
// game views
ItemView.Store uint8; ItemView.Slot uint8    // 0 loose
ZoneView.Profile string                       // the building's build name; "" for the rest
// art (sprite keys and metadata)
furniture/<slug>_<south|east|north|west>     // rotatable pieces; furniture/<slug> for fixed ones
ui_icon/<slug>                               // Architect icon, 128x128
attachments: slot0..slot<cap-1>: [x, y]      // canvas px, first cell only; slot_scale: <float>; capacity: <int>
```

Slug is the build name with spaces as underscores (`two_tier_display_shelf`); the 2-tier piece's build name is `two tier display shelf`.

### Wave 1 (parallel, disjoint)

| Id | Model and reason | Job | Owned files | Forbidden | Tests |
|---|---|---|---|---|---|
| WP-A domain/kinds | sonnet: table-driven, mechanical | Real `defs` rows, `furniture` beauty, `axMaxHP`, `neat.go` (`IsNeatStorage`, `NeatProfile`), research `Unlocks` keys (`MachineTools`: safe, drum, horizontal and vertical weapon rack; `Metalworking`: textile rack, meat hook), `neat_test.go` | `domain/building/construction/{neat.go,neat_test.go,furniture.go,ax_hp.go}`, the 27 `defs` rows in `construction.go`, `domain/research/{research.go,neat_test.go}` | all others, `ax_gate.go`, golden files | `go test -race ./internal/domain/building/construction/ ./internal/domain/research/` |
| WP-B storage behaviour | opus: invariants, save and budget | `zone/profile.go` with the table and sets; `UniverseOf` and the 5 `Universe(z.Kind)` callers; `Zone.Slots()`; `Link` refuses unequal profiles; `store.ZoneKind` for the 27 (returns the proto zone with the profile, default priority, filter); replace the 5 `Stacks()` sites; `onShelf` building-based; `ItemView.Store/Slot` fill; `ZoneView.Profile`; the benchmark | `domain/building/zone/{profile.go,filter.go,map.go,defaults.go,profile_test.go}`, `application/game/store/{buildings.go,search.go,store_test.go}`, line-neutral edits in `application/game/{tick.go,zones.go,game.go,placement.go}` | everything else in `game` (file and line ceiling), `save.go`, golden files | `go test -race ./internal/domain/building/zone/ ./internal/application/game/store/ ./internal/application/game/haul/ ./internal/arch/` plus the targeted `game` tests `-run 'Shelf|Stockpile|Haul|Neat'` |
| WP-G admire | sonnet: a small mode | `recreate.Admire`, `SpecOf` entries, seek the adjacent cell with a stocked piece, tests incl. no-flap and the sculptures (S1) | `application/game/recreate/*.go` | `game` package (glue only if `recreate/deps.go` needs a method; report it to the lead) | `go test -race ./internal/application/game/recreate/` |
| WP-D renderer | opus: hot path, A/B gate | `render/world/neat_keys.go` (`NeatKey(kind, facing)` and the slot layout), `build_keys.go` cases, `drawItemObj` for `Store != 0`, `art.Meta` `SlotScale` and `slot<N>` attachments, the 12-corpse heap, `Labels` guard; tests with a fake art and the A/B benchmark | `render/world/{neat_keys.go,build_keys.go,objects.go,neat_test.go}`, `ebiten/art/catalog.go` | architect, inspector, root | `go test -race ./internal/adapters/inbound/ebiten/render/world/ ./internal/adapters/inbound/ebiten/art/` and `BenchmarkDrawArt500` A/B by the lead |
| WP-E1 art: displays | **opus**, art skill, `rimworld-go-art-assets`, ART §12, §13, §4 | Display case, round pedestal, small display shelf, display shelf, 2-tier display shelf, narrow and wide display cabinet (7 pieces: 6 rotatable x 4 facings + the fixed pedestal), icons, slot attachments | `assets/source/blender/artlib/families/neatdisplay.py` | `contracts.py`, `stage.py`, palettes, build.py (lead) | `make art FAMILIES=neatdisplay` then `python3 assets/source/tools/validate_assets.py` |
| WP-E2 art: racks | **opus** | Weapon rack, horizontal, vertical, narrow wardrobe, wardrobe, textile rack, meat hook, safe, drum (9 pieces) | `.../families/neatracks.py` | same | `make art FAMILIES=neatracks` |
| WP-E3 art: bulk | **opus** | Open shelf, narrow tall shelf, tall shelf, mini, half and full pallet, small, normal, large and tall crate, corpse dump spot (11 pieces) | `.../families/neatbulk.py` | same | `make art FAMILIES=neatbulk` |

Art rules for all three: original forms (wood, steel and glass built from `palettes/materials.yaml`, no outline on plants or grass, which these do not contain); built from stuff the way `storage.py` builds the shelf (frame in `wood`, stone and steel by the default stuff colour shift); open fronts so the items drawn over them read; the slot attachments sit on the boards; 4 facings by `S.face` for the rotatable pieces; each piece's icon via `O._icon`. Items are never baked into a sprite. `make art FAMILIES=…` also runs `make normals`, `make atlas` and the validator for the whole tree (`Makefile:78`): two art agents running it at once race on the atlas and generated files, so the lead starts them a few minutes apart, or each agent renders with `blender -b --factory-startup -P assets/source/blender/build.py -- <family>` and the lead runs the tail (`make normals atlas` and `make art-check`) once. Generated files and metadata are never edited by hand.

### Wave 2 (after A and B merge)

| Id | Model | Job | Owned files | Forbidden | Tests |
|---|---|---|---|---|---|
| WP-C Architect and inspector | sonnet: catalog rows and a pane | Plan A: the `storage` category and 27 cards (`Build` = build name, `Icon` = slug, `Desc` from "Proposed card text", rotation hint), move shelf, small shelf, egg box and the bookcase family into it; locks via `Prog.Locks` (already generic, `architect/locks.go:53`); `storageKinds` in the inspector derived from "a zone names this building" (`storage_ui.go:41`) instead of a name list; group buttons for every storage building of equal profile; `Stored/Capacity` already shown (`building_view.go:185`); `ZoneView.Profile` selects the filter window's rows (`zone.RowsFor(k, p)`); tests `TestEveryNeatStorageRowHasACard`, a journey | `adapters/inbound/ebiten/architect/{catalog.go,neat_test.go,AGENTS.md}`, `inspector/{storage_ui.go,cmds_defence.go}` and `ebiten/journey_neat_test.go` | root `App`, `hudlayout` (lead, plan A), renderer | `go test -race ./internal/adapters/inbound/ebiten/architect/ ./internal/adapters/inbound/ebiten/inspector/`; the lead runs the root journeys |

### Integration (the lead)

1. After wave 1: build, `make arch` (`go test ./internal/arch/`), `go test -race` on the touched packages, `make art-check`, re-pin `save_schema.txt` and the goldens only if the diff is exactly `Zone.Profile` and the version.
2. After wave 2: the whole `ebiten` package once with `-race`, the journeys, `BenchmarkDrawArt500` A/B against HEAD (draw-calls/op identical when no neat piece is on screen), `make test` and `staticcheck`.
3. Commit points: wave 0, then one commit per work package, then the lead's pin and docs commit. No branch, no push.
4. Open for the lead to settle: the Architect placement (plan A or B) before wave 0 ends; whether S1 (sculptures admirable) ships with the first change; the `JoySolitary` reuse.
