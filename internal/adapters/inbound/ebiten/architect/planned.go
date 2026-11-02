package architect

// Placeholder cards (STK-AG, .cursor/ARCHITECT_GAP_ANALYSIS.md): every
// core vanilla Architect item this game does not have yet sits in its
// vanilla category as a greyed card. It cannot be armed, placed or
// dragged, so it never emits a command; its tooltip says PlannedReason
// and one original sentence about the item. When an item lands, its
// placeholder goes and a working ToolDef takes its place in Tools.

// PlannedReason is a placeholder card's disabled reason.
const PlannedReason = "Not in this game yet"

// PlannedIcon is the generic greyed "planned" icon every placeholder shows
// (ui_icon/planned, STK-AR).
const PlannedIcon = "planned"

// PlannedPrefix starts every placeholder's tool id.
const PlannedPrefix = "planned_"

// planned is a placeholder card.
func planned(id, category, label, desc string) ToolDef {
	return ToolDef{ID: PlannedPrefix + id, Label: label, Category: category, Icon: PlannedIcon, Desc: desc, Planned: true}
}

// Planned are the placeholder cards, by category in the vanilla order.
var Planned = []ToolDef{
	// Orders
	planned("uninstall", "orders", "Uninstall", "Packs a finished building up whole so it can be carried off and set down again elsewhere."),
	planned("slaughter", "orders", "Slaughter", "Marks tame animals to be killed cleanly for their meat and leather."),
	planned("claim", "orders", "Claim", "Takes over abandoned or ruined buildings so the colony owns them."),
	planned("strip", "orders", "Strip", "Has colonists take the clothes and gear off downed people and bodies."),
	planned("open", "orders", "Open", "Has a colonist pry open sealed containers such as caskets."),

	// Zones
	planned("delete_zone", "zones", "Delete zone", "Erases the cells of any stockpile or growing zone under the drag."),
	planned("clear_roof_area", "zones", "Clear build roof area", "Takes cells out of the build roof area without touching the roofs already up."),
	planned("clear_noroof_area", "zones", "Clear remove roof area", "Takes cells out of the remove roof area so roofs there may stay."),

	// Structure
	planned("ornate_door", "structure", "Ornate door", "A wide, finely worked double door that makes a grand entrance."),
	planned("column", "structure", "Column", "A slim pillar that holds a roof up without walling anything in."),
	planned("animal_flap", "structure", "Animal flap", "A small swinging hatch that lets animals through a wall while people stay out."),

	// Production
	planned("deep_drill", "production", "Deep drill", "A powered drill that reaches stone and metal far below the surface."),
	planned("crematorium", "production", "Electric crematorium", "A powered furnace that burns bodies to ash."),

	// Furniture
	planned("double_sleeping_spot", "furniture", "Double sleeping spot", "A marked patch of floor wide enough for two to sleep side by side."),
	planned("animal_sleeping_box", "furniture", "Animal sleeping box", "A simple crate bed that small animals rest in."),
	planned("tables", "furniture", "Large tables", "Square, long and big square tables that seat a whole household."),
	planned("couch", "furniture", "Couch", "A padded bench two colonists can lounge on together."),
	planned("plant_pot", "furniture", "Plant pot", "A pot of decorative greenery that brightens a room."),
	planned("bookcase", "furniture", "Bookcase", "Tall or low shelves that hold a colony's books."),
	planned("small_shelf", "furniture", "Small shelf", "A one-tile storage shelf for a corner."),
	planned("wall_torch_lamp", "furniture", "Wall torch lamp", "A fuelled torch fixed to a wall, lighting the room it faces."),
	planned("wall_lamp", "furniture", "Wall lamp", "A powered light fixed to a wall, out of the way of furniture."),
	planned("flood_light", "furniture", "Flood light", "A bright powered lamp that lights a wide stretch of ground."),
	planned("urn", "furniture", "Urn", "A decorative vessel found or traded for, never built."),

	// Power
	planned("waterproof_conduit", "power", "Waterproof conduit", "A sealed conduit that carries power across shallow water."),
	planned("chemfuel_generator", "power", "Chemfuel generator", "Burns refined fuel for strong, steady power."),
	planned("watermill", "power", "Watermill generator", "A wheel set in flowing water that turns the current into power."),

	// Security
	planned("ied_firefoam", "security", "Firefoam charge", "A hidden charge that bursts into fire-smothering foam."),
	planned("ied_antigrain", "security", "Antigrain charge", "A hidden charge built around a devastating warhead."),
	planned("foam_turret", "security", "Foam turret", "A post that sprays foam onto nearby fires on its own."),
	planned("rocketswarm", "security", "Rocketswarm launcher", "A launcher that fires a volley of rockets at a crowd of attackers."),

	// Misc
	planned("hitching_spot", "misc", "Caravan hitching spot", "Where pack animals wait while a caravan is loaded."),
	planned("egg_box", "misc", "Egg box", "A nesting box where laying animals leave their eggs."),
	planned("cryptosleep_casket", "misc", "Cryptosleep casket", "A powered pod that keeps a sleeper frozen in time."),
	planned("firefoam_popper", "misc", "Firefoam popper", "Pops a cloud of foam over a fire that starts near it."),
	planned("deep_scanner", "misc", "Ground-penetrating scanner", "Finds ore deposits deep below the map for the deep drill."),
	planned("mineral_scanner", "misc", "Long-range mineral scanner", "Searches the surrounding land for mineral-rich sites."),
	planned("moisture_pump", "misc", "Moisture pump", "Slowly dries out marsh and wet ground around it."),
	planned("pod_launcher", "misc", "Pod launcher", "Fuels and fires transport pods to faraway places."),
	planned("transport_pod", "misc", "Transport pod", "A one-way pod for sending people and goods across the world."),
	planned("large_stele", "misc", "Large stele", "A carved stone monument that lifts a room's beauty."),
	planned("grand_stele", "misc", "Grand stele", "A towering carved monument for a colony's proudest hall."),

	// Floors
	planned("straw_matting", CatFloors, "Straw matting", "Woven straw laid on the ground: cheap and quick."),
	planned("concrete", CatFloors, "Concrete", "A plain hard floor poured from steel-bound stone."),
	planned("paved_tile", CatFloors, "Paved tile", "Hard-wearing pavers for yards and roads."),
	planned("steel_tile", CatFloors, "Steel tile", "Metal floor plates that clean easily."),
	planned("silver_tile", CatFloors, "Silver tile", "A gleaming floor of silver that impresses visitors."),
	planned("gold_tile", CatFloors, "Gold tile", "An extravagant floor of gold for the finest rooms."),
	planned("sterile_tile", CatFloors, "Sterile tile", "A clean floor that lowers infections in hospitals and kitchens."),
	planned("flagstone", CatFloors, "Flagstone", "Rough stone slabs laid straight from mined chunks."),
	planned("remove_floor", CatFloors, "Remove floor", "Pulls up a laid floor, leaving the bare ground."),

	// Recreation
	planned("hoopstone", CatRecreation, "Hoopstone ring", "A ring to toss stones at, for simple fun."),
	planned("game_of_ur", CatRecreation, "Game board", "An old race game played on a carved board."),
	planned("chess_table", CatRecreation, "Chess table", "A table for slow, thoughtful board games."),
	planned("billiards", CatRecreation, "Billiards table", "A felt table where colonists shoot balls with cues."),
	planned("poker_table", CatRecreation, "Poker table", "A card table for games of chance among friends."),
	planned("tube_tv", CatRecreation, "Tube television", "A bulky powered screen to watch from nearby seats."),
	planned("flatscreen_tv", CatRecreation, "Flatscreen television", "A wide powered screen that is easy on the eyes."),
	planned("megascreen_tv", CatRecreation, "Megascreen television", "A wall-sized powered screen for a big audience."),
	planned("telescope", CatRecreation, "Telescope", "A stargazing scope for clear nights."),

	// Ship
	planned("ship_beam", CatShip, "Ship structural beam", "The frame every part of an escape ship hangs on."),
	planned("ship_casket", CatShip, "Ship cryptosleep casket", "A berth that keeps one passenger asleep for the long flight."),
	planned("ship_core", CatShip, "Ship computer core", "The mind that steers the ship once it launches."),
	planned("ship_reactor", CatShip, "Ship reactor", "Powers the ship; starting it draws danger for days."),
	planned("ship_engine", CatShip, "Ship engine", "Pushes the ship off the ground and into orbit."),
	planned("ship_sensor", CatShip, "Ship sensor cluster", "Lets the ship see where it is going."),
}
