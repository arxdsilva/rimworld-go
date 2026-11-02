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
	// Production

	// Furniture
	planned("bookcase", "furniture", "Bookcase", "Tall or low shelves that hold a colony's books."),

	// Power

	// Misc
	planned("hitching_spot", "misc", "Caravan hitching spot", "Where pack animals wait while a caravan is loaded."),
	planned("cryptosleep_casket", "misc", "Cryptosleep casket", "A powered pod that keeps a sleeper frozen in time."),
	planned("mineral_scanner", "misc", "Long-range mineral scanner", "Searches the surrounding land for mineral-rich sites."),
	planned("pod_launcher", "misc", "Pod launcher", "Fuels and fires transport pods to faraway places."),
	planned("transport_pod", "misc", "Transport pod", "A one-way pod for sending people and goods across the world."),

	// Floors

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
