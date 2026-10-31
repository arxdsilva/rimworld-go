package inspector

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/plant"
)

// Plant inspection (UI-011, MAIN_UI §10): what the plant is and how far
// it has grown (plantPane), and Harvest / Cut (or Chop) with "now" orders
// per colonist (plantCommands, cmds_thing.go).

func isTree(p game.PlantView) bool { return sprite.IsTreePlant(p.Species) }

func Capitalize(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// plantTitle names the plant: trees by their drawn variant, the rest by
// species.
func PlantTitle(p game.PlantView) string {
	switch p.Species {
	case "pine":
		return "Pine tree"
	case "jungle_tree":
		return "Jungle tree"
	}
	if isTree(p) {
		return Capitalize(env.TreeSpecies[env.TreeSpeciesAt(sprite.TileHash(p.X, p.Y))]) + " tree"
	}
	return Capitalize(p.Species)
}

// plantCategory is the header's category word.
func PlantCategory(p game.PlantView) string {
	if isTree(p) {
		return "Tree"
	}
	if _, ok := world.PlantKeys.Crops[p.Species]; ok || p.Zone != "" {
		return "Crop"
	}
	return "Wild plant"
}

// plantStageWord is the growth bar's label.
func PlantStageWord(p game.PlantView) string {
	if p.Harvestable && !isTree(p) {
		return "ready to harvest"
	}
	return [...]string{"sown", "sprouting", "young", "mature"}[world.ClampStage(p.Stage)]
}

// plantGrowCaption is the growth bar's caption (PLAY-017): the progress and
// why the plant is or isn't growing now, from the game's growth state
// (Simulation.PlantGrowth). "" without one.
func PlantGrowCaption(p game.PlantView, gv game.PlantGrowthView) string {
	var word string
	switch gv.State {
	case "growing":
		word = "growing"
	case "resting":
		word = "resting (night)"
	case "dark":
		return "too dark: needs sun or a lamp nearby"
	case "cold":
		word = "too cold"
	case "hot":
		word = "too hot"
	case "ripe":
		if plantYields(p) {
			return "ripe"
		}
		return "fully grown"
	default:
		return ""
	}
	return fmt.Sprintf("%d%% · %s", p.Growth, word)
}

// plantCaptionWarns reports a state that needs the player's attention (a
// roof, a lamp or a fire), drawn in the warning colour.
func plantCaptionWarns(gv game.PlantGrowthView) bool {
	switch gv.State {
	case "dark", "cold", "hot":
		return true
	}
	return false
}

// plantYields reports a plant that is picked when ripe (crops, berry
// bushes, reeds); the rest (trees, moss) are just full grown.
func plantYields(p game.PlantView) bool {
	sp, ok := plant.Parse(p.Species)
	return ok && plant.DefOf(sp).Yield > 0
}

// plantRipeLine is "Ripe in about N days" from the game's RipeIn estimate
// (one decimal under two days, whole days after); "" when the plant is not
// growing or is ready.
func PlantRipeLine(p game.PlantView, gv game.PlantGrowthView) string {
	if gv.RipeIn <= 0 || p.Harvestable {
		return ""
	}
	days := math.Round(math.Max(float64(gv.RipeIn)/clock.TicksPerDay, 0.1)*10) / 10
	n := strconv.Itoa(int(math.Round(days)))
	if days < 2 {
		n = strconv.FormatFloat(days, 'f', 1, 64)
	}
	if plantYields(p) {
		return "Ripe in about " + n + " days"
	}
	return "Fully grown in about " + n + " days"
}

// plantLines are the body lines under the growth bar that the plant's
// view alone gives (no ripening estimate: plantBodyLines).
func PlantLines(p game.PlantView) []string { return PlantBodyLines(p, game.PlantGrowthView{}) }

// plantBodyLines are the body lines under the growth bar, the ripening
// estimate from the growth state gv among them.
func PlantBodyLines(p game.PlantView, gv game.PlantGrowthView) []string {
	var out []string
	switch {
	case isTree(p):
	case p.Harvestable:
		out = append(out, "Ready to harvest")
	default:
		out = append(out, fmt.Sprintf("Not ready (%d%%)", p.Growth))
	}
	if l := PlantRipeLine(p, gv); l != "" {
		out = append(out, l)
	}
	if l := env.PlantDamageLine(p); l != "" { // crop_batch.go: cold or heat damage
		out = append(out, l)
	}
	if p.Wilted {
		out = append(out, "Wilting: too cold")
	}
	if slowsWalking(p) {
		out = append(out, "Slows walking")
	}
	if p.Zone != "" {
		out = append(out, "In a growing zone ("+Capitalize(p.Zone)+")")
	}
	out = append(out, FoodPlantLines(p)...)
	out = append(out, env.PlantLightLines(p)...) // cave_plants_draw.go
	if p.Designated {
		out = append(out, "Marked to harvest")
	}
	if p.Cut {
		out = append(out, "Marked to cut")
	}
	return out
}

// plantPaneLines are the pane's body lines: the growth caption, then the
// body lines under it.
func PlantPaneLines(p game.PlantView, gv game.PlantGrowthView) []string {
	var out []string
	if c := PlantGrowCaption(p, gv); c != "" {
		out = append(out, c)
	}
	return append(out, PlantBodyLines(p, gv)...)
}

// slowsWalking reports a plant whose cell costs extra step time (bramble,
// elephant ear), from its definition's WalkCost.
func slowsWalking(p game.PlantView) bool {
	sp, ok := plant.Parse(p.Species)
	return ok && plant.DefOf(sp).WalkCost > 100
}

// plantNowMatch picks which order the "now" chips issue.
func plantNowMatch(p game.PlantView) func(game.OrderOption) bool {
	if p.Harvestable && !isTree(p) {
		return OrderIs(command.OrderHarvest)
	}
	return OrderIs(command.OrderCut)
}

// plantMarkCmd marks or (with undo) clears the harvest or cut mark on the
// plant's tile only; it never cancels zones.
func PlantMarkCmd(p game.PlantView, harvest, undo bool) command.Command {
	c := command.Command{Kind: command.DesignateCut, X: p.X, Y: p.Y, X2: p.X, Y2: p.Y}
	if harvest {
		c.Kind = command.DesignateHarvest
	}
	if undo {
		c.Kind, c.Order = command.Undesignate, command.OrderCut
		if harvest {
			c.Order = command.OrderHarvest
		}
	}
	return c
}

// applyPlantClick handles a "plant.*" button.
func (a Thing) ApplyPlantClick(id string) {
	p, ok := a.sel.SelectedPlant()
	if !ok {
		return
	}
	switch {
	case id == "plant.close":
		a.sel.ClearPlant()
	case id == "plant.harvest" && !isTree(p):
		if p.Designated || p.Harvestable {
			a.shell.Enqueue(PlantMarkCmd(p, true, p.Designated))
		}
	case id == "plant.cut":
		a.shell.Enqueue(PlantMarkCmd(p, false, p.Cut))
	case id == CutBlightID: // FOOD-5: every blighted plant on the map
		for _, c := range CutBlightCmds(a.shell.World().Plants) {
			a.shell.Enqueue(c)
		}
	}
}

// FoodPlantLines are the FOOD-2 readouts of a plant: the soil's fertility
// (a hydroponics basin reads 280%), the growing light it last had, the
// share of its lifespan gone (a tree's or a perennial's), and blight.
// Fields the simulation has not filled (0) give no line.
func FoodPlantLines(p game.PlantView) []string {
	var out []string
	if p.Fertility > 0 {
		l := fmt.Sprintf("Fertility %d%%", p.Fertility)
		if p.Fertility >= 280 {
			l += " (hydroponics)"
		}
		out = append(out, l)
	}
	if p.Light > 0 {
		out = append(out, fmt.Sprintf("Growing light %d%%", p.Light))
	}
	if p.Lifespan > 0 {
		out = append(out, fmt.Sprintf("Lifespan %d%% used", p.Lifespan))
	}
	if p.Blighted {
		out = append(out, "Blighted: growth stopped, it spreads to plants within 4 tiles. Cut it down.")
	}
	return out
}

// CutBlightID is the plant pane's Cut all blight button.
const CutBlightID = "plant.cutblight"

// CutBlightCmds are the cut designations for every blighted plant not yet
// marked (the existing DesignateCut, one per plant).
func CutBlightCmds(plants []game.PlantView) []command.Command {
	var out []command.Command
	for _, q := range plants {
		if q.Blighted && !q.Cut {
			out = append(out, PlantMarkCmd(q, false, false))
		}
	}
	return out
}
