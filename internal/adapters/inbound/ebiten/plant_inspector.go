package ebiten

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/plant"
)

// Plant inspection (UI-011, MAIN_UI §10): what the plant is and how far
// it has grown (plantPane), and Harvest / Cut (or Chop) with "now" orders
// per colonist (plantCommands, cmds_thing.go).

func isTree(p game.PlantView) bool { return isTreePlant(p.Species) }

func capitalize(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// plantTitle names the plant: trees by their drawn variant, the rest by
// species.
func plantTitle(p game.PlantView) string {
	switch p.Species {
	case "pine":
		return "Pine tree"
	case "jungle_tree":
		return "Jungle tree"
	}
	if isTree(p) {
		return capitalize(treeSpecies[treeSpeciesAt(tileHash(p.X, p.Y))]) + " tree"
	}
	return capitalize(p.Species)
}

// plantCategory is the header's category word.
func plantCategory(p game.PlantView) string {
	if isTree(p) {
		return "Tree"
	}
	if _, ok := plantKeys.Crops[p.Species]; ok || p.Zone != "" {
		return "Crop"
	}
	return "Wild plant"
}

// plantStageWord is the growth bar's label.
func plantStageWord(p game.PlantView) string {
	if p.Harvestable && !isTree(p) {
		return "ready to harvest"
	}
	return [...]string{"sown", "sprouting", "young", "mature"}[clampStage(p.Stage)]
}

// plantGrowCaption is the growth bar's caption (PLAY-017): the progress and
// why the plant is or isn't growing now, from the game's growth state
// (Simulation.PlantGrowth). "" without one.
func plantGrowCaption(p game.PlantView, gv game.PlantGrowthView) string {
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
func plantRipeLine(p game.PlantView, gv game.PlantGrowthView) string {
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
func plantLines(p game.PlantView) []string { return plantBodyLines(p, game.PlantGrowthView{}) }

// plantBodyLines are the body lines under the growth bar, the ripening
// estimate from the growth state gv among them.
func plantBodyLines(p game.PlantView, gv game.PlantGrowthView) []string {
	var out []string
	switch {
	case isTree(p):
	case p.Harvestable:
		out = append(out, "Ready to harvest")
	default:
		out = append(out, fmt.Sprintf("Not ready (%d%%)", p.Growth))
	}
	if l := plantRipeLine(p, gv); l != "" {
		out = append(out, l)
	}
	if l := plantDamageLine(p); l != "" { // crop_batch.go: cold or heat damage
		out = append(out, l)
	}
	if p.Wilted {
		out = append(out, "Wilting: too cold")
	}
	if slowsWalking(p) {
		out = append(out, "Slows walking")
	}
	if p.Zone != "" {
		out = append(out, "In a growing zone ("+capitalize(p.Zone)+")")
	}
	out = append(out, plantLightLines(p)...) // cave_plants_draw.go
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
func plantPaneLines(p game.PlantView, gv game.PlantGrowthView) []string {
	var out []string
	if c := plantGrowCaption(p, gv); c != "" {
		out = append(out, c)
	}
	return append(out, plantBodyLines(p, gv)...)
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
		return orderIs(command.OrderHarvest)
	}
	return orderIs(command.OrderCut)
}

// plantMarkCmd marks or (with undo) clears the harvest or cut mark on the
// plant's tile only; it never cancels zones.
func plantMarkCmd(p game.PlantView, harvest, undo bool) command.Command {
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

// plantIconKey is the sprite key of the plant's icon.
func (a *App) plantIconKey(p game.PlantView) string {
	key, _ := plantSprite(p)
	if a.renderer.art != nil {
		key = a.renderer.seasonKey(key, a.shell.World())
	}
	return key
}

// drawPlantIcon draws the plant's icon, size px square at (x, y). Nothing
// without art.
func (a *App) drawPlantIcon(dst *ebiten.Image, p game.PlantView, x, y, size float64) {
	if a.renderer == nil || a.renderer.art == nil {
		return
	}
	if img := a.renderer.art.Image(a.plantIconKey(p), color.NRGBA{}); img != nil {
		drawIcon(dst, img, x, y, size)
	}
}

// drawPlantMark draws the selection mark on the plant's tile; the
// panel itself is the inspect pane (plantPane).
func (a *App) drawPlantMark(screen *ebiten.Image) {
	p, ok := a.selectedPlant()
	if !ok || a.win.Open || a.arch.Open {
		return
	}
	if a.renderer.art != nil {
		a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, float64(p.X), float64(p.Y), a.shell.TilePx(), a.pw, a.ph) // physical, like the map
	}
}

// applyPlantClick handles a "plant.*" button.
func (a *App) applyPlantClick(id string) {
	p, ok := a.selectedPlant()
	if !ok {
		return
	}
	switch {
	case id == "plant.close":
		a.clearPlant()
	case id == "plant.harvest" && !isTree(p):
		if p.Designated || p.Harvestable {
			a.shell.Enqueue(plantMarkCmd(p, true, p.Designated))
		}
	case id == "plant.cut":
		a.shell.Enqueue(plantMarkCmd(p, false, p.Cut))
	}
}
