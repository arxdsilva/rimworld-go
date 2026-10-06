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
}

var archCategories = []struct{ id, label string }{
	{"orders", "Orders"},
	{"zones", "Zones"},
	{"structure", "Structure"},
	{"furniture", "Furniture"},
}

var archTools = []archTool{
	{id: "harvest", label: "Harvest", category: "orders", icon: "order_harvest", mode: placeArea, cmd: command.DesignateHarvest,
		desc: "Mark ripe berry bushes for harvesting."},
	{id: "deconstruct", label: "Deconstruct", category: "orders", icon: "order_deconstruct", mode: placeArea, cmd: command.DesignateDeconstruct,
		desc: "Take buildings down. Refunds half the wood."},
	{id: "cancel", label: "Cancel", category: "orders", icon: "order_cancel", mode: placeArea, cmd: command.DesignateCancel,
		desc: "Remove blueprints, zones and orders in an area."},
	{id: "stockpile", label: "Stockpile", category: "zones", icon: "zone_stockpile", mode: placeArea, cmd: command.DesignateStockpile,
		desc: "Storage area. Haulers bring food and wood here."},
	{id: "wall", label: "Wall", category: "structure", icon: "wall", mode: placeLine, build: "wall",
		desc: "Blocks movement. Drag to draw a line."},
	{id: "door", label: "Door", category: "structure", icon: "door", mode: placeSingle, build: "door",
		desc: "Lets colonists through a wall."},
	{id: "floor", label: "Wood floor", category: "structure", icon: "floor", mode: placeFill, build: "floor",
		desc: "Drag to cover an area. Furniture can stand on it."},
	{id: "bed", label: "Bed", category: "furniture", icon: "bed", mode: placeSingle, build: "bed",
		desc: "Colonists sleep better in a bed. Q/E rotates."},
	{id: "table", label: "Table", category: "furniture", icon: "table", mode: placeSingle, build: "table",
		desc: "A place to eat and work."},
	{id: "chair", label: "Chair", category: "furniture", icon: "chair", mode: placeSingle, build: "chair",
		desc: "Seat for a table. Q/E rotates."},
	{id: "lamp", label: "Lamp", category: "furniture", icon: "lamp", mode: placeSingle, build: "lamp",
		desc: "Decorative light (power comes later)."},
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
	case "bed":
		return "furniture/bed_cot_" + facingOr(facing)
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
