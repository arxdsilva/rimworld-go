package architect

import (
	"fmt"
	"sync"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/sim/command"
)

// Architect tool catalog (MAIN_UI_SCREENS §17, UI checklist §5). Only
// categories with content are shown. Buildable costs come from the
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
}

// PLAY-021 categories: Power (conduits, switch, generators, battery). The
// category panel holds four rows of two (hud_layout.go archCatsH), so a
// ninth category does not fit: the heater and cooler sit in Misc and the
// campfire stays in Furniture. CatTemperature stays unlisted until
// the panel grows a row; every listed category gets a button.
const (
	CatPower       = "power"
	CatTemperature = "temperature"
)

// Category is one button of the category panel.
type Category struct{ ID, Label string }

// Categories are the category panel's buttons, in order.
var Categories = []Category{
	{"orders", "Orders"},
	{"zones", "Zones"},
	{"structure", "Structure"},
	{"furniture", "Furniture"},
	{"production", "Production"},
	{CatPower, "Power"}, // PLAY-021
	{"security", "Security"},
	{"misc", "Misc"}, // ENG-003: marriage and party spots; PLAY-021: heater and cooler
}

// Tools is the tool catalog, in card order.
var Tools = []ToolDef{
	{ID: "harvest", Label: "Harvest", Category: "orders", Icon: "order_harvest", Mode: PlaceArea, Cmd: command.DesignateHarvest,
		Desc: "Mark ripe berry bushes for harvesting."},
	{ID: "deconstruct", Label: "Deconstruct", Category: "orders", Icon: "order_deconstruct", Mode: PlaceArea, Cmd: command.DesignateDeconstruct,
		Desc: "Take buildings down. Refunds half the wood."},
	{ID: "cancel", Label: "Cancel", Category: "orders", Icon: "order_cancel", Mode: PlaceArea, Cmd: command.DesignateCancel,
		Desc: "Remove blueprints, zones and orders in an area."},
	{ID: "cut", Label: "Cut plants", Category: "orders", Icon: "order_cut", Mode: PlaceArea, Cmd: command.DesignateCut,
		Desc: "Chop trees for wood or clear bushes. Grass too."},
	{ID: "mine", Label: "Mine", Category: "orders", Icon: "order_mine", Mode: PlaceArea, Cmd: command.DesignateMine,
		Desc: "Dig out rock. Each tile drops a stone chunk; ore veins drop metal."},
	{ID: "smooth", Label: "Smooth stone", Category: "orders", Icon: "order_smooth", Mode: PlaceArea, Cmd: command.DesignateSmooth,
		Desc: "Smooth rock walls and rough stone ground into clean walls and floors. Not on ore."},
	{ID: "plan", Label: "Plan", Category: "orders", Icon: "plan", Mode: PlaceArea, Cmd: command.DesignatePlan, Enable: true,
		Desc: "Sketch a layout: paint placeholder marks. Nobody works on them and they block nothing; a finished building clears its mark."},
	{ID: "unplan", Label: "Remove plan", Category: "orders", Icon: "plan_clear", Mode: PlaceArea, Cmd: command.DesignatePlan,
		Desc: "Erase plan marks in an area."},
	{ID: "forbid", Label: "Forbid", Category: "orders", Icon: "order_forbid", Mode: PlaceArea, Cmd: command.DesignateForbid, Enable: true,
		Desc: "Colonists leave forbidden items alone. Drag over the items."},
	{ID: "allow", Label: "Allow", Category: "orders", Icon: "order_allow", Mode: PlaceArea, Cmd: command.DesignateForbid,
		Desc: "Let colonists use forbidden items again. Drag over the items."},
	{ID: "haul", Label: "Haul things", Category: "orders", Icon: "order_haul", Mode: PlaceArea, Cmd: command.DesignateHaul, Enable: true,
		Desc: "Mark things found on the map for hauling. Colonists haul the colony's own goods by themselves; anything else waits for this mark."},
	{ID: "hunt", Label: "Hunt", Category: "orders", Icon: "order_hunt", Mode: PlaceArea, Cmd: command.DesignateHunt,
		Desc: "Mark animals for hunters. Hunters need a rifle; kills drop meat."},
	{ID: "tame", Label: "Tame", Category: "orders", Icon: "order_tame", Mode: PlaceArea, Cmd: command.DesignateTame,
		Desc: "Mark animals for taming. Hunters coax them; a failed try can enrage the animal. Tame wolves guard the colony."},
	{ID: "stockpile", Label: "Stockpile", Category: "zones", Icon: "zone_stockpile", Mode: PlaceArea, Cmd: command.DesignateStockpile,
		Desc: "Storage area. Haulers bring food, wood, stone and goods here, filling higher-priority stockpiles first. Not on deep water, walls or the map edge."},
	{ID: "dump", Label: "Dumping stockpile", Category: "zones", Icon: "zone_stockpile", Mode: PlaceArea, Cmd: command.DesignateStockpile,
		Variant: game.StockpileVariantDump, Desc: "A place to dump animal corpses and stone chunks, out of the way. Same ground rules as a stockpile."},
	{ID: "grow", Label: "Growing zone", Category: "zones", Icon: "zone_grow", Mode: PlaceArea, Cmd: command.DesignateGrowZone,
		Crop: "potato", Desc: "Pick a crop or trees below. Colonists sow it and harvest when ripe; crops grow in daylight, faster on rich soil. Soil only: not sand, mud, stone, floors or bridges."},
	{ID: "home", Label: "Expand home area", Category: "zones", Icon: "home", Mode: PlaceArea, Cmd: command.DesignateHome, Enable: true,
		Desc: "Paint the Home area: colonists sweep dirt off these tiles. It grows around new buildings."},
	{ID: "unhome", Label: "Clear home area", Category: "zones", Icon: "home_clear", Mode: PlaceArea, Cmd: command.DesignateHome,
		Desc: "Take tiles out of the Home area so colonists leave them dirty."},
	{ID: "roof", Label: "Build roof area", Category: "zones", Icon: "roof_area", Mode: PlaceArea, Cmd: command.DesignateRoof,
		Desc: "Colonists roof these tiles and rebuild the roof if it falls. A roof needs a wall within 6 tiles; closed rooms are roofed automatically."},
	{ID: "noroof", Label: "Remove roof area", Category: "zones", Icon: "roof_remove", Mode: PlaceArea, Cmd: command.DesignateRemoveRoof,
		Desc: "Take roofs down and keep them off, even in closed rooms."},
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
	{ID: "floor", Label: "Wood floor", Category: "structure", Icon: "floor", Mode: PlaceFill, Build: "floor",
		Desc: "Drag to cover an area. Furniture can stand on it."},
	{ID: "bridge", Label: "Bridge", Category: "structure", Icon: "bridge", Mode: PlaceFill, Build: "bridge",
		Desc: "A wooden deck over water, marsh or mud. Holds light things only: no stone walls or floors. Drag to cover an area."},
	{ID: "bed", Label: "Bed", Category: "furniture", Icon: "bed", Mode: PlaceSingle, Build: "bed",
		Desc: "A proper two-tile bed. Q/E rotates."},
	{ID: "double_bed", Label: "Double bed", Category: "furniture", Icon: "double_bed", Mode: PlaceSingle, Build: "double bed",
		Desc: "A 2x2 bed two colonists share side by side. Q/E rotates."},
	{ID: "hospital_bed", Label: "Hospital bed", Category: "furniture", Icon: "hospital_bed", Mode: PlaceSingle, Build: "hospital bed",
		Desc: "A bed for patients: the sick and hurt go here first and are tended better. Q/E rotates."},
	{ID: "vitals_monitor", Label: "Vitals monitor", Category: "furniture", Icon: "vitals_monitor", Mode: PlaceSingle, Build: "vitals monitor",
		Desc: "Set it beside a hospital bed: while powered (80 W), the patient there is tended 7% better, operated on 5% more safely and gains immunity 2% faster. One per bed. Q/E rotates."},
	{ID: "spot", Label: "Sleeping spot", Category: "furniture", Icon: "sleeping_spot", Mode: PlaceSingle, Build: "sleeping spot",
		Desc: "Marks a bed-sized place to sleep on the floor. Free and instant; a bed is more comfortable. Q/E rotates."},
	{ID: "table", Label: "Table", Category: "furniture", Icon: "table", Mode: PlaceSingle, Build: "table",
		Desc: "A place to eat and work."},
	{ID: "chair", Label: "Chair", Category: "furniture", Icon: "chair", Mode: PlaceSingle, Build: "chair",
		Desc: "Seat for a table. Q/E rotates."},
	{ID: "horseshoes", Label: "Horseshoe pin", Category: "furniture", Icon: "horseshoe_pin", Mode: PlaceSingle, Build: "horseshoe pin",
		Desc: "A game of horseshoes: the best free-time fun, keeps colonists from getting bored."},
	{ID: "carpet", Label: "Carpet", Category: "furniture", Icon: "carpet", Mode: PlaceSingle, Build: "carpet",
		Desc: "Wipes dirt off boots. Lay it on the tile just inside a door. Pick a carpet below."},
	{ID: "lamp", Label: "Lamp", Category: "furniture", Icon: "lamp", Mode: PlaceSingle, Build: "lamp",
		Desc: "An electric light: lights its room at night while it has power. Needs a conduit within 3 tiles."},
	{ID: "torch_lamp", Label: "Torch lamp", Category: "furniture", Icon: "torch_lamp", Mode: PlaceSingle, Build: overlay.TorchLampKind,
		Desc: "Burns wood to light its room; no power needed. Haulers keep it fuelled, and rain or snow puts it out outdoors."},
	{ID: "trough", Label: "Feeding trough", Category: "furniture", Icon: "feeding_trough", Mode: PlaceSingle, Build: "feeding trough",
		Desc: "Pen animals eat from it when the grass is gone. Haulers fill it; choose plants, meat or any when you select it. Q/E rotates."},
	{ID: "workbench", Label: "Workbench", Category: "production", Icon: "workbench", Mode: PlaceSingle, Build: "workbench",
		Desc: "Crafting station (2x1). Select it to add bills. Q/E rotates."},
	{ID: "stove", Label: "Stove", Category: "production", Icon: "stove", Mode: PlaceSingle, Build: "stove",
		Desc: "Cooks meals from raw food (2x1). Select it to add bills."},
	{ID: "electric_stove", Label: "Electric stove", Category: "production", Icon: "electric_stove", Mode: PlaceSingle, Build: "electric stove",
		Desc: "Cooks the stove's meals 40% faster while it has power, and nothing without (2x1). Select it to add bills. Q/E rotates."},
	{ID: "butcher", Label: "Butcher table", Category: "production", Icon: "butcher_table", Mode: PlaceSingle, Build: "butcher table",
		Desc: "Butchers hunted carcasses into meat (2x1). Starts with butcher bills."},
	{ID: "campfire", Label: "Campfire", Category: "furniture", Icon: "campfire", Mode: PlaceSingle, Build: "campfire",
		Desc: "Burns wood to heat and light its room, and cooks simple meals (slowly). Haulers keep it fuelled."},
	{ID: "grave", Label: "Grave", Category: "furniture", Icon: "stone", Mode: PlaceSingle, Build: "grave",
		Desc: "Dig a grave: colonists bury the dead here."},
	{ID: "pyre", Label: "Funeral pyre", Category: "furniture", Icon: "campfire", Mode: PlaceSingle, Build: "funeral pyre",
		Desc: "Funeral pyre: burn the dead (2 wood per body)."},
	{ID: "tailor", Label: "Tailoring bench", Category: "production", Icon: "tailoring_bench", Mode: PlaceSingle, Build: "tailoring bench",
		Desc: "Sews leather into parkas, tuques and dusters (2x1). Select it to add bills."},
	{ID: "electric_tailor", Label: "Electric tailoring bench", Category: "production", Icon: "electric_tailoring_bench", Mode: PlaceSingle, Build: "electric tailoring bench",
		Desc: "Sews the tailoring bench's clothes 40% faster while it has power, and nothing without (2x1). Select it to add bills. Q/E rotates."},
	{ID: "drying", Label: "Drying rack", Category: "production", Icon: "drying_rack", Mode: PlaceSingle, Build: "drying rack",
		Desc: "Dries up to 25 perishable food at a time so it never rots (2x1). Starts with a Dry food bill."},
	{ID: "stonecutter", Label: "Stonecutter table", Category: "production", Icon: "stonecutter_table", Mode: PlaceSingle, Build: "stonecutter table",
		Desc: "Cuts stone chunks into stone blocks for building (2x1). Select it to add bills. Q/E rotates."},
	{ID: "research", Label: "Research bench", Category: "production", Icon: "research_bench", Mode: PlaceSingle, Build: "research bench",
		Desc: "A desk to study at (2x1). Pick a project in the Research tab; colonists with Research work sit here. Q/E rotates."},
	{ID: "electric_research", Label: "Electric research bench", Category: "production", Icon: "electric_research_bench", Mode: PlaceSingle, Build: "electric research bench",
		Desc: "A powered desk: research goes 40% faster here while it has power, and stops without (2x1). Q/E rotates."},
	{ID: "tool_cabinet", Label: "Tool cabinet", Category: "production", Icon: "tool_cabinet", Mode: PlaceSingle, Build: "tool cabinet",
		Desc: "Tools on hand: benches within 6 tiles work 6% faster (two cabinets count)."},
	{ID: "sandbags", Label: "Sandbags", Category: "security", Icon: "sandbags", Mode: PlaceLine, Build: "sandbags",
		Desc: "Waist-high cover. Colonists right behind them are much harder to shoot. Drag to draw a line."},
	{ID: "spike_trap", Label: "Spike trap", Category: "security", Icon: "spike_trap", Mode: PlaceSingle, Build: "spike trap",
		Desc: "Springs on raiders and predators that walk over it, then is re-laid as a blueprint. Colonists step around it safely."},
	// PLAY-024: defence and trade.
	{ID: "sentry_turret", Label: "Sentry turret", Category: "security", Icon: "sentry_turret", Mode: PlaceSingle, Build: overlay.SentryTurretKind,
		Desc: "A powered gun post: shoots raiders and hostile animals within 10 tiles in line of sight. Shoots only while powered; select it to see its reach."},
	{ID: "radio_console", Label: "Radio console", Category: "misc", Icon: "radio_console", Mode: PlaceSingle, Build: overlay.RadioConsoleKind,
		Desc: "A powered desk (2x1): a colonist works it to call traders, gift silver and ask factions for help. Q/E rotates."},
	{ID: "trade_beacon", Label: "Trade beacon", Category: "misc", Icon: "trade_beacon", Mode: PlaceSingle, Build: overlay.TradeBeaconKind,
		Desc: "Marks the goods an orbital trader can buy: stacks within 6 tiles of it. Unpowered; the ghost shows its radius."},
	{ID: "brewing_bench", Label: "Brewing bench", Category: "production", Icon: "brewing_bench", Mode: PlaceSingle, Build: "brewing bench",
		Desc: "Brews brewgrain into beer (2x1). Select it to add bills. Q/E rotates."},
	{ID: "drug_table", Label: "Drug table", Category: "production", Icon: "drug_table", Mode: PlaceSingle, Build: "drug table",
		Desc: "Steeps ashleaf into tea and presses spark tablets (2x1). Select it to add bills. Q/E rotates."},
	{ID: "marriage_spot", Label: "Marriage spot", Category: "misc", Icon: "marriage_spot", Mode: PlaceSingle, Build: "marriage spot",
		Desc: "Where engaged couples hold their wedding (2x1). Free and instant. Q/E rotates."},
	{ID: "pen_marker", Label: "Pen marker", Category: "misc", Icon: "pen_marker", Mode: PlaceSingle, Build: "pen marker",
		Desc: "Marks a pen: the area fenced in around it. Fence it in, then pick which animals it takes, its food and plants to cut when you select it."},
	{ID: "party_spot", Label: "Party spot", Category: "misc", Icon: "party_spot", Mode: PlaceSingle, Build: "party spot",
		Desc: "Where the colony gathers for parties. Free and instant."},
	// PLAY-021 temperature (Misc: see CatTemperature).
	{ID: "heater", Label: "Heater", Category: "misc", Icon: "heater", Mode: PlaceSingle, Build: overlay.HeaterKind,
		Desc: "Warms its room up to its thermostat target, then idles. Select it to set the target."},
	{ID: "cooler", Label: "Cooler", Category: "misc", Icon: "cooler", Mode: PlaceSingle, Build: overlay.CoolerKind,
		Desc: "Set into a wall gap, facing the room to cool: it pumps the heat out of its back. Select it to set the target. Q/E rotates."},
	// PLAY-021 power: the grid's pieces.
	{ID: "conduit", Label: "Power conduit", Category: CatPower, Icon: "power_conduit", Mode: PlaceLine, Build: overlay.ConduitKind,
		Desc: "Carries power along the line; generators, batteries and devices within 3 tiles join it. Drag to draw a line."},
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
}

// AllTools is every architect tool.
func AllTools() []ToolDef { return Tools }

// ToolByID is the tool with id.
func ToolByID(id string) (ToolDef, bool) {
	for _, t := range AllTools() {
		if t.ID == id {
			return t, true
		}
	}
	return ToolDef{}, false
}

// ToolsIn is category's tools, in catalog order.
func ToolsIn(category string) []ToolDef {
	var out []ToolDef
	for _, t := range AllTools() {
		if t.Category == category {
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
		dx, dy := Abs(x1-x0), Abs(y1-y0)
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

// Abs is |v|.
func Abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
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
