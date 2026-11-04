package architect

import (
	"fmt"
	"sync"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/pkg/mathx"
)

// Architect tool catalog (MAIN_UI_SCREENS §17, UI checklist §5). The
// category panel shows every vanilla Architect category; an item this game
// does not have yet is a greyed placeholder card (planned.go,
// .cursor/ARCHITECT_GAP_ANALYSIS.md). Buildable costs come from the
// simulation's catalog (game.Buildables) so the UI never drifts.

// PlaceMode is how a tool turns a drag into cells.
type PlaceMode uint8

const (
	PlaceSingle PlaceMode = iota // one cell under the cursor
	PlaceLine                    // straight line along the dominant drag axis
	PlaceFill                    // every cell of the dragged rectangle
	PlaceArea                    // one rectangle command (zones, orders)
)

// ToolDef is one Architect tool (a card in the command grid).
type ToolDef struct {
	ID       string
	Label    string
	Category string
	Icon     string // ui_icon/<name>
	Desc     string
	Mode     PlaceMode
	Build    string       // construction kind for buildables
	Cmd      command.Kind // area command for zones/orders
	Crop     string       // crop for growing zones
	Enable   bool         // area command paints (Enabled) rather than clears
	Variant  uint8        // area command's Build (DesignateStockpile: 1 = dumping stockpile)
	Planned  bool         // a vanilla item not in this game yet: a greyed card, never armed (planned.go)
	// Family (A2) is the id of the card a drop-down family shows (the
	// tables by size): only that card is in the grid, standing for the
	// armed member, and clicking it again arms the next member (after the
	// last, none).
	Family string
}

// Category ids beyond the original four rows (STK-AG: the panel shows all
// vanilla categories, storage and Neat Storage in seven rows of two, hudlayout.ArchCatsH).
const (
	CatPower       = "power"
	CatTemperature = "temperature"
	CatFloors      = "floors"
	CatRecreation  = "recreation"
	CatShip        = "ship"
	CatStorage     = "storage" // NST: the 27 Neat Storage pieces, shelves and bookcases
)

// Category is one button of the category panel.
type Category struct{ ID, Label string }

// Categories are the category panel's buttons, in the vanilla order.
var Categories = []Category{
	{"orders", "Orders"},
	{"zones", "Zones"},
	{"structure", "Structure"},
	{"production", "Production"},
	{"furniture", "Furniture"},
	{CatStorage, "Storage"}, // NST
	{CatPower, "Power"},     // PLAY-021
	{"security", "Security"},
	{"misc", "Misc"}, // ENG-003: marriage and party spots
	{CatFloors, "Floors"},
	{CatRecreation, "Recreation"},
	{CatShip, "Ship"},
	{CatTemperature, "Temperature"}, // PLAY-021, TEMP-*
}

// Tools is the tool catalog, in card order.
var Tools = []ToolDef{
	{ID: "harvest", Label: "Harvest", Category: "orders", Icon: "order_harvest", Mode: PlaceArea, Cmd: command.DesignateHarvest,
		Desc: "Mark ripe berry bushes for harvesting."},
	{ID: "deconstruct", Label: "Deconstruct", Category: "orders", Icon: "order_deconstruct", Mode: PlaceArea, Cmd: command.DesignateDeconstruct,
		Desc: "Take buildings down. Refunds half the wood."},
	// AX-1 orders (wiki: Orders).
	{ID: "uninstall", Label: "Uninstall", Category: "orders", Icon: "order_uninstall", Mode: PlaceArea, Cmd: command.DesignateUninstall, Enable: true,
		Desc: "A builder packs each building into a crate that keeps its material, quality and wear. Haul it, sell it, or select the crate and Reinstall it elsewhere."},
	{ID: "cancel", Label: "Cancel", Category: "orders", Icon: "order_cancel", Mode: PlaceArea, Cmd: command.DesignateCancel,
		Desc: "Remove blueprints, zones and orders in an area."},
	{ID: "cut", Label: "Cut plants", Category: "orders", Icon: "order_cut", Mode: PlaceArea, Cmd: command.DesignateCut,
		Desc: "Chop trees for wood or clear bushes. Grass too."},
	{ID: "chop", Label: "Chop wood", Category: "orders", Icon: "order_chop", Mode: PlaceArea, Cmd: command.DesignateChopWood, Enable: true,
		Desc: "Mark trees for felling, and only trees: bushes, crops and grass are left. A tree must be 40% grown; Cut plants takes younger ones for less wood."},
	{ID: "mine", Label: "Mine", Category: "orders", Icon: "order_mine", Mode: PlaceArea, Cmd: command.DesignateMine,
		Desc: "Dig out rock. Each tile drops a stone chunk; ore veins drop metal."},
	{ID: "smooth", Label: "Smooth stone", Category: CatFloors, Icon: "order_smooth", Mode: PlaceArea, Cmd: command.DesignateSmooth,
		Desc: "Smooth rock walls and rough stone ground into clean walls and floors. Not on ore."},
	{ID: "remove_floor", Label: "Remove floor", Category: CatFloors, Icon: "floor_remove", Mode: PlaceArea, Cmd: command.DesignateRemoveFloor, Enable: true,
		Desc: "Take laid floors up, leaving the ground beneath. Half the material comes back; flagstone gives nothing. Smoothed stone is ground and stays."},
	{ID: "plan", Label: "Plan", Category: "orders", Icon: "plan", Mode: PlaceArea, Cmd: command.DesignatePlan, Enable: true,
		Desc: "Sketch a layout: paint placeholder marks. Nobody works on them and they block nothing; a finished building clears its mark."},
	{ID: "unplan", Label: "Remove plan", Category: "orders", Icon: "plan_clear", Mode: PlaceArea, Cmd: command.DesignatePlan,
		Desc: "Erase plan marks in an area."},
	{ID: "forbid", Label: "Forbid", Category: "orders", Icon: "order_forbid", Mode: PlaceArea, Cmd: command.DesignateForbid, Enable: true,
		Desc: "Colonists leave forbidden items alone. Drag over the items."},
	{ID: "allow", Label: "Allow", Category: "orders", Icon: "order_allow", Mode: PlaceArea, Cmd: command.DesignateForbid,
		Desc: "Let colonists use forbidden items again. Drag over the items."},
	{ID: "haul", Label: "Haul things", Category: "orders", Icon: "order_haul", Mode: PlaceArea, Cmd: command.DesignateHaul, Enable: true,
		Desc: "Mark things found on the map for hauling. Colonists haul the colony's own goods by themselves; anything else waits for this mark. Marking also allows forbidden things."},
	{ID: "haul_urgent", Label: "Haul urgently", Category: "orders", Icon: "order_haul", Mode: PlaceArea, Cmd: command.HaulUrgently, Enable: true,
		Desc: "Mark things on the map to be hauled before any other hauling. Drag over the items."},
	{ID: "hunt", Label: "Hunt", Category: "orders", Icon: "order_hunt", Mode: PlaceArea, Cmd: command.DesignateHunt,
		Desc: "Mark animals for hunters. Hunters need a rifle; kills drop meat."},
	{ID: "slaughter", Label: "Slaughter", Category: "orders", Icon: "slaughter", Mode: PlaceArea, Cmd: command.DesignateSlaughter, Enable: true,
		Desc: "Mark tame animals for slaughter. A handler kills each where it stands; the body gives its full meat and leather."},
	{ID: "tame", Label: "Tame", Category: "orders", Icon: "order_tame", Mode: PlaceArea, Cmd: command.DesignateTame,
		Desc: "Mark animals for taming. Hunters coax them; a failed try can enrage the animal. Tame wolves guard the colony."},
	{ID: "claim", Label: "Claim", Category: "orders", Icon: "order_claim", Mode: PlaceArea, Cmd: command.DesignateClaim, Enable: true,
		Desc: "Make abandoned ruins and buildings left by raiders the colony's own, at once. Claimed, they join the Home area and are repaired like any other."},
	{ID: "strip", Label: "Strip", Category: "orders", Icon: "order_strip", Mode: PlaceArea, Cmd: command.DesignateStrip, Enable: true,
		Desc: "Mark downed people: a hauler takes off what they wear and carry. The dead already lie beside what they wore."},
	{ID: "open", Label: "Open", Category: "orders", Icon: "order_open", Mode: PlaceArea, Cmd: command.DesignateOpen, Enable: true,
		Desc: "Mark sealed sleeper caskets and filled graves or sarcophagi. A colonist breaks a casket's seal; a hauler lifts a body out of its grave."},
	{ID: "stockpile", Label: "Stockpile", Category: "zones", Icon: "zone_stockpile", Mode: PlaceArea, Cmd: command.DesignateStockpile,
		Desc: "Storage area. Haulers bring food, wood, stone and goods here, filling higher-priority stockpiles first. Not on deep water, walls or the map edge."},
	{ID: "dump", Label: "Dumping stockpile", Category: "zones", Icon: "zone_dump", Mode: PlaceArea, Cmd: command.DesignateStockpile,
		Variant: game.StockpileVariantDump, Desc: "A stockpile for corpses and stone chunks, rotting ones too, out of the way. Haulers fill it like any stockpile; change what it takes in its storage settings."},
	{ID: "grow", Label: "Growing zone", Category: "zones", Icon: "zone_grow", Mode: PlaceArea, Cmd: command.DesignateGrowZone,
		Crop: "potato", Desc: "Pick a crop or trees below. Colonists sow it and harvest when ripe; crops grow in daylight, faster on rich soil. Soil only: not sand, mud, stone, floors or bridges."},
	{ID: "delete_zone", Label: "Delete zone", Category: "zones", Icon: "zone_delete", Mode: PlaceArea, Cmd: command.DesignateDeleteZone,
		Desc: "Erase the cells of any stockpile, growing zone or pen under the drag. A zone cut in two becomes two zones."},
	{ID: "home", Label: "Expand home area", Category: "zones", Icon: "home", Mode: PlaceArea, Cmd: command.DesignateHome, Enable: true,
		Desc: "Paint the Home area: colonists sweep dirt off these tiles. It grows around new buildings."},
	{ID: "unhome", Label: "Clear home area", Category: "zones", Icon: "home_clear", Mode: PlaceArea, Cmd: command.DesignateHome,
		Desc: "Take tiles out of the Home area so colonists leave them dirty."},
	{ID: "roof", Label: "Build roof area", Category: "zones", Icon: "roof_area", Mode: PlaceArea, Cmd: command.DesignateRoof,
		Desc: "Colonists roof these tiles and rebuild the roof if it falls. A roof needs a wall within 6 tiles; closed rooms are roofed automatically."},
	{ID: "clear_roof_area", Label: "Clear build roof area", Category: "zones", Icon: "roof_area_clear", Mode: PlaceArea, Cmd: command.DesignateClearRoofArea,
		Desc: "Take tiles out of the build roof area. Roofs already up stay up."},
	{ID: "noroof", Label: "Remove roof area", Category: "zones", Icon: "roof_remove", Mode: PlaceArea, Cmd: command.DesignateRemoveRoof,
		Desc: "Take roofs down and keep them off, even in closed rooms."},
	{ID: "clear_noroof_area", Label: "Clear remove roof area", Category: "zones", Icon: "roof_remove_clear", Mode: PlaceArea, Cmd: command.DesignateClearNoRoofArea,
		Desc: "Take tiles out of the remove roof area: a closed room there is roofed again."},
	{ID: "snow_clear", Label: "Snow clearing area", Category: "zones", Icon: "snow_clear", Mode: PlaceArea, Cmd: command.DesignateSnowClear, Enable: true,
		Desc: "Paint where colonists shovel snow away once it lies 15% deep: paths, yards, a landing spot. Cleaning work."},
	{ID: "unsnow_clear", Label: "Clear snow clearing area", Category: "zones", Icon: "snow_clear", Mode: PlaceArea, Cmd: command.DesignateSnowClear,
		Desc: "Take tiles out of the snow clearing area; their snow is left to lie."},
	{ID: "allowed_area", Label: "Expand allowed area >", Category: "zones", Icon: "colonist", Mode: PlaceArea, Cmd: command.PaintArea, Enable: true,
		Desc: "Pick an allowed area, then drag to paint it. Pawns restricted to it keep to it; Manage allowed areas makes, renames and deletes them."},
	{ID: "allowed_area_clear", Label: "Clear allowed area >", Category: "zones", Icon: "home_clear", Mode: PlaceArea, Cmd: command.PaintArea,
		Desc: "Pick an allowed area, then drag to take tiles out of it."},
	{ID: "wall", Label: "Wall", Category: "structure", Icon: "wall", Mode: PlaceLine, Build: "wall",
		Desc: "Blocks movement. Drag to draw a line."},
	{ID: "fence", Label: "Fence", Category: "structure", Icon: "fence", Mode: PlaceLine, Build: "fence",
		Desc: "Animals can't cross; colonists climb over (slow). Drag to draw a line."},
	{ID: "fence_gate", Label: "Fence gate", Category: "structure", Icon: "fence_gate", Mode: PlaceSingle, Build: "fence gate",
		Desc: "Animals can't cross; colonists climb over (slow). A gate lets colonists and tame animals through. Faces along the fence by itself."},
	{ID: "door", Label: "Door", Category: "structure", Icon: "door", Mode: PlaceSingle, Build: "door",
		Desc: "Lets colonists through a wall. Faces along the wall by itself."},
	// AX-2 structure (wiki: Ornate door, Column, Animal flap; AR1's art).
	{ID: "ornate_door", Label: "Ornate door", Category: "structure", Icon: "ornate_door", Mode: PlaceSingle, Build: "ornate door",
		Desc: "A wide double door (2x1) that makes a grand entrance: very beautiful. Faces along its wall by itself; set over a wall, both pieces come down first."},
	{ID: "column", Label: "Column", Category: "structure", Icon: "column", Mode: PlaceSingle, Build: "column",
		Desc: "A stone or wooden pillar that holds a roof up to 6 tiles away, as a wall does, without closing in a room."},
	{ID: "animal_flap", Label: "Animal flap", Category: "structure", Icon: "animal_flap", Mode: PlaceSingle, Build: "animal flap",
		Desc: "A small hatch set in a wall: tame animals walk through it, people and wild animals can't. Keeps a room's warmth like a door."},
	{ID: "floor", Label: "Wood floor", Category: CatFloors, Icon: "floor", Mode: PlaceFill, Build: "floor",
		Desc: "Drag to cover an area. Furniture can stand on it."},
	// AX-3 floors (wiki: Floors).
	{ID: "straw_matting", Label: "Straw matting", Category: CatFloors, Icon: "straw_matting", Mode: PlaceFill, Build: "straw matting",
		Desc: "Woven hay laid on the ground: quick and cheap, a little ugly and quick to catch fire. Drag to cover an area."},
	{ID: "concrete", Label: "Concrete", Category: CatFloors, Icon: "concrete", Mode: PlaceFill, Build: "concrete",
		Desc: "A plain, hard floor of steel-bound stone: quick to pour, a little ugly. Drag to cover an area."},
	{ID: "paved_tile", Label: "Paved tile", Category: CatFloors, Icon: "paved_tile", Mode: PlaceFill, Build: "paved tile",
		Desc: "Hard-wearing pavers for yards, paths and workshops. Drag to cover an area."},
	{ID: "steel_tile", Label: "Steel tile", Category: CatFloors, Icon: "steel_tile", Mode: PlaceFill, Build: "steel tile",
		Desc: "Metal floor plates that keep a room clean. Drag to cover an area."},
	{ID: "silver_tile", Label: "Silver tile", Category: CatFloors, Icon: "silver_tile", Mode: PlaceFill, Build: "silver tile",
		Desc: "A gleaming silver floor: clean and handsome. Drag to cover an area."},
	{ID: "gold_tile", Label: "Gold tile", Category: CatFloors, Icon: "gold_tile", Mode: PlaceFill, Build: "gold tile",
		Desc: "An extravagant gold floor for the finest rooms: clean and very beautiful. Drag to cover an area."},
	{ID: "sterile_tile", Label: "Sterile tile", Category: CatFloors, Icon: "sterile_tile", Mode: PlaceFill, Build: "sterile tile",
		Desc: "A spotless floor of steel and silver. In a clean room it cuts wound infections and helps surgery and research. Drag to cover an area."},
	{ID: "stone_tile", Label: "Stone tile", Category: CatFloors, Icon: "stone_tile", Mode: PlaceFill, Build: "stone tile",
		Desc: "Cut stone tiles, handsome and hard-wearing. Pick the stone below; each has its own look. Drag to cover an area."},
	{ID: "flagstone", Label: "Flagstone", Category: CatFloors, Icon: "flagstone", Mode: PlaceFill, Build: "flagstone",
		Desc: "Rough stone slabs, quick to lay. Pick the stone below. Drag to cover an area."},
	{ID: "carpet_floor", Label: "Carpet floor", Category: CatFloors, Icon: "carpet_floor", Mode: PlaceFill, Build: "carpet floor",
		Desc: "Soft cloth laid wall to wall: beautiful, but it burns. Pick a colour below; every colour costs the same. Drag to cover an area."},
	{ID: "bridge", Label: "Bridge", Category: "structure", Icon: "bridge", Mode: PlaceFill, Build: "bridge",
		Desc: "A wooden deck over water, marsh or mud. Holds light things only: no stone walls or floors. Drag to cover an area."},
	{ID: "bed", Label: "Bed", Category: "furniture", Icon: "bed", Mode: PlaceSingle, Build: "bed",
		Desc: "A proper two-tile bed. Q/E rotates."},
	{ID: "double_bed", Label: "Double bed", Category: "furniture", Icon: "double_bed", Mode: PlaceSingle, Build: "double bed",
		Desc: "A 2x2 bed two colonists share side by side. Q/E rotates."},
	{ID: "shelf", Label: "Shelf", Category: CatStorage, Icon: "shelf", Mode: PlaceSingle, Build: "shelf",
		Desc: "Holds stacks of one filter on the tiles it covers. Set what it accepts from its pane. Q/E rotates."},
	{ID: "sarcophagus", Label: "Sarcophagus", Category: "misc", Icon: "sarcophagus", Mode: PlaceSingle, Build: "sarcophagus",
		Desc: "A grand stone coffin: holds one body, and keeps it from rotting. Set whose it holds from its Storage button. Q/E rotates."},
	{ID: "hospital_bed", Label: "Hospital bed", Category: "furniture", Icon: "hospital_bed", Mode: PlaceSingle, Build: "hospital bed",
		Desc: "A bed for patients: the sick and hurt go here first and are tended better. Q/E rotates."},
	{ID: "vitals_monitor", Label: "Vitals monitor", Category: "misc", Icon: "vitals_monitor", Mode: PlaceSingle, Build: "vitals monitor",
		Desc: "Set it beside a hospital bed: while powered (80 W), the patient there is tended 7% better, operated on 5% more safely and gains immunity 2% faster. One per bed. Q/E rotates."},
	{ID: "spot", Label: "Sleeping spot", Category: "furniture", Icon: "sleeping_spot", Mode: PlaceSingle, Build: "sleeping spot",
		Desc: "Marks a bed-sized place to sleep on the floor. Free and instant; a bed is more comfortable. Q/E rotates."},
	{ID: "bedroll", Label: "Bedroll", Category: "furniture", Icon: "bedroll", Mode: PlaceSingle, Build: "bedroll",
		Desc: "A padded roll sewn from cloth or leather: quick to make, rests nearly as well as a bed. It can be moved. Q/E rotates."},
	{ID: "double_bedroll", Label: "Double bedroll", Category: "furniture", Icon: "double_bedroll", Mode: PlaceSingle, Build: "double bedroll",
		Desc: "A wide bedroll two colonists share, sewn from cloth or leather. It can be moved. Q/E rotates."},
	{ID: "animal_spot", Label: "Animal sleeping spot", Category: "furniture", Icon: "animal_sleeping_spot", Mode: PlaceSingle, Build: "animal sleeping spot",
		Desc: "Marks where a pet sleeps. Free and instant; livestock sleep in their pen."},
	{ID: "animal_bed", Label: "Animal bed", Category: "furniture", Icon: "animal_bed", Mode: PlaceSingle, Build: "animal bed",
		Desc: "A soft basket for a pet, sewn from cloth or leather: it sleeps better than on the floor."},
	{ID: "end_table", Label: "End table", Category: "furniture", Icon: "end_table", Mode: PlaceSingle, Build: "end table",
		Desc: "Set it beside the head of a bed to make that bed more comfortable."},
	{ID: "dresser", Label: "Dresser", Category: "furniture", Icon: "dresser", Mode: PlaceSingle, Build: "dresser",
		Desc: "Makes every bed within 6 tiles in the same room more comfortable. Q/E rotates."},
	{ID: "table", Label: "Table (1x2)", Category: "furniture", Icon: "table_1x2", Mode: PlaceSingle, Build: "table", Family: "table",
		Desc: "A place to eat: it seats 6, a diner at each side cell. Click the card again for the bigger tables. Q/E rotates."},
	{ID: "chair", Label: "Chair", Category: "furniture", Icon: "chair", Mode: PlaceSingle, Build: "chair",
		Desc: "Seat for a table. Q/E rotates."},
	// AX-2 furniture (wiki: Table, Couch, Plant pot, Small shelf, Wall torch lamp, Wall lamp, Flood light, Urn,
	// Double sleeping spot, Animal sleeping box; AR1's art). The bigger tables are the table card's family.
	{ID: "table_2x2", Label: "Table (2x2)", Category: "furniture", Icon: "table_2x2", Mode: PlaceSingle, Build: "table 2x2", Family: "table",
		Desc: "A square table that seats 8, a diner at each side cell. Click the card again for the 2x4 table."},
	{ID: "table_2x4", Label: "Table (2x4)", Category: "furniture", Icon: "table_2x4", Mode: PlaceSingle, Build: "table 2x4", Family: "table",
		Desc: "A long table that seats 12. Q/E rotates. Click the card again for the 3x3 table."},
	{ID: "table_3x3", Label: "Table (3x3)", Category: "furniture", Icon: "table_3x3", Mode: PlaceSingle, Build: "table 3x3", Family: "table",
		Desc: "A big square table that seats 12. Click the card again to put the tables away."},
	{ID: "couch", Label: "Couch", Category: "furniture", Icon: "couch", Mode: PlaceSingle, Build: "couch",
		Desc: "A padded seat for two (2x1), sewn from cloth or leather: the comfiest seat there is. Q/E rotates."},
	{ID: "double_sleeping_spot", Label: "Double sleeping spot", Category: "furniture", Icon: "double_sleeping_spot", Mode: PlaceSingle,
		Build: "double sleeping spot", Desc: "A 2x2 patch of floor two can sleep on side by side. Free and instant; it rests like the bare ground. Q/E rotates."},
	{ID: "animal_sleeping_box", Label: "Animal sleeping box", Category: "furniture", Icon: "animal_sleeping_box", Mode: PlaceSingle,
		Build: "animal sleeping box", Desc: "A small box bed a pet sleeps in: it rests better than on the floor."},
	{ID: "plant_pot", Label: "Plant pot", Category: "furniture", Icon: "plant_pot", Mode: PlaceSingle, Build: "plant pot",
		Desc: "A pot for a decorative plant: select it to sow a rose or a daylily. The plant grows in light, making the room more beautiful as it grows."},
	{ID: "small_shelf", Label: "Small shelf", Category: CatStorage, Icon: "small_shelf", Mode: PlaceSingle, Build: "small shelf",
		Desc: "A one-tile shelf: holds 3 stacks of what you set from its pane. Q/E rotates."},
	// AX-7 (wiki: Bookcase, Small bookcase; AR3's art): one family card, the small one next.
	{ID: "bookcase", Label: "Bookcase", Category: CatStorage, Icon: "bookcase", Mode: PlaceSingle, Build: "bookcase", Family: "bookcase",
		Desc: "Shelves for 10 books. Every book shelved makes reading and research in its room a little better. Q/E rotates. Click the card again for the small bookcase."},
	{ID: "small_bookcase", Label: "Small bookcase", Category: CatStorage, Icon: "small_bookcase", Mode: PlaceSingle, Build: "small bookcase",
		Family: "bookcase", Desc: "A one-tile bookcase for 5 books; they count toward the room's reading bonus. Click the card again to put the bookcases away."},
	{ID: "wall_torch", Label: "Wall torch", Category: "furniture", Icon: "wall_torch", Mode: PlaceSingle, Build: "wall torch",
		Desc: "A torch hung on a wall: set it in front of one, facing out. Burns wood (holds 15, about 2 a day) to light and warm the room."},
	{ID: "wall_lamp", Label: "Wall lamp", Category: "furniture", Icon: "wall_lamp", Mode: PlaceSingle, Build: "wall lamp",
		Desc: "An electric light hung on a wall: set it in front of one, facing out. Lights 8 tiles around while powered. Needs a conduit, generator or battery within 6 tiles."},
	{ID: "flood_light", Label: "Flood light", Category: "furniture", Icon: "flood_light", Mode: PlaceSingle, Build: "flood light",
		Desc: "A bright powered light that lights everything within 17 tiles: yards, fields and killboxes."},
	{ID: "urn", Label: "Urn", Category: "furniture", Icon: "urn", Mode: PlaceSingle, Build: "urn",
		Desc: "A decorative vessel no one builds: it is found standing in old ruins. Claim the ruin to make it the colony's."},
	{ID: "horseshoes", Label: "Horseshoe pin", Category: CatRecreation, Icon: "horseshoe_pin", Mode: PlaceSingle, Build: "horseshoe pin",
		Desc: "A game of horseshoes: dexterity fun thrown from 3 tiles off, up to 8 at once. Trains Shooting."},
	{ID: "ring_toss", Label: "Ring toss", Category: CatRecreation, Icon: "ring_toss", Mode: PlaceSingle, Build: "ring toss",
		Desc: "A peg to toss rings at from 3 tiles off: dexterity fun for up to 3. Trains Shooting."},
	{ID: "ur_board", Label: "Ur board", Category: CatRecreation, Icon: "ur_board", Mode: PlaceSingle, Build: "ur board",
		Desc: "An old race game for 1 or 2, played from beside it (on a chair if there is one): cerebral fun at 80% power. Trains Intellectual."},
	{ID: "chess_table", Label: "Chess table", Category: CatRecreation, Icon: "chess_table", Mode: PlaceSingle, Build: "chess table",
		Desc: "A board game for 2, played from beside it: cerebral fun; better quality, more fun. Trains Intellectual."},
	{ID: "billiards_table", Label: "Billiards table", Category: CatRecreation, Icon: "billiards_table", Mode: PlaceSingle, Build: "billiards table",
		Desc: "A 2x3 felt table for 2 players: dexterity fun at 130% power. Needs cloth and wood besides its material. Trains Shooting. Q/E rotates."},
	{ID: "poker_table", Label: "Poker table", Category: CatRecreation, Icon: "poker_table", Mode: PlaceSingle, Build: "poker table",
		Desc: "A 2x2 card table for up to 4, sat around: cerebral fun at 130% power. Needs cloth and wood besides its material. Trains Intellectual."},
	{ID: "tube_tv", Label: "Tube television", Category: CatRecreation, Icon: "tube_television", Mode: PlaceSingle, Build: "tube television",
		Desc: "Watched from the 3x5 area in front of it, seats best: television fun at 120% power while powered (200 W). Q/E rotates."},
	{ID: "flatscreen_tv", Label: "Flatscreen television", Category: CatRecreation, Icon: "flatscreen_television", Mode: PlaceSingle, Build: "flatscreen television",
		Desc: "A 2x1 screen watched from the 5x6 area in front of it: television fun at 140% power while powered (330 W). Q/E rotates."},
	{ID: "big_screen_tv", Label: "Big screen television", Category: CatRecreation, Icon: "big_screen_television", Mode: PlaceSingle, Build: "big screen television",
		Desc: "A 3x1 screen no one builds: found in old ruins. Watched from the 7x6 area in front: television fun at 160% power while powered (450 W)."},
	{ID: "telescope", Label: "Telescope", Category: CatRecreation, Icon: "telescope", Mode: PlaceSingle, Build: "telescope",
		Desc: "A scope no one builds: found in old ruins. Stargazing at night under an open sky: telescope fun at 120% power. Trains Intellectual."},
	{ID: "carpet", Label: "Carpet", Category: CatFloors, Icon: "carpet", Mode: PlaceSingle, Build: "carpet",
		Desc: "Wipes dirt off boots. Lay it on the tile just inside a door. Pick a carpet below."},
	{ID: "lamp", Label: "Lamp", Category: "furniture", Icon: "lamp", Mode: PlaceSingle, Build: "lamp",
		Desc: "An electric light: lights its room at night while it has power. Needs a conduit, generator or battery within 6 tiles."},
	{ID: "torch_lamp", Label: "Torch lamp", Category: "furniture", Icon: "torch_lamp", Mode: PlaceSingle, Build: overlay.TorchLampKind,
		Desc: "Burns wood to light its room; no power needed. Haulers keep it fuelled, and rain or snow puts it out outdoors."},
	{ID: "trough", Label: "Feeding trough", Category: "furniture", Icon: "feeding_trough", Mode: PlaceSingle, Build: "feeding trough",
		Desc: "Pen animals eat from it when the grass is gone. Haulers fill it; choose plants, meat or any when you select it. Q/E rotates."},
	{ID: "workbench", Label: "Workbench", Category: "production", Icon: "workbench", Mode: PlaceSingle, Build: "workbench",
		Desc: "Crafting station (3x1). Select it to add bills. Q/E rotates."},
	{ID: "hopper", Label: "Hopper", Category: "production", Icon: "hopper", Mode: PlaceSingle, Build: "hopper",
		Desc: "Holds raw food for a nutrient paste dispenser beside it. Haulers fill it; set what it takes from its Storage button."},
	{ID: "nutrient_paste_dispenser", Label: "Nutrient paste dispenser", Category: "production", Icon: "nutrient_paste_dispenser", Mode: PlaceSingle, Build: "nutrient paste dispenser",
		Desc: "Turns raw food from hoppers beside it into paste meals, with power. Colonists take a meal from the tile south of its middle. Q/E rotates."},
	{ID: "stove", Label: "Stove", Category: "production", Icon: "stove", Mode: PlaceSingle, Build: "stove",
		Desc: "Cooks meals from raw food (3x1). Select it to add bills."},
	{ID: "electric_stove", Label: "Electric stove", Category: "production", Icon: "electric_stove", Mode: PlaceSingle, Build: "electric stove",
		Desc: "Cooks the stove's meals 40% faster while it has power, and nothing without (3x1). Select it to add bills. Q/E rotates."},
	{ID: "butcher", Label: "Butcher table", Category: "production", Icon: "butcher_table", Mode: PlaceSingle, Build: "butcher table",
		Desc: "Butchers hunted carcasses into meat (3x1). Starts with butcher bills."},
	{ID: "campfire", Label: "Campfire", Category: CatTemperature, Icon: "campfire", Mode: PlaceSingle, Build: "campfire",
		Desc: "Burns wood to heat and light its room, and cooks simple meals (slowly). Haulers keep it fuelled."},
	{ID: "grave", Label: "Grave", Category: "misc", Icon: "stone", Mode: PlaceSingle, Build: "grave",
		Desc: "Dig a grave: colonists bury the dead here."},
	{ID: "pyre", Label: "Funeral pyre", Category: "misc", Icon: "campfire", Mode: PlaceSingle, Build: "funeral pyre",
		Desc: "Funeral pyre: burn the dead (2 wood per body)."},
	{ID: "tailor", Label: "Tailoring bench", Category: "production", Icon: "tailoring_bench", Mode: PlaceSingle, Build: "tailoring bench",
		Desc: "Sews leather into parkas, tuques and dusters (3x1). Select it to add bills."},
	{ID: "electric_tailor", Label: "Electric tailoring bench", Category: "production", Icon: "electric_tailoring_bench", Mode: PlaceSingle, Build: "electric tailoring bench",
		Desc: "Sews the tailoring bench's clothes 40% faster while it has power, and nothing without (3x1). Select it to add bills. Q/E rotates."},
	{ID: "drying", Label: "Drying rack", Category: "production", Icon: "drying_rack", Mode: PlaceSingle, Build: "drying rack",
		Desc: "Dries up to 25 perishable food at a time so it never rots (2x1). Starts with a Dry food bill."},
	{ID: "stonecutter", Label: "Stonecutter table", Category: "production", Icon: "stonecutter_table", Mode: PlaceSingle, Build: "stonecutter table",
		Desc: "Cuts stone chunks into stone blocks for building (3x1). Select it to add bills. Q/E rotates."},
	{ID: "research", Label: "Research bench", Category: "production", Icon: "research_bench", Mode: PlaceSingle, Build: "research bench",
		Desc: "A simple desk to study at (3x2): research runs at 75% here, and only basic projects. Pick a project in the Research tab; colonists with Research work sit here. Q/E rotates."},
	{ID: "electric_research", Label: "Hi-tech research bench", Category: "production", Icon: "electric_research_bench", Mode: PlaceSingle, Build: "electric research bench",
		Desc: "A powered research station (5x2, 250 W): full research speed and the advanced projects, nothing without power. A multi-analyzer nearby adds 10%. Q/E rotates."},
	{ID: "multi_analyzer", Label: "Multi-analyzer", Category: "misc", Icon: "multi_analyzer", Mode: PlaceSingle, Build: "multi-analyzer",
		Desc: "A powered sensor bank (2x2, 200 W). A hi-tech research bench within 6 tiles links to it: +10% research and the hardest projects."},
	{ID: "tool_cabinet", Label: "Tool cabinet", Category: "misc", Icon: "tool_cabinet", Mode: PlaceSingle, Build: "tool cabinet",
		Desc: "Tools on hand: benches within 6 tiles work 6% faster (two cabinets count)."},
	{ID: "sandbags", Label: "Sandbags", Category: "security", Icon: "sandbags", Mode: PlaceLine, Build: "sandbags",
		Desc: "Waist-high cover. Colonists right behind them are much harder to shoot. Drag to draw a line."},
	{ID: "spike_trap", Label: "Spike trap", Category: "security", Icon: "spike_trap", Mode: PlaceSingle, Build: "spike trap",
		Desc: "Springs on raiders and predators that walk over it, then is re-laid as a blueprint. Colonists step around it safely."},
	// PLAY-024: defence and trade.
	{ID: "sentry_turret", Label: "Sentry turret", Category: "security", Icon: "sentry_turret", Mode: PlaceSingle, Build: overlay.SentryTurretKind,
		Desc: "A powered gun post: shoots raiders and hostile animals within 10 tiles in line of sight. Shoots only while powered; select it to see its reach."},
	// Wave D (PRD-3, BED-3, DEF-3).
	{ID: "crafting_spot", Label: "Crafting spot", Category: "production", Icon: "crafting_spot", Mode: PlaceSingle, Build: "crafting spot",
		Desc: "A marked tile for simple hand crafting (no bench). Free and instant."},
	{ID: "butcher_spot", Label: "Butchering spot", Category: "production", Icon: "butcher_spot", Mode: PlaceSingle, Build: "butchering spot",
		Desc: "A marked tile to butcher carcasses on, slowly. Free and instant."},
	{ID: "fueled_smithy", Label: "Wood-fired forge", Category: "production", Icon: "fueled_smithy", Mode: PlaceSingle, Build: "wood-fired forge",
		Desc: "Smiths weapons and tools; burns wood while it works (3x1). Select it to add bills. Q/E rotates."},
	{ID: "electric_smithy", Label: "Electric forge", Category: "production", Icon: "electric_smithy", Mode: PlaceSingle, Build: "electric forge",
		Desc: "A powered forge: faster smithing than the wood-fired one (3x1). Q/E rotates."},
	{ID: "machining_table", Label: "Machining table", Category: "production", Icon: "machining_table", Mode: PlaceSingle, Build: "machining table",
		Desc: "Machines firearms and heavy parts while powered (3x1). Q/E rotates."},
	{ID: "fabrication_bench", Label: "Fabrication bench", Category: "production", Icon: "fabrication_bench", Mode: PlaceSingle, Build: "fabrication bench",
		Desc: "Builds advanced components and parts while powered (5x2). Q/E rotates."},
	{ID: "art_bench", Label: "Sculpting bench", Category: "production", Icon: "art_bench", Mode: PlaceSingle, Build: "sculpting bench",
		Desc: "Carves sculptures to make rooms prettier (3x1). Select it to add bills. Q/E rotates."},
	{ID: "electric_smelter", Label: "Electric smelter", Category: "production", Icon: "electric_smelter", Mode: PlaceSingle, Build: "electric smelter",
		Desc: "Smelts slag and scrap into metal while powered (3x1). Q/E rotates."},
	{ID: "biofuel_refinery", Label: "Fuel refinery", Category: "production", Icon: "biofuel_refinery", Mode: PlaceSingle, Build: "fuel refinery",
		Desc: "Refines plant matter into fuel while powered (3x2). Q/E rotates."},
	{ID: "fermenting_barrel", Label: "Fermenting barrel", Category: "production", Icon: "fermenting_barrel", Mode: PlaceSingle, Build: "fermenting barrel",
		Desc: "Ferments brewgrain into beer over days. Haulers fill and empty it."},
	{ID: "small_sculpture", Label: "Small sculpture", Category: "furniture", Icon: "sculpture_small", Mode: PlaceSingle, Build: "small sculpture",
		Desc: "A small carved piece that makes its surroundings prettier."},
	{ID: "large_sculpture", Label: "Large sculpture", Category: "furniture", Icon: "sculpture_large", Mode: PlaceSingle, Build: "large sculpture",
		Desc: "A large carved piece that makes its surroundings prettier."},
	{ID: "grand_sculpture", Label: "Grand sculpture", Category: "furniture", Icon: "sculpture_grand", Mode: PlaceSingle, Build: "grand sculpture",
		Desc: "A grand carved piece that makes its surroundings much prettier."},
	{ID: "stool", Label: "Stool", Category: "furniture", Icon: "stool", Mode: PlaceSingle, Build: "stool",
		Desc: "A cheap seat: colonists can sit at a table, but it is not comfy."},
	{ID: "armchair", Label: "Armchair", Category: "furniture", Icon: "armchair", Mode: PlaceSingle, Build: "armchair",
		Desc: "A comfortable seat: colonists relax in it. Q/E rotates."},
	{ID: "barricade", Label: "Barricade", Category: "security", Icon: "barricade", Mode: PlaceLine, Build: "barricade",
		Desc: "Cover that stops movement and bullets alike. Drag to draw a line."},
	{ID: "ied_blast", Label: "Blast charge", Category: "security", Icon: "ied_high_explosive", Mode: PlaceSingle, Build: "blast charge",
		Desc: "A hidden charge that explodes when raiders step on it."},
	{ID: "ied_fire", Label: "Fire charge", Category: "security", Icon: "ied_incendiary", Mode: PlaceSingle, Build: "fire charge",
		Desc: "A hidden charge that sets fire to those who step on it."},
	{ID: "ied_pulse", Label: "Pulse charge", Category: "security", Icon: "ied_emp", Mode: PlaceSingle, Build: "pulse charge",
		Desc: "A hidden charge that knocks out machines nearby."},
	{ID: "ied_smoke", Label: "Smoke charge", Category: "security", Icon: "ied_smoke", Mode: PlaceSingle, Build: "smoke charge",
		Desc: "A hidden charge that blankets the area in smoke."},
	// AX-5 (AR2's art).
	{ID: "ied_foam", Label: "Foam charge", Category: "security", Icon: "ied_foam", Mode: PlaceSingle, Build: "foam charge",
		Desc: "A hidden charge of two foam shells: it smothers every fire within 10 tiles when a raider steps on it."},
	{ID: "ied_warhead", Label: "Warhead charge", Category: "security", Icon: "ied_warhead", Mode: PlaceSingle, Build: "warhead charge",
		Desc: "A hidden charge built around an antimatter warhead (found in ancient vaults): a huge blast that sets fires."},
	{ID: "autocannon", Label: "Autocannon", Category: "security", Icon: "autocannon", Mode: PlaceSingle, Build: "autocannon",
		Desc: "A heavy powered gun post with long reach. Needs shells loaded by colonists."},
	{ID: "slug_turret", Label: "Slug turret", Category: "security", Icon: "slug_turret", Mode: PlaceSingle, Build: "slug turret",
		Desc: "A powered gun post firing heavy slugs at long range."},
	{ID: "foam_turret", Label: "Foam turret", Category: "security", Icon: "foam_turret", Mode: PlaceSingle, Build: "foam turret",
		Desc: "A powered nozzle that sprays foam on the fires it sees, then rests a minute. 10 charges; colonists refill it with chemfuel."},
	{ID: "rocket_battery", Label: "Rocket battery", Category: "security", Icon: "rocket_battery", Mode: PlaceSingle, Build: "rocket battery",
		Desc: "A powered launcher (2x2): select it to set a target and a drafted colonist fires 12 rockets there. Then it needs a full load of chemfuel and two minutes to recharge."},
	{ID: "mortar", Label: "Mortar", Category: "security", Icon: "mortar", Mode: PlaceSingle, Build: "mortar",
		Desc: "Long-range artillery. Select it to set a target; drafted colonists man it and carry shells."},
	{ID: "autodoor", Label: "Powered door", Category: "structure", Icon: "autodoor", Mode: PlaceSingle, Build: "powered door",
		Desc: "A door that opens for colonists on its own while powered."},
	{ID: "radio_console", Label: "Radio console", Category: "misc", Icon: "radio_console", Mode: PlaceSingle, Build: overlay.RadioConsoleKind,
		Desc: "A powered desk (3x2): a colonist works it to call traders, gift silver and ask factions for help. Q/E rotates."},
	{ID: "trade_beacon", Label: "Trade beacon", Category: "misc", Icon: "trade_beacon", Mode: PlaceSingle, Build: overlay.TradeBeaconKind,
		Desc: "Marks the goods an orbital trader can buy: stacks within 6 tiles of it. Unpowered; the ghost shows its radius."},
	{ID: "foam_popper", Label: "Foam popper", Category: "misc", Icon: "foam_popper", Mode: PlaceSingle, Build: "foam popper", // AX-5
		Desc: "Bursts in foam over 10 tiles when a fire starts within 3 tiles of it, then is rebuilt in its place."},
	{ID: "brewing_bench", Label: "Brewing bench", Category: "production", Icon: "brewing_bench", Mode: PlaceSingle, Build: "brewing bench",
		Desc: "Brews brewgrain into beer (3x1). Select it to add bills. Q/E rotates."},
	{ID: "drug_table", Label: "Drug table", Category: "production", Icon: "drug_table", Mode: PlaceSingle, Build: "drug table",
		Desc: "Steeps ashleaf into tea and presses spark tablets (3x1). Select it to add bills. Q/E rotates."},
	{ID: "marriage_spot", Label: "Marriage spot", Category: "misc", Icon: "marriage_spot", Mode: PlaceSingle, Build: "marriage spot",
		Desc: "Where engaged couples hold their wedding (2x1). Free and instant. Q/E rotates."},
	{ID: "pen_marker", Label: "Pen marker", Category: "misc", Icon: "pen_marker", Mode: PlaceSingle, Build: "pen marker",
		Desc: "Marks a pen: the area fenced in around it. Fence it in, then pick which animals it takes, its food and plants to cut when you select it."},
	{ID: "party_spot", Label: "Party spot", Category: "misc", Icon: "party_spot", Mode: PlaceSingle, Build: "party spot",
		Desc: "Where the colony gathers for parties. Free and instant."},
	// AX-2 misc (wiki: Egg box, Large stele, Grand stele; AR1's art).
	{ID: "egg_box", Label: "Egg box", Category: CatStorage, Icon: "egg_box", Mode: PlaceSingle, Build: "egg box",
		Desc: "A nesting box: tame hens within 20 tiles lay their eggs in it, and it stores them."},
	{ID: "large_stele", Label: "Large stele", Category: "misc", Icon: "large_stele", Mode: PlaceSingle, Build: "large stele",
		Desc: "A carved stone monument (2x2) that makes its room more beautiful."},
	{ID: "grand_stele", Label: "Grand stele", Category: "misc", Icon: "grand_stele", Mode: PlaceSingle, Build: "grand stele",
		Desc: "A towering stone monument (3x3): twice the large stele's beauty."},
	// PLAY-021 temperature (Misc: see CatTemperature).
	{ID: "heater", Label: "Heater", Category: CatTemperature, Icon: "heater", Mode: PlaceSingle, Build: overlay.HeaterKind,
		Desc: "Warms its room up to its thermostat target, then idles. Select it to set the target."},
	{ID: "cooler", Label: "Cooler", Category: CatTemperature, Icon: "cooler", Mode: PlaceSingle, Build: overlay.CoolerKind,
		Desc: "Set into a wall gap, facing the room to cool: it pumps the heat out of its back. Select it to set the target. Q/E rotates."},
	{ID: "passive_cooler", Label: "Passive cooler", Category: CatTemperature, Icon: "passive_cooler", Mode: PlaceSingle, Build: "passive cooler",
		Desc: "Cools its room without power, burning wood, down to 17 °C and no further. Haulers keep it fuelled (50 wood, about 5 days)."},
	{ID: "vent", Label: "Vent", Category: CatTemperature, Icon: "vent", Mode: PlaceSingle, Build: "vent",
		Desc: "Set into a wall gap: while open it lets air through, evening out the temperatures on its two sides. Q/E rotates."},
	// STK-AG: built kinds that had no card (FOOD-2 contract).
	{ID: "hydroponics", Label: "Hydroponics basin", Category: "production", Icon: "crop", Mode: PlaceSingle, Build: "hydroponics basin",
		Desc: "A powered bed of nutrient water (1x4): crops sown in it grow far faster than on soil, but only under a sun lamp or the open sky. Q/E rotates."},
	{ID: "fish_tank", Label: "Fish tank", Category: "production", Icon: "fish_tank", Mode: PlaceSingle, Build: "fish tank",
		Desc: "A powered tank (1x4) that raises fish for the table: pick a stock, keep it fed, warm and powered, and a hand nets the catch. Q/E rotates."},
	{ID: "sun_lamp", Label: "Sun lamp", Category: "furniture", Icon: "lamp", Mode: PlaceSingle, Build: "sun lamp",
		Desc: "A strong powered light that lets crops grow indoors, under a roof, in the area it lights."},
	// PLAY-021 power: the grid's pieces.
	{ID: "conduit", Label: "Power conduit", Category: CatPower, Icon: "power_conduit", Mode: PlaceLine, Build: overlay.ConduitKind,
		Desc: "Carries power along the line; devices within 6 tiles wire to it, and generators and batteries too unless they touch it. Drag to draw a line."},
	{ID: "hidden_conduit", Label: "Hidden conduit", Category: CatPower, Icon: "hidden_conduit", Mode: PlaceLine, Build: overlay.HiddenConduitKind,
		Desc: "A conduit buried out of sight: it carries power the same way, shows only in the power overlay and never shorts out. Drag to draw a line."},
	{ID: "power_switch", Label: "Power switch", Category: CatPower, Icon: "power_switch", Mode: PlaceSingle, Build: overlay.PowerSwitchKind,
		Desc: "Joins the conduits on either side while on; switched off, it cuts the grid in two there. A colonist flicks it."},
	{ID: "wood_generator", Label: "Wood-fired generator", Category: CatPower, Icon: "wood_fired_generator", Mode: PlaceSingle, Build: overlay.WoodGeneratorKind,
		Desc: "Burns wood for steady power, day and night (2x2). Haulers keep it fuelled."},
	{ID: "solar_panel", Label: "Solar panel", Category: CatPower, Icon: "solar_panel", Mode: PlaceSingle, Build: overlay.SolarPanelKind,
		Desc: "Turns daylight into power: most at noon, nothing at night or under a roof (3x3)."},
	{ID: "wind_turbine", Label: "Wind turbine", Category: CatPower, Icon: "wind_turbine", Mode: PlaceSingle, Build: overlay.WindTurbineKind,
		Desc: "Power from the wind, more in a gale (1x2). Walls, rock and trees close by steal its wind. Q/E rotates."},
	{ID: "geothermal", Label: "Geothermal generator", Category: CatPower, Icon: "geothermal_generator", Mode: PlaceSingle, Build: overlay.GeothermalKind,
		Desc: "Built over a steam vent: steady power day and night, no fuel (2x2)."},
	{ID: "battery", Label: "Battery", Category: CatPower, Icon: "battery", Mode: PlaceSingle, Build: overlay.BatteryKind,
		Desc: "Stores the grid's spare power for the night and lean hours, and slowly leaks it (1x2). Q/E rotates."},
	// AX-4 power and industry (AR2's art).
	{ID: "waterproof_conduit", Label: "Waterproof conduit", Category: CatPower, Icon: "waterproof_conduit", Mode: PlaceLine, Build: "waterproof conduit",
		Desc: "A sealed conduit that also runs across shallow water and marsh. Drag to draw a line."},
	{ID: "chemfuel_generator", Label: "Chemfuel generator", Category: CatPower, Icon: "chemfuel_generator", Mode: PlaceSingle, Build: "chemfuel generator",
		Desc: "Burns chemfuel for 1000 W, day and night (2x2): a full tank of 30 lasts almost a week. Haulers keep it fuelled."},
	{ID: "watermill", Label: "Watermill generator", Category: CatPower, Icon: "watermill", Mode: PlaceSingle, Build: "watermill generator",
		Desc: "A wheel turned by a river (5x6): 1100 W while the wheel row sits in moving water, a third with another watermill close up or down the river. Q/E rotates."},
	{ID: "deep_drill", Label: "Deep drill", Category: "production", Icon: "deep_drill", Mode: PlaceSingle, Build: "deep drill",
		Desc: "A powered drill (1x2, 200 W) a miner works: it brings up the deep ore a ground scan found within 2 tiles, else stone chunks. Q/E rotates."},
	{ID: "crematorium", Label: "Electric crematorium", Category: "production", Icon: "electric_crematorium", Mode: PlaceSingle, Build: "electric crematorium",
		Desc: "A powered stone furnace (3x2, 250 W) that burns corpses without wood. Starts with a Cremate bill. Q/E rotates."},
	{ID: "deep_scanner", Label: "Ground-penetrating scanner", Category: "misc", Icon: "ground_scanner", Mode: PlaceSingle, Build: "ground-penetrating scanner",
		Desc: "A powered scanner (3x3, 700 W) a researcher works: every few days of work it finds a lump of deep ore for the deep drill. Q/E rotates."},
	{ID: "moisture_pump", Label: "Moisture pump", Category: "misc", Icon: "moisture_pump", Mode: PlaceSingle, Build: "moisture pump",
		Desc: "A powered pump (150 W) that slowly dries marsh, mud and shallow water around it into soil: one tile of radius every 9 days, up to 7."},
	// AX-9 travel (AR4's art).
	{ID: "stasis_casket", Label: "Stasis casket", Category: "misc", Icon: "stasis_casket", Mode: PlaceSingle, Build: "stasis casket",
		Desc: "A sealed bed (1x2) that holds one sleeper out of time: no hunger, no aging, wounds and sickness stand still. A colonist climbs in, or carries a downed one in; Eject lets them out. Q/E rotates."},
	{ID: "transport_pod", Label: "Transport pod", Category: "misc", Icon: "transport_pod", Mode: PlaceSingle, Build: "transport pod",
		Desc: "A one-way pod that lifts 150 kg of people, animals and goods. Pods that touch load and fly as a group; each needs a pod launcher beside it."},
	{ID: "pod_launcher", Label: "Pod launcher", Category: "misc", Icon: "pod_launcher", Mode: PlaceSingle, Build: "pod launcher",
		Desc: "Holds 150 chemfuel and fires the transport pod beside it up to 66 tiles: 2.25 chemfuel a tile a pod (1x2). Haulers keep it fuelled. Q/E rotates."},
	{ID: "mineral_scanner", Label: "Long-range mineral scanner", Category: "misc", Icon: "mineral_scanner", Mode: PlaceSingle, Build: "long-range mineral scanner",
		Desc: "A powered dish (3x3, 700 W) a researcher works: every few days of work it finds a lump of ore 6 to 25 tiles away on the world map for a caravan or pods to mine. Q/E rotates."},
	{ID: "hitching_spot", Label: "Caravan hitching spot", Category: "misc", Icon: "hitching_spot", Mode: PlaceSingle, Build: "caravan hitching spot",
		Desc: "A forming caravan gathers here and waits a while before it heads for the map edge."},
	// AX-8 the escape ship (wiki: Ship; AR4's starship art).
	{ID: "ship_beam", Label: "Ship structural beam", Category: CatShip, Icon: "ship_beam", Mode: PlaceSingle, Build: "ship structural beam",
		Desc: "The keel of an escape ship (2x6): parts that touch join into one ship, and a stasis casket joins only through a beam. Q/E rotates."},
	{ID: "ship_casket", Label: "Ship stasis casket", Category: CatShip, Icon: "ship_casket", Mode: PlaceSingle, Build: "ship stasis casket",
		Desc: "A berth for one sleeper on the flight (1x2), joined to a beam. Right-click it with a colonist selected to board. Q/E rotates."},
	{ID: "ship_core", Label: "Ship computer core", Category: CatShip, Icon: "ship_core", Mode: PlaceSingle, Build: "ship computer core",
		Desc: "The mind that steers the ship (2x2). Built around a mind core, which only traders and quests offer."},
	{ID: "ship_reactor", Label: "Ship reactor", Category: CatShip, Icon: "ship_reactor", Mode: PlaceSingle, Build: "ship reactor",
		Desc: "Lifts the ship (6x7) and gives 1000 W. Starting it begins a 15-day waking that draws raids until it is done."},
	{ID: "ship_engine", Label: "Ship engine", Category: CatShip, Icon: "ship_engine", Mode: PlaceSingle, Build: "ship engine",
		Desc: "Pushes the ship into the sky (3x4); a ship needs three. Q/E rotates."},
	{ID: "ship_sensor", Label: "Ship sensor cluster", Category: CatShip, Icon: "ship_sensor", Mode: PlaceSingle, Build: "ship sensor cluster",
		Desc: "Lets the ship find its way (2x2); a ship needs one to launch."},
	// NST-001..027 (Neat Storage; .cursor/NEAT_STORAGE_GAP_ANALYSIS.md): Build is the build name, Icon the slug.
	{ID: "display_case", Label: "Display case", Category: CatStorage, Icon: "display_case", Mode: PlaceSingle, Build: "display case",
		Desc: "A glass-fronted box for one fine thing. Colonists who walk past stop to look at it. Q/E rotates."},
	{ID: "round_pedestal", Label: "Round pedestal", Category: CatStorage, Icon: "round_pedestal", Mode: PlaceSingle, Build: "round pedestal",
		Desc: "A turned stand that holds one piece up for everyone to see; looking at it calms people."},
	{ID: "small_display_shelf", Label: "Small display shelf", Category: CatStorage, Icon: "small_display_shelf", Mode: PlaceSingle, Build: "small display shelf",
		Desc: "A one-tile rack with three slots, for nice things you want in sight. Q/E rotates."},
	{ID: "display_shelf", Label: "Display shelf", Category: CatStorage, Icon: "display_shelf", Mode: PlaceSingle, Build: "display shelf",
		Desc: "A wide display rack, three slots a tile, kept tidy for goods worth showing. Q/E rotates."},
	{ID: "weapon_rack", Label: "Weapon rack", Category: CatStorage, Icon: "weapon_rack", Mode: PlaceSingle, Build: "weapon rack",
		Desc: "A rack with pegs for three weapons a tile. Q/E rotates."},
	{ID: "narrow_wardrobe", Label: "Narrow wardrobe", Category: CatStorage, Icon: "narrow_wardrobe", Mode: PlaceSingle, Build: "narrow wardrobe",
		Desc: "A slim cupboard with four slots a tile for clothes and cloth. Q/E rotates."},
	{ID: "wardrobe", Label: "Wardrobe", Category: CatStorage, Icon: "wardrobe", Mode: PlaceSingle, Build: "wardrobe",
		Desc: "A two-tile cupboard with four slots a tile for clothes and cloth. Q/E rotates."},
	{ID: "narrow_display_cabinet", Label: "Narrow display cabinet", Category: CatStorage, Icon: "narrow_display_cabinet", Mode: PlaceSingle, Build: "narrow display cabinet",
		Desc: "A slim glass cabinet, four slots a tile, for goods, meals and medicine. Q/E rotates."},
	{ID: "display_cabinet", Label: "Display cabinet", Category: CatStorage, Icon: "display_cabinet", Mode: PlaceSingle, Build: "display cabinet",
		Desc: "A two-tile glass cabinet, four slots a tile, for goods, meals and medicine. Q/E rotates."},
	{ID: "open_shelf", Label: "Open shelf", Category: CatStorage, Icon: "open_shelf", Mode: PlaceSingle, Build: "open shelf",
		Desc: "Plain open boards, six slots a tile, for anything that is not a corpse or a chunk. Q/E rotates."},
	{ID: "two_tier_display_shelf", Label: "2-tier display shelf", Category: CatStorage, Icon: "two_tier_display_shelf", Mode: PlaceSingle, Build: "two tier display shelf",
		Desc: "Two shelves stacked on one frame: six slots a tile for goods worth showing. Q/E rotates."},
	{ID: "narrow_tall_shelf", Label: "Narrow tall shelf", Category: CatStorage, Icon: "narrow_tall_shelf", Mode: PlaceSingle, Build: "narrow tall shelf",
		Desc: "A tall, narrow unit with six slots a tile. Q/E rotates."},
	{ID: "tall_shelf", Label: "Tall shelf", Category: CatStorage, Icon: "tall_shelf", Mode: PlaceSingle, Build: "tall shelf",
		Desc: "A tall two-tile unit with six slots a tile. Q/E rotates."},
	{ID: "safe", Label: "Safe", Category: CatStorage, Icon: "safe", Mode: PlaceSingle, Build: "safe",
		Desc: "A steel strongbox with six slots, for metals, parts, drugs and medicine."},
	{ID: "drum", Label: "Drum", Category: CatStorage, Icon: "drum", Mode: PlaceSingle, Build: "drum",
		Desc: "A steel drum with six slots for fuel, chemicals, milk and drink."},
	{ID: "horizontal_weapon_rack", Label: "Horizontal weapon rack", Category: CatStorage, Icon: "horizontal_weapon_rack", Mode: PlaceSingle, Build: "horizontal weapon rack",
		Desc: "A steel rack that lays six weapons a tile flat. Q/E rotates."},
	{ID: "vertical_weapon_rack", Label: "Vertical weapon rack", Category: CatStorage, Icon: "vertical_weapon_rack", Mode: PlaceSingle, Build: "vertical weapon rack",
		Desc: "A steel rack that stands six weapons a tile on end. Q/E rotates."},
	{ID: "textile_rack", Label: "Textile rack", Category: CatStorage, Icon: "textile_rack", Mode: PlaceSingle, Build: "textile rack",
		Desc: "A frame with six slots a tile for cloth, hides and carpets. Q/E rotates."},
	{ID: "meat_hook", Label: "Meat hook", Category: CatStorage, Icon: "meat_hook", Mode: PlaceSingle, Build: "meat hook",
		Desc: "Steel hooks with six slots a tile, to keep raw meat off the floor. Q/E rotates."},
	{ID: "mini_pallet", Label: "Mini pallet", Category: CatStorage, Icon: "mini_pallet", Mode: PlaceSingle, Build: "mini pallet",
		Desc: "A tiny board platform with six slots; goods keep off the bare ground."},
	{ID: "half_pallet", Label: "Half pallet", Category: CatStorage, Icon: "half_pallet", Mode: PlaceSingle, Build: "half pallet",
		Desc: "A two-tile board platform with six slots a tile. Q/E rotates."},
	{ID: "pallet", Label: "Pallet", Category: CatStorage, Icon: "pallet", Mode: PlaceSingle, Build: "pallet",
		Desc: "A big board platform, four tiles with six slots each."},
	{ID: "small_crate", Label: "Small crate", Category: CatStorage, Icon: "small_crate", Mode: PlaceSingle, Build: "small crate",
		Desc: "A small slatted crate with six slots."},
	{ID: "crate", Label: "Crate", Category: CatStorage, Icon: "crate", Mode: PlaceSingle, Build: "crate",
		Desc: "A two-tile crate with six slots a tile. Q/E rotates."},
	{ID: "large_crate", Label: "Large crate", Category: CatStorage, Icon: "large_crate", Mode: PlaceSingle, Build: "large crate",
		Desc: "A big crate, four tiles with six slots each."},
	{ID: "tall_crate", Label: "Tall crate", Category: CatStorage, Icon: "tall_crate", Mode: PlaceSingle, Build: "tall crate",
		Desc: "A tall crate on a four-tile base, six slots a tile."},
	{ID: "corpse_dump_spot", Label: "Corpse dump spot", Category: CatStorage, Icon: "corpse_dump_spot", Mode: PlaceSingle, Build: "corpse dump spot",
		Desc: "One tile for up to twelve bodies, bones and chunks, out of the way."},
}

// allTools is the working tools, then the placeholders.
var allTools = append(append(make([]ToolDef, 0, len(Tools)+len(Planned)), Tools...), Planned...)

// AllTools is every architect card: the working tools, then the
// placeholders (Planned).
func AllTools() []ToolDef { return allTools }

// ToolByID is the tool with id.
func ToolByID(id string) (ToolDef, bool) {
	for _, t := range AllTools() {
		if t.ID == id {
			return t, true
		}
	}
	return ToolDef{}, false
}

// ToolsIn is category's cards: its working tools in catalog order, then
// its placeholders in the vanilla order. Working tools come first so a
// small screen, whose grid holds fewer cells, never hides one behind a
// greyed card, and they keep the first command keys.
func ToolsIn(category string) []ToolDef {
	var out []ToolDef
	for _, t := range AllTools() {
		if t.Category == category && (t.Family == "" || t.Family == t.ID) { // a family shows one card
			out = append(out, t)
		}
	}
	return out
}

// NextInFamily is the member of cur's family after it ("" after the last
// or for a tool in no family).
func NextInFamily(cur ToolDef) string {
	seen := false
	for _, t := range AllTools() {
		switch {
		case cur.Family == "" || t.Family != cur.Family:
		case seen:
			return t.ID
		case t.ID == cur.ID:
			seen = true
		}
	}
	return ""
}

// buildableCatalog is the static catalog by kind, read once: game.Buildables
// rebuilds it on every call and the Architect asks several times a frame.
// The views are shared and read-only.
var buildableCatalog = sync.OnceValue(func() map[string]game.BuildableView {
	all := game.Buildables()
	out := make(map[string]game.BuildableView, len(all))
	for _, b := range all {
		out[b.Kind] = b
	}
	return out
})

// Buildable is the catalog entry of a construction kind.
func Buildable(kind string) (game.BuildableView, bool) {
	b, ok := buildableCatalog()[kind]
	return b, ok
}

// PowerLine is an electrical card's power line, from the simulation's
// own numbers: what a consumer draws, a producer makes at most or a
// battery holds. "" for conduits and switches (they only carry power) and
// for everything that is not electrical.
func PowerLine(bv game.BuildableView) string {
	s := power.SpecOf(construction.Kind(bv.ID))
	switch s.Role {
	case power.Consumer:
		if s.Idle == s.Draw {
			return fmt.Sprintf("Power: uses %d W", s.Draw)
		}
		return fmt.Sprintf("Power: uses %d W working, %d W idle", s.Draw, s.Idle)
	case power.Producer:
		return fmt.Sprintf("Power: makes up to %d W", s.Peak)
	case power.Storage:
		return fmt.Sprintf("Power: stores %d Wh", s.CapWh)
	}
	return ""
}

// PlacementCells returns the cells a drag from (x0, y0) to (x1, y1)
// covers for mode, in a stable order.
func PlacementCells(mode PlaceMode, x0, y0, x1, y1 int) [][2]int {
	switch mode {
	case PlaceSingle:
		return [][2]int{{x1, y1}}
	case PlaceLine:
		dx, dy := mathx.Abs(x1-x0), mathx.Abs(y1-y0)
		var out [][2]int
		if dx >= dy {
			for _, x := range span(x0, x1) {
				out = append(out, [2]int{x, y0})
			}
		} else {
			for _, y := range span(y0, y1) {
				out = append(out, [2]int{x0, y})
			}
		}
		return out
	default:
		var out [][2]int
		for _, y := range span(y0, y1) {
			for _, x := range span(x0, x1) {
				out = append(out, [2]int{x, y})
			}
		}
		return out
	}
}

func span(a, b int) []int {
	if b < a {
		a, b = b, a
	}
	out := make([]int, 0, b-a+1)
	for v := a; v <= b; v++ {
		out = append(out, v)
	}
	return out
}

// FacingNames maps construction facings to sprite suffixes.
var FacingNames = [...]string{"south", "east", "north", "west"}

// RotateFacing turns facing f by dir quarter turns (clockwise positive).
func RotateFacing(f uint8, dir int) uint8 {
	return uint8((int(f) + dir + 4) % 4)
}

// KindID is the construction kind id the simulation expects in commands.
func KindID(kind string) (uint8, bool) {
	b, ok := Buildable(kind)
	return b.ID, ok
}
