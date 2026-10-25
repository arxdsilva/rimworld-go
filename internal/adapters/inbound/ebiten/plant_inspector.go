package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Plant panel (UI-011, MAIN_UI §10): what the plant is, how far it has
// grown, and Harvest / Cut (or Chop) with "now" orders per colonist.

// plantRect is the plant panel's frame (the inspector's top-left slot).
func plantRect() button { return button{x: hudPad, y: hudTopH + hudPad, w: 340, h: 210} }

// maxPlantChips is how many "now" colonist chips fit in the panel's row.
const maxPlantChips = 3

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
	if _, ok := plantKeys.crops[p.Species]; ok || p.Zone != "" {
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

// plantLines are the body lines under the growth bar.
func plantLines(p game.PlantView) []string {
	var out []string
	switch {
	case isTree(p):
	case p.Harvestable:
		out = append(out, "Ready to harvest")
	default:
		out = append(out, fmt.Sprintf("Not ready (%d%%)", p.Growth))
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

func plantNowChips(p game.PlantView, acts []colonistAction) []colonistAction {
	if len(acts) > maxPlantChips {
		acts = acts[:maxPlantChips]
	}
	return acts
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

func chipLabel(c colonistAction) string {
	n := c.Name
	if c.Disabled {
		n += " ×"
	}
	return n
}

// plantPanelButtons lays out the plant panel's buttons for p and its
// "now" colonist chips.
func plantPanelButtons(r button, p game.PlantView, chips []colonistAction) []button {
	out := []button{{id: "plant.close", label: "X", x: r.x + r.w - 28, y: r.y + 6, w: 22, h: 22}}
	by := r.y + r.h - 30
	x := r.x + 12
	if !isTree(p) {
		label, dis, reason := "Harvest", false, ""
		switch {
		case p.Designated:
			label = "Cancel harvest"
		case !p.Harvestable:
			dis, reason = true, "not ready"
		}
		out = append(out, button{id: "plant.harvest", label: label, x: x, y: by, w: 130, h: 24, selected: p.Designated, disabled: dis, reason: reason, quiet: true})
		x += 138
	}
	cut := "Cut"
	if isTree(p) {
		cut = "Chop"
	}
	if p.Cut {
		cut = "Cancel cut"
	}
	out = append(out, button{id: "plant.cut", label: cut, x: x, y: by, w: 130, h: 24, selected: p.Cut})
	cx := r.x + 48
	for _, c := range plantNowChips(p, chips) {
		out = append(out, button{id: "plant.now." + strconv.FormatUint(uint64(c.Pawn), 10), label: chipLabel(c),
			x: cx, y: by - 26, w: 88, h: 20, disabled: c.Disabled, reason: c.Reason, quiet: true})
		cx += 90
	}
	return out
}

func (a *App) plantNow(p game.PlantView) []colonistAction {
	return a.colonistActions(p.X, p.Y, plantNowMatch(p))
}

// plantButtons are the plant panel's buttons (ids "plant.*").
func (a *App) plantButtons() []button {
	p, ok := a.selectedPlant()
	if !ok || a.workOpen || a.archOpen {
		return nil
	}
	return plantPanelButtons(plantRect(), p, a.plantNow(p))
}

// plantIconKey is the sprite key of the plant's icon.
func (a *App) plantIconKey(p game.PlantView) string {
	key, _ := plantSprite(p)
	if a.renderer.art != nil {
		key = a.renderer.seasonKey(key, a.renderer.world())
	}
	return key
}

// drawPlantInspector draws the plant panel when a plant is selected.
func (a *App) drawPlantInspector(screen *ebiten.Image) {
	p, ok := a.selectedPlant()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	r := plantRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	if a.renderer.art != nil {
		if img := a.renderer.art.image(a.plantIconKey(p), color.NRGBA{}); img != nil {
			drawIcon(screen, img, float64(r.x)+12, float64(r.y)+10, 40)
		}
	}
	x, y := int(r.x), int(r.y)
	drawText(screen, ellipsize(plantTitle(p), tsLabel, float64(r.w)-60-40), float64(x+60), float64(y+10), withColor(tsLabel, uiHeading))
	drawText(screen, ellipsize(plantCategory(p), tsSmall, float64(r.w)-60-12), float64(x+60), float64(y+32), withColor(tsSmall, uiTextDim))
	drawNeedBar(screen, x+12, y+62, int(r.w)-24, 12, p.Growth, plantStageWord(p))
	ly := float64(y + 84)
	for _, l := range plantLines(p) {
		for _, wl := range wrapPx(l, tsSmall, float64(r.w)-24) {
			drawText(screen, wl, float64(x+12), ly, tsSmall)
			ly += float64(lineStep(tsSmall))
		}
	}
	drawText(screen, "now:", float64(x+12), float64(y)+float64(r.h)-56+(20-lineH(tsSmall))/2, withColor(tsSmall, uiTextDim))
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.plantButtons(), mx, my)
	if a.renderer.art != nil {
		a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, float64(p.X), float64(p.Y), a.renderer.tilePx(), a.pw, a.ph) // physical, like the map
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
			a.renderer.enqueue(plantMarkCmd(p, true, p.Designated))
		}
	case id == "plant.cut":
		a.renderer.enqueue(plantMarkCmd(p, false, p.Cut))
	case strings.HasPrefix(id, "plant.now."):
		n, err := strconv.ParseUint(strings.TrimPrefix(id, "plant.now."), 10, 64)
		if err != nil {
			return
		}
		for _, c := range a.plantNow(p) {
			if c.Pawn == entity.EntityID(n) && !c.Disabled {
				a.renderer.enqueue(c.Cmd)
				return
			}
		}
	}
}
