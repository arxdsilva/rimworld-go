package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Feeding trough inspector section (plan 6.2): a fill bar, the Plants /
// Meat / Any filter (SetTroughFilter) and the pen's tame animals.

const troughKind = "feeding trough"

// troughFilters are the filter buttons; Mode is the SetTroughFilter value.
var troughFilters = []struct {
	key, label string
	mode       uint8
}{
	{"plants", "Plants", 0},
	{"meat", "Meat", 1},
	{"any", "Any", 2},
}

const maxTroughAnimals = 6

func isTrough(b game.BuildingView) bool { return b.Kind == troughKind }

func (a *App) troughButtons(b game.BuildingView, r button) []button {
	if !isTrough(b) {
		return nil
	}
	out := make([]button, 0, len(troughFilters))
	for i, f := range troughFilters {
		out = append(out, button{id: "bldg.trough." + f.key, label: f.label,
			x: r.x + 12 + float32(i*88), y: r.y + 96, w: 82, h: 24, selected: b.TroughFilter == f.key})
	}
	return out
}

// applyTroughClick enqueues SetTroughFilter for a filter button.
func (a *App) applyTroughClick(id string, b game.BuildingView) bool {
	key, ok := strings.CutPrefix(id, "bldg.trough.")
	if !ok || !isTrough(b) {
		return false
	}
	for _, f := range troughFilters {
		if f.key == key {
			a.renderer.enqueue(command.Command{Kind: command.SetTroughFilter, Target: uint64(b.ID), Mode: f.mode})
		}
	}
	return true
}

// penAnimals are the tame animals standing in the pen that holds tile
// (x, y): the trough's herd.
func penAnimals(w game.WorldView, x, y int) []game.AnimalView {
	cells := map[[2]int]bool{}
	for _, z := range w.Zones {
		if z.Kind != game.ZonePen {
			continue
		}
		// the trough's own tile may not be a pen cell; its bounding box is
		if x >= z.MinX && x <= z.MaxX && y >= z.MinY && y <= z.MaxY {
			for _, c := range z.Cells {
				cells[c] = true
			}
		}
	}
	var out []game.AnimalView
	for _, an := range w.Animals {
		if an.Tame && cells[[2]int{an.X, an.Y}] {
			out = append(out, an)
		}
	}
	return out
}

func (a *App) drawTroughSection(screen *ebiten.Image, b game.BuildingView, r button) {
	if !isTrough(b) {
		return
	}
	x, y := int(r.x)+12, int(r.y)+46
	pct := 0
	if b.TroughCap > 0 {
		pct = b.TroughFill * 100 / b.TroughCap
	}
	drawLabel(screen, x, y, fmt.Sprintf("Feed  %d / %d", b.TroughFill, b.TroughCap))
	bx := float32(x + 130)
	vector.FillRect(screen, bx, float32(y), 200, 14, uiBtnDisabled, false)
	vector.FillRect(screen, bx, float32(y), float32(200*pct/100), 14, uiAccent, false)
	drawLabel(screen, x, y+18, "Haulers fill it with feed matching the filter.")
	drawLabel(screen, x, y+36, "Accepts:")
	list := penAnimals(a.renderer.world(), b.X, b.Y)
	drawLabel(screen, x, int(r.y)+134, fmt.Sprintf("Pen animals (%d)", len(list)))
	for i, an := range list {
		if i >= maxTroughAnimals {
			drawLabel(screen, x, int(r.y)+152+i*16, fmt.Sprintf("... and %d more", len(list)-i))
			break
		}
		line := fmt.Sprintf("%s (%s)", an.Name, animalLabel(an))
		if animalHasNeeds(an) {
			line += fmt.Sprintf("  food %d%%", an.Food)
		}
		drawLabel(screen, x+8, int(r.y)+152+i*16, line)
	}
}
