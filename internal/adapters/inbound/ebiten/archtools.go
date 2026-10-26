package ebiten

import (
	"fmt"
	"strings"
	"sync"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
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
	enable   bool         // area command paints (Enabled) rather than clears
}

// PLAY-021 categories: Power (conduits, switch, generators, battery). The
// category panel holds four rows of two (hud_layout.go archCatsH), so a
// ninth category does not fit: the heater and cooler sit in Misc and the
// campfire stays in Furniture. archCatTemperature stays unlisted until
// the panel grows a row; every listed category gets a button.
const (
	archCatPower       = "power"
	archCatTemperature = "temperature"
)

var archCategories = []struct{ id, label string }{
	{"orders", "Orders"},
	{"zones", "Zones"},
	{"structure", "Structure"},
	{"furniture", "Furniture"},
	{"production", "Production"},
	{archCatPower, "Power"}, // PLAY-021
	{"security", "Security"},
	{"misc", "Misc"}, // ENG-003: marriage and party spots; PLAY-021: heater and cooler
}

var archTools = []archTool{
	{id: "harvest", label: "Harvest", category: "orders", icon: "order_harvest", mode: placeArea, cmd: command.DesignateHarvest,
		desc: "Mark ripe berry bushes for harvesting."},
	{id: "deconstruct", label: "Deconstruct", category: "orders", icon: "order_deconstruct", mode: placeArea, cmd: command.DesignateDeconstruct,
		desc: "Take buildings down. Refunds half the wood."},
	{id: "cancel", label: "Cancel", category: "orders", icon: "order_cancel", mode: placeArea, cmd: command.DesignateCancel,
		desc: "Remove blueprints, zones and orders in an area."},
	{id: "cut", label: "Cut plants", category: "orders", icon: "order_cut", mode: placeArea, cmd: command.DesignateCut,
		desc: "Chop trees for wood or clear bushes. Grass too."},
	{id: "mine", label: "Mine", category: "orders", icon: "order_mine", mode: placeArea, cmd: command.DesignateMine,
		desc: "Dig out rock. Each tile drops a stone chunk; ore veins drop metal."},
	{id: "smooth", label: "Smooth stone", category: "orders", icon: "order_smooth", mode: placeArea, cmd: command.DesignateSmooth,
		desc: "Smooth rock walls and rough stone ground into clean walls and floors. Not on ore."},
	{id: "plan", label: "Plan", category: "orders", icon: "plan", mode: placeArea, cmd: command.DesignatePlan, enable: true,
		desc: "Sketch a layout: paint placeholder marks. Nobody works on them and they block nothing; a finished building clears its mark."},
	{id: "unplan", label: "Remove plan", category: "orders", icon: "plan_clear", mode: placeArea, cmd: command.DesignatePlan,
		desc: "Erase plan marks in an area."},
	{id: "forbid", label: "Forbid", category: "orders", icon: "order_forbid", mode: placeArea, cmd: command.DesignateForbid, enable: true,
		desc: "Colonists leave forbidden items alone. Drag over the items."},
	{id: "allow", label: "Allow", category: "orders", icon: "order_allow", mode: placeArea, cmd: command.DesignateForbid,
		desc: "Let colonists use forbidden items again. Drag over the items."},
	{id: "haul", label: "Haul things", category: "orders", icon: "order_haul", mode: placeArea, cmd: command.DesignateHaul, enable: true,
		desc: "Mark things found on the map for hauling. Colonists haul the colony's own goods by themselves; anything else waits for this mark."},
	{id: "hunt", label: "Hunt", category: "orders", icon: "order_hunt", mode: placeArea, cmd: command.DesignateHunt,
		desc: "Mark animals for hunters. Hunters need a rifle; kills drop meat."},
	{id: "tame", label: "Tame", category: "orders", icon: "order_tame", mode: placeArea, cmd: command.DesignateTame,
		desc: "Mark animals for taming. Hunters coax them; a failed try can enrage the animal. Tame wolves guard the colony."},
	{id: "stockpile", label: "Stockpile", category: "zones", icon: "zone_stockpile", mode: placeArea, cmd: command.DesignateStockpile,
		desc: "Storage area. Haulers bring food and wood here."},
	{id: "grow", label: "Growing zone", category: "zones", icon: "zone_grow", mode: placeArea, cmd: command.DesignateGrowZone,
		crop: "potato", desc: "Pick a crop or trees below. Colonists sow it and harvest when ripe; crops grow in daylight, faster on rich soil."},
	{id: "home", label: "Expand home area", category: "zones", icon: "home", mode: placeArea, cmd: command.DesignateHome, enable: true,
		desc: "Paint the Home area: colonists sweep dirt off these tiles. It grows around new buildings."},
	{id: "unhome", label: "Clear home area", category: "zones", icon: "home_clear", mode: placeArea, cmd: command.DesignateHome,
		desc: "Take tiles out of the Home area so colonists leave them dirty."},
	{id: "roof", label: "Build roof area", category: "zones", icon: "roof_area", mode: placeArea, cmd: command.DesignateRoof,
		desc: "Colonists roof these tiles and rebuild the roof if it falls. A roof needs a wall within 6 tiles; closed rooms are roofed automatically."},
	{id: "noroof", label: "Remove roof area", category: "zones", icon: "roof_remove", mode: placeArea, cmd: command.DesignateRemoveRoof,
		desc: "Take roofs down and keep them off, even in closed rooms."},
	{id: "snow_clear", label: "Snow clearing area", category: "zones", icon: "snow_clear", mode: placeArea, cmd: command.DesignateSnowClear, enable: true,
		desc: "Paint where colonists shovel snow away once it lies 15% deep: paths, yards, a landing spot. Cleaning work."},
	{id: "unsnow_clear", label: "Clear snow clearing area", category: "zones", icon: "snow_clear", mode: placeArea, cmd: command.DesignateSnowClear,
		desc: "Take tiles out of the snow clearing area; their snow is left to lie."},
	{id: "allowed_area", label: "Expand allowed area >", category: "zones", icon: "colonist", mode: placeArea, cmd: command.PaintArea, enable: true,
		desc: "Pick an allowed area, then drag to paint it. Pawns restricted to it keep to it; Manage allowed areas makes, renames and deletes them."},
	{id: "allowed_area_clear", label: "Clear allowed area >", category: "zones", icon: "home_clear", mode: placeArea, cmd: command.PaintArea,
		desc: "Pick an allowed area, then drag to take tiles out of it."},
	{id: "wall", label: "Wall", category: "structure", icon: "wall", mode: placeLine, build: "wall",
		desc: "Blocks movement. Drag to draw a line."},
	{id: "fence", label: "Fence", category: "structure", icon: "fence", mode: placeLine, build: "fence",
		desc: "Animals can't cross; colonists climb over (slow). Drag to draw a line."},
	{id: "fence_gate", label: "Fence gate", category: "structure", icon: "fence_gate", mode: placeSingle, build: "fence gate",
		desc: "Animals can't cross; colonists climb over (slow). A gate lets colonists and tame animals through. Faces along the fence by itself."},
	{id: "door", label: "Door", category: "structure", icon: "door", mode: placeSingle, build: "door",
		desc: "Lets colonists through a wall. Faces along the wall by itself."},
	{id: "floor", label: "Wood floor", category: "structure", icon: "floor", mode: placeFill, build: "floor",
		desc: "Drag to cover an area. Furniture can stand on it."},
	{id: "bridge", label: "Bridge", category: "structure", icon: "bridge", mode: placeFill, build: "bridge",
		desc: "A wooden deck over water, marsh or mud. Holds light things only: no stone walls or floors. Drag to cover an area."},
	{id: "bed", label: "Bed", category: "furniture", icon: "bed", mode: placeSingle, build: "bed",
		desc: "A proper two-tile bed. Q/E rotates."},
	{id: "double_bed", label: "Double bed", category: "furniture", icon: "double_bed", mode: placeSingle, build: "double bed",
		desc: "A 2x2 bed two colonists share side by side. Q/E rotates."},
	{id: "hospital_bed", label: "Hospital bed", category: "furniture", icon: "hospital_bed", mode: placeSingle, build: "hospital bed",
		desc: "A bed for patients: the sick and hurt go here first and are tended better. Q/E rotates."},
	{id: "spot", label: "Sleeping spot", category: "furniture", icon: "sleeping_spot", mode: placeSingle, build: "sleeping spot",
		desc: "Marks a bed-sized place to sleep on the floor. Free and instant; a bed is more comfortable. Q/E rotates."},
	{id: "table", label: "Table", category: "furniture", icon: "table", mode: placeSingle, build: "table",
		desc: "A place to eat and work."},
	{id: "chair", label: "Chair", category: "furniture", icon: "chair", mode: placeSingle, build: "chair",
		desc: "Seat for a table. Q/E rotates."},
	{id: "horseshoes", label: "Horseshoe pin", category: "furniture", icon: "horseshoe_pin", mode: placeSingle, build: "horseshoe pin",
		desc: "A game of horseshoes: the best free-time fun, keeps colonists from getting bored."},
	{id: "carpet", label: "Carpet", category: "furniture", icon: "carpet", mode: placeSingle, build: "carpet",
		desc: "Wipes dirt off boots. Lay it on the tile just inside a door. Pick a carpet below."},
	{id: "lamp", label: "Lamp", category: "furniture", icon: "lamp", mode: placeSingle, build: "lamp",
		desc: "An electric light: lights its room at night while it has power. Needs a conduit within 3 tiles."},
	{id: "torch_lamp", label: "Torch lamp", category: "furniture", icon: "torch_lamp", mode: placeSingle, build: torchLampKind,
		desc: "Burns wood to light its room; no power needed. Haulers keep it fuelled, and rain or snow puts it out outdoors."},
	{id: "trough", label: "Feeding trough", category: "furniture", icon: "feeding_trough", mode: placeSingle, build: "feeding trough",
		desc: "Pen animals eat from it when the grass is gone. Haulers fill it; choose plants, meat or any when you select it. Q/E rotates."},
	{id: "workbench", label: "Workbench", category: "production", icon: "workbench", mode: placeSingle, build: "workbench",
		desc: "Crafting station (2x1). Select it to add bills. Q/E rotates."},
	{id: "stove", label: "Stove", category: "production", icon: "stove", mode: placeSingle, build: "stove",
		desc: "Cooks meals from raw food (2x1). Select it to add bills."},
	{id: "electric_stove", label: "Electric stove", category: "production", icon: "electric_stove", mode: placeSingle, build: "electric stove",
		desc: "Cooks the stove's meals 40% faster while it has power, and nothing without (2x1). Select it to add bills. Q/E rotates."},
	{id: "butcher", label: "Butcher table", category: "production", icon: "butcher_table", mode: placeSingle, build: "butcher table",
		desc: "Butchers hunted carcasses into meat (2x1). Starts with butcher bills."},
	{id: "campfire", label: "Campfire", category: "furniture", icon: "campfire", mode: placeSingle, build: "campfire",
		desc: "Burns wood to heat and light its room, and cooks simple meals (slowly). Haulers keep it fuelled."},
	{id: "grave", label: "Grave", category: "furniture", icon: "stone", mode: placeSingle, build: "grave",
		desc: "Dig a grave: colonists bury the dead here."},
	{id: "pyre", label: "Funeral pyre", category: "furniture", icon: "campfire", mode: placeSingle, build: "funeral pyre",
		desc: "Funeral pyre: burn the dead (2 wood per body)."},
	{id: "tailor", label: "Tailoring bench", category: "production", icon: "tailoring_bench", mode: placeSingle, build: "tailoring bench",
		desc: "Sews leather into parkas, tuques and dusters (2x1). Select it to add bills."},
	{id: "electric_tailor", label: "Electric tailoring bench", category: "production", icon: "electric_tailoring_bench", mode: placeSingle, build: "electric tailoring bench",
		desc: "Sews the tailoring bench's clothes 40% faster while it has power, and nothing without (2x1). Select it to add bills. Q/E rotates."},
	{id: "drying", label: "Drying rack", category: "production", icon: "drying_rack", mode: placeSingle, build: "drying rack",
		desc: "Dries up to 25 perishable food at a time so it never rots (2x1). Starts with a Dry food bill."},
	{id: "stonecutter", label: "Stonecutter table", category: "production", icon: "stonecutter_table", mode: placeSingle, build: "stonecutter table",
		desc: "Cuts stone chunks into stone blocks for building (2x1). Select it to add bills. Q/E rotates."},
	{id: "research", label: "Research bench", category: "production", icon: "research_bench", mode: placeSingle, build: "research bench",
		desc: "A desk to study at (2x1). Pick a project in the Research tab; colonists with Research work sit here. Q/E rotates."},
	{id: "electric_research", label: "Electric research bench", category: "production", icon: "electric_research_bench", mode: placeSingle, build: "electric research bench",
		desc: "A powered desk: research goes 40% faster here while it has power, and stops without (2x1). Q/E rotates."},
	{id: "tool_cabinet", label: "Tool cabinet", category: "production", icon: "tool_cabinet", mode: placeSingle, build: "tool cabinet",
		desc: "Tools on hand: benches within 6 tiles work 6% faster (two cabinets count)."},
	{id: "sandbags", label: "Sandbags", category: "security", icon: "sandbags", mode: placeLine, build: "sandbags",
		desc: "Waist-high cover. Colonists right behind them are much harder to shoot. Drag to draw a line."},
	{id: "spike_trap", label: "Spike trap", category: "security", icon: "spike_trap", mode: placeSingle, build: "spike trap",
		desc: "Springs on raiders and predators that walk over it, then is re-laid as a blueprint. Colonists step around it safely."},
	{id: "brewing_bench", label: "Brewing bench", category: "production", icon: "brewing_bench", mode: placeSingle, build: "brewing bench",
		desc: "Brews brewgrain into beer (2x1). Select it to add bills. Q/E rotates."},
	{id: "drug_table", label: "Drug table", category: "production", icon: "drug_table", mode: placeSingle, build: "drug table",
		desc: "Steeps ashleaf into tea and presses spark tablets (2x1). Select it to add bills. Q/E rotates."},
	{id: "marriage_spot", label: "Marriage spot", category: "misc", icon: "marriage_spot", mode: placeSingle, build: "marriage spot",
		desc: "Where engaged couples hold their wedding (2x1). Free and instant. Q/E rotates."},
	{id: "pen_marker", label: "Pen marker", category: "misc", icon: "pen_marker", mode: placeSingle, build: "pen marker",
		desc: "Marks a pen: the area fenced in around it. Fence it in, then pick which animals it takes, its food and plants to cut when you select it."},
	{id: "party_spot", label: "Party spot", category: "misc", icon: "party_spot", mode: placeSingle, build: "party spot",
		desc: "Where the colony gathers for parties. Free and instant."},
	// PLAY-021 temperature (Misc: see archCatTemperature).
	{id: "heater", label: "Heater", category: "misc", icon: "heater", mode: placeSingle, build: heaterKind,
		desc: "Warms its room up to its thermostat target, then idles. Select it to set the target."},
	{id: "cooler", label: "Cooler", category: "misc", icon: "cooler", mode: placeSingle, build: coolerKind,
		desc: "Set into a wall gap, facing the room to cool: it pumps the heat out of its back. Select it to set the target. Q/E rotates."},
	// PLAY-021 power: the grid's pieces.
	{id: "conduit", label: "Power conduit", category: archCatPower, icon: "power_conduit", mode: placeLine, build: conduitKind,
		desc: "Carries power along the line; generators, batteries and devices within 3 tiles join it. Drag to draw a line."},
	{id: "hidden_conduit", label: "Hidden conduit", category: archCatPower, icon: "hidden_conduit", mode: placeLine, build: hiddenConduitKind,
		desc: "A conduit buried out of sight: it carries power the same way, shows only in the power overlay and never shorts out. Drag to draw a line."},
	{id: "power_switch", label: "Power switch", category: archCatPower, icon: "power_switch", mode: placeSingle, build: powerSwitchKind,
		desc: "Joins the conduits on either side while on; switched off, it cuts the grid in two there. A colonist flicks it."},
	{id: "wood_generator", label: "Wood-fired generator", category: archCatPower, icon: "wood_fired_generator", mode: placeSingle, build: woodGeneratorKind,
		desc: "Burns wood for steady power, day and night (2x2). Haulers keep it fuelled."},
	{id: "solar_panel", label: "Solar panel", category: archCatPower, icon: "solar_panel", mode: placeSingle, build: solarPanelKind,
		desc: "Turns daylight into power: most at noon, nothing at night or under a roof (3x3)."},
	{id: "wind_turbine", label: "Wind turbine", category: archCatPower, icon: "wind_turbine", mode: placeSingle, build: windTurbineKind,
		desc: "Power from the wind, more in a gale (1x2). Walls, rock and trees close by steal its wind. Q/E rotates."},
	{id: "geothermal", label: "Geothermal generator", category: archCatPower, icon: "geothermal_generator", mode: placeSingle, build: geothermalKind,
		desc: "Built over a steam vent: steady power day and night, no fuel (2x2)."},
	{id: "battery", label: "Battery", category: archCatPower, icon: "battery", mode: placeSingle, build: batteryKind,
		desc: "Stores the grid's spare power for the night and lean hours, and slowly leaks it (1x2). Q/E rotates."},
}

// allArchTools is every architect tool.
func allArchTools() []archTool { return archTools }

func toolByID(id string) (archTool, bool) {
	for _, t := range allArchTools() {
		if t.id == id {
			return t, true
		}
	}
	return archTool{}, false
}

func toolsIn(category string) []archTool {
	var out []archTool
	for _, t := range allArchTools() {
		if t.category == category {
			out = append(out, t)
		}
	}
	return out
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

func buildable(kind string) (game.BuildableView, bool) {
	b, ok := buildableCatalog()[kind]
	return b, ok
}

// archPowerLine is an electrical card's power line, from the simulation's
// own numbers: what a consumer draws, a producer makes at most or a
// battery holds. "" for conduits and switches (they only carry power) and
// for everything that is not electrical.
func archPowerLine(bv game.BuildableView) string {
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
	switch kind {
	case fenceKind:
		return fenceSprite(stuff, mask)
	case gateKind:
		return gateSprite(facing, open)
	}
	if kind == "carpet" {
		return carpetSprite(stuff)
	}
	switch m := stuffMaterial(stuff); {
	case kind == "wall" && m != "wood":
		return wallPrefix(stuff) + "_" + twoDigits(mask)
	case kind == "floor" && m == "stone":
		return "structure/floor_stone"
	case kind == "door" && m == "steel":
		return doorSprite("steel", ew, open)
	}
	return buildSprite(kind, facing, mask, ew, open)
}

// carpetSprite is furniture/carpet_<straw|cloth|leather|wood> for a carpet
// material ("cloth carpet" or just "cloth"); straw when unknown.
func carpetSprite(stuff string) string {
	m := strings.TrimSuffix(stuff, " carpet")
	switch m {
	case "straw", "cloth", "leather", "wood", "wool":
		return "furniture/carpet_" + m
	}
	return "furniture/carpet_straw"
}

// buildSprite is the world sprite for a building kind; neighbours only
// matter for walls and conduits (autotile) and doors (axis). open draws
// doors open and a power switch off.
func buildSprite(kind, facing string, mask int, ew, open bool) string {
	switch kind {
	case fenceKind:
		return fenceSprite("wood", mask)
	case gateKind:
		return gateSprite(facing, open)
	case "wall":
		return "structure/wall_wood_" + twoDigits(mask)
	case "door":
		return doorSprite("wood", ew, open)
	case "floor":
		return "structure/floor_wood"
	case bridgeKind:
		return bridgeKey(mask)
	case "double bed":
		return "furniture/bed_double_" + facingOr(facing)
	case "bed":
		return "furniture/bed_basic_" + facingOr(facing)
	case "sleeping spot":
		return "overlay/sleeping_spot_" + facingOr(facing)
	case "marriage spot":
		return "overlay/marriage_spot_" + facingOr(facing)
	case "party spot":
		return "overlay/party_spot"
	case "brewing bench":
		return "furniture/brewing_bench_" + facingOr(facing)
	case "drug table":
		return "furniture/drug_table_" + facingOr(facing)
	case "workbench":
		return "furniture/workbench_basic_" + facingOr(facing)
	case "stove":
		return "furniture/stove_basic_" + facingOr(facing)
	case "butcher table":
		return "furniture/butcher_table_" + facingOr(facing)
	case "campfire":
		return "furniture/campfire"
	case "grave":
		return "furniture/grave_dug_" + map[bool]string{true: "east", false: "south"}[facing == "east" || facing == "west"]
	case "funeral pyre":
		return "furniture/pyre"
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
	case "carpet":
		return carpetSprite("")
	case "feeding trough":
		return "furniture/feeding_trough_empty_" + facingOr(facing)
	case "stonecutter table":
		return "furniture/stonecutter_table_" + facingOr(facing)
	case "tool cabinet":
		return "furniture/tool_cabinet"
	case "hospital bed":
		return "furniture/hospital_bed_" + facingOr(facing)
	case "pod shell":
		return "structure/pod_shell"
	case steamVentKind:
		return "structure/steam_vent"
	case "pen marker":
		return "structure/pen_marker"
	}
	return powerSprite(kind, facing, mask, open)
}

// powerSprite is buildSprite for the PLAY-021 kinds: conduits by link
// mask (power_draw.go), a switch on or (open circuit) off, the rest by
// facing where they rotate.
func powerSprite(kind, facing string, mask int, open bool) string {
	switch kind {
	case conduitKind:
		return "power/power_conduit_" + twoDigits(mask&15)
	case hiddenConduitKind:
		return "power/hidden_conduit_" + twoDigits(mask&15)
	case powerSwitchKind:
		if open {
			return "power/power_switch_off"
		}
		return "power/power_switch_on"
	case woodGeneratorKind:
		return "power/wood_fired_generator"
	case solarPanelKind:
		return "power/solar_panel"
	case windTurbineKind:
		return "power/wind_turbine_" + facingOr(facing)
	case geothermalKind:
		return "power/geothermal_generator"
	case batteryKind:
		return "power/battery_" + facingOr(facing)
	case torchLampKind:
		return "furniture/torch_lamp"
	case heaterKind:
		return "power/heater"
	case coolerKind:
		return "power/cooler_" + facingOr(facing)
	case "electric stove":
		return "furniture/electric_stove_" + facingOr(facing)
	case "electric tailoring bench":
		return "furniture/electric_tailoring_bench_" + facingOr(facing)
	case "electric research bench":
		return "furniture/electric_research_bench_" + facingOr(facing)
	}
	return ""
}

// doorSprite is structure/door_<material>_<ns|ew>_<open|closed>.
func doorSprite(material string, ew, open bool) string {
	axis, state := "ns", "closed"
	if ew {
		axis = "ew"
	}
	if open {
		state = "open"
	}
	return "structure/door_" + material + "_" + axis + "_" + state
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
