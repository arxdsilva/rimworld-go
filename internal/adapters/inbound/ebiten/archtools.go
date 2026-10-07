package ebiten

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Architect tool catalog (MAIN_UI_SCREENS §17, UI checklist §5). Only
// categories with content are shown. Buildable costs come from the
// simulation's catalog (game.Buildables) so the UI never drifts.

type placeMode uint8

const (
	placeSingle placeMode = iota // one cell under the cursor
	placeLine                    // straight line along the dominant drag axis
	placeFill                    // every cell of the dragged rectangle
	placeArea                    // one rectangle command (zones, orders)
)

type archTool struct {
	id       string
	label    string
	category string
	icon     string // ui_icon/<name>
	desc     string
	mode     placeMode
	build    string       // construction kind for buildables
	cmd      command.Kind // area command for zones/orders
	crop     string       // crop for growing zones
}

var archCategories = []struct{ id, label string }{
	{"orders", "Orders"},
	{"zones", "Zones"},
	{"structure", "Structure"},
	{"furniture", "Furniture"},
	{"production", "Production"},
	{"security", "Security"},
}

var archTools = []archTool{
	{id: "harvest", label: "Harvest", category: "orders", icon: "order_harvest", mode: placeArea, cmd: command.DesignateHarvest,
		desc: "Mark ripe berry bushes for harvesting."},
	{id: "deconstruct", label: "Deconstruct", category: "orders", icon: "order_deconstruct", mode: placeArea, cmd: command.DesignateDeconstruct,
		desc: "Take buildings down. Refunds half the wood."},
	{id: "cancel", label: "Cancel", category: "orders", icon: "order_cancel", mode: placeArea, cmd: command.DesignateCancel,
		desc: "Remove blueprints, zones and orders in an area."},
	{id: "cut", label: "Cut plants", category: "orders", icon: "order_cut", mode: placeArea, cmd: command.DesignateCut,
		desc: "Chop trees for wood or clear bushes."},
	{id: "mine", label: "Mine", category: "orders", icon: "order_mine", mode: placeArea, cmd: command.DesignateMine,
		desc: "Dig out rock. Each tile drops stone blocks."},
	{id: "hunt", label: "Hunt", category: "orders", icon: "order_hunt", mode: placeArea, cmd: command.DesignateHunt,
		desc: "Mark animals for hunters. Hunters need a rifle; kills drop meat."},
	{id: "tame", label: "Tame", category: "orders", icon: "order_tame", mode: placeArea, cmd: command.DesignateTame,
		desc: "Mark animals for taming. Hunters coax them; a failed try can enrage the animal. Tame wolves guard the colony."},
	{id: "pen", label: "Animal pen", category: "zones", icon: "zone_pen", mode: placeArea, cmd: command.DesignatePen,
		desc: "Tame animals live inside pens (without one they stay near the colony)."},
	{id: "stockpile", label: "Stockpile", category: "zones", icon: "zone_stockpile", mode: placeArea, cmd: command.DesignateStockpile,
		desc: "Storage area. Haulers bring food and wood here."},
	{id: "grow", label: "Growing zone", category: "zones", icon: "zone_grow", mode: placeArea, cmd: command.DesignateGrowZone,
		crop: "potato", desc: "Pick a crop below. Colonists sow it and harvest when ripe; crops grow in daylight, faster on rich soil."},
	{id: "wall", label: "Wall", category: "structure", icon: "wall", mode: placeLine, build: "wall",
		desc: "Blocks movement. Drag to draw a line."},
	{id: "door", label: "Door", category: "structure", icon: "door", mode: placeSingle, build: "door",
		desc: "Lets colonists through a wall."},
	{id: "floor", label: "Wood floor", category: "structure", icon: "floor", mode: placeFill, build: "floor",
		desc: "Drag to cover an area. Furniture can stand on it."},
	{id: "bed", label: "Bed", category: "furniture", icon: "bed", mode: placeSingle, build: "bed",
		desc: "A proper two-tile bed. Q/E rotates."},
	{id: "double_bed", label: "Double bed", category: "furniture", icon: "double_bed", mode: placeSingle, build: "double bed",
		desc: "A 2x2 bed two colonists share side by side. Q/E rotates."},
	{id: "spot", label: "Sleeping spot", category: "furniture", icon: "sleeping_spot", mode: placeSingle, build: "sleeping spot",
		desc: "Marks a bed-sized place to sleep on the floor. Free and instant; a bed is more comfortable. Q/E rotates."},
	{id: "table", label: "Table", category: "furniture", icon: "table", mode: placeSingle, build: "table",
		desc: "A place to eat and work."},
	{id: "chair", label: "Chair", category: "furniture", icon: "chair", mode: placeSingle, build: "chair",
		desc: "Seat for a table. Q/E rotates."},
	{id: "horseshoes", label: "Horseshoe pin", category: "furniture", icon: "horseshoe_pin", mode: placeSingle, build: "horseshoe pin",
		desc: "A game of horseshoes: the best free-time fun, keeps colonists from getting bored."},
	{id: "lamp", label: "Lamp", category: "furniture", icon: "lamp", mode: placeSingle, build: "lamp",
		desc: "Lights its room at night."},
	{id: "workbench", label: "Workbench", category: "production", icon: "workbench", mode: placeSingle, build: "workbench",
		desc: "Crafting station (2x1). Select it to add bills. Q/E rotates."},
	{id: "stove", label: "Stove", category: "production", icon: "stove", mode: placeSingle, build: "stove",
		desc: "Cooks meals from raw food (2x1). Select it to add bills."},
	{id: "butcher", label: "Butcher table", category: "production", icon: "butcher_table", mode: placeSingle, build: "butcher table",
		desc: "Butchers hunted carcasses into meat (2x1). Starts with butcher bills."},
	{id: "campfire", label: "Campfire", category: "furniture", icon: "campfire", mode: placeSingle, build: "campfire",
		desc: "Burns wood to heat and light its room, and cooks simple meals (slowly). Haulers keep it fuelled."},
	{id: "tailor", label: "Tailoring bench", category: "production", icon: "tailoring_bench", mode: placeSingle, build: "tailoring bench",
		desc: "Sews leather into parkas, tuques and dusters (2x1). Select it to add bills."},
	{id: "drying", label: "Drying rack", category: "production", icon: "drying_rack", mode: placeSingle, build: "drying rack",
		desc: "Dries up to 25 perishable food at a time so it never rots (2x1). Starts with a Dry food bill."},
	{id: "research", label: "Research bench", category: "production", icon: "research_bench", mode: placeSingle, build: "research bench",
		desc: "A desk to study at (2x1). Pick a project in the Research tab; colonists with Research work sit here. Q/E rotates."},
	{id: "sandbags", label: "Sandbags", category: "security", icon: "sandbags", mode: placeLine, build: "sandbags",
		desc: "Waist-high cover. Colonists right behind them are much harder to shoot. Drag to draw a line."},
	{id: "spike_trap", label: "Spike trap", category: "security", icon: "spike_trap", mode: placeSingle, build: "spike trap",
		desc: "Springs on raiders and predators that walk over it, then is re-laid as a blueprint. Colonists step around it safely."},
}

func toolByID(id string) (archTool, bool) {
	for _, t := range archTools {
		if t.id == id {
			return t, true
		}
	}
	return archTool{}, false
}

func toolsIn(category string) []archTool {
	var out []archTool
	for _, t := range archTools {
		if t.category == category {
			out = append(out, t)
		}
	}
	return out
}

func buildable(kind string) (game.BuildableView, bool) {
	for _, b := range game.Buildables() {
		if b.Kind == kind {
			return b, true
		}
	}
	return game.BuildableView{}, false
}

// placementCells returns the cells a drag from (x0, y0) to (x1, y1)
// covers for mode, in a stable order.
func placementCells(mode placeMode, x0, y0, x1, y1 int) [][2]int {
	switch mode {
	case placeSingle:
		return [][2]int{{x1, y1}}
	case placeLine:
		dx, dy := abs(x1-x0), abs(y1-y0)
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

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// facingNames maps construction facings to sprite suffixes.
var facingNames = [...]string{"south", "east", "north", "west"}

func rotateFacing(f uint8, dir int) uint8 {
	return uint8((int(f) + dir + 4) % 4)
}

// buildSpriteStuff is buildSprite with a material (walls and floors).
func buildSpriteStuff(kind, stuff, facing string, mask int, ew, open bool) string {
	if stuff == "stone" {
		switch kind {
		case "wall":
			return "structure/wall_stone_" + twoDigits(mask)
		case "floor":
			return "structure/floor_stone"
		}
	}
	return buildSprite(kind, facing, mask, ew, open)
}

// buildSprite is the world sprite for a building kind; neighbours only
// matter for walls (autotile) and doors (axis). open draws doors open.
func buildSprite(kind, facing string, mask int, ew, open bool) string {
	switch kind {
	case "wall":
		return "structure/wall_wood_" + twoDigits(mask)
	case "door":
		axis, state := "ns", "closed"
		if ew {
			axis = "ew"
		}
		if open {
			state = "open"
		}
		return "structure/door_wood_" + axis + "_" + state
	case "floor":
		return "structure/floor_wood"
	case "double bed":
		return "furniture/bed_double_" + facingOr(facing)
	case "bed":
		return "furniture/bed_basic_" + facingOr(facing)
	case "sleeping spot":
		return "overlay/sleeping_spot_" + facingOr(facing)
	case "workbench":
		return "furniture/workbench_basic_" + facingOr(facing)
	case "stove":
		return "furniture/stove_basic_" + facingOr(facing)
	case "butcher table":
		return "furniture/butcher_table_" + facingOr(facing)
	case "campfire":
		return "furniture/campfire"
	case "horseshoe pin":
		return "furniture/horseshoe_pin"
	case "sandbags":
		return "furniture/sandbags"
	case "research bench":
		return "furniture/research_bench_" + facingOr(facing)
	case "spike trap":
		return "furniture/spike_trap"
	case "tailoring bench":
		return "furniture/tailoring_bench_" + facingOr(facing)
	case "drying rack":
		return "furniture/drying_rack_" + facingOr(facing)
	case "table":
		return "furniture/table_small"
	case "chair":
		return "furniture/chair_basic_" + facingOr(facing)
	case "lamp":
		return "furniture/lamp_standing"
	}
	return ""
}

func facingOr(f string) string {
	if f == "" {
		return "south"
	}
	return f
}

// kindID is the construction kind id the simulation expects in commands.
func kindID(kind string) (uint8, bool) {
	b, ok := buildable(kind)
	return b.ID, ok
}
