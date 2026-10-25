package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
			x: r.x + 12 + float32(i*88), y: r.y + 104, w: 82, h: 24, selected: b.TroughFilter == f.key})
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
// (x, y): the trough's herd. The trough itself blocks movement, so it is
// not a pen cell: the pen is the one holding a cell next to it.
func penAnimals(w game.WorldView, x, y int) []game.AnimalView {
	var bits []uint64
	for _, p := range w.Pens {
		if !p.Enclosed {
			continue
		}
		for dy := -1; dy <= 1 && bits == nil; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if bitAt(p.Bits, w.TerrainW, x+dx, y+dy) {
					bits = p.Bits
					break
				}
			}
		}
		if bits != nil {
			break
		}
	}
	if bits == nil {
		return nil
	}
	var out []game.AnimalView
	for _, an := range w.Animals {
		if an.Tame && bitAt(bits, w.TerrainW, an.X, an.Y) {
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
	fx, fy := float64(x), float64(y)
	drawText(screen, fmt.Sprintf("Feed  %d / %d", b.TroughFill, b.TroughCap), fx, fy, tsBody)
	bx := float32(x + 130)
	by := float32(y) + (lineStep(tsBody)-14)/2
	uiFillRect(screen, bx, by, 200, 14, uiBtnDisabled, false)
	uiFillRect(screen, bx, by, float32(200*pct/100), 14, uiAccent, false)
	dim := withColor(tsSmall, uiTextDim)
	drawText(screen, "Haulers fill it with feed matching the filter.", fx, fy+float64(lineStep(tsBody)), dim)
	drawText(screen, "Accepts:", fx, fy+float64(lineStep(tsBody)+lineStep(tsSmall)), dim)
	list := penAnimals(a.renderer.world(), b.X, b.Y)
	drawText(screen, fmt.Sprintf("Pen animals (%d)", len(list)), fx, float64(r.y)+138, withColor(tsLabel, uiHeading))
	ly, ss := float64(r.y)+162, float64(lineStep(tsSmall))
	for i, an := range list {
		if i >= maxTroughAnimals {
			drawText(screen, fmt.Sprintf("... and %d more", len(list)-i), fx, ly, dim)
			break
		}
		line := fmt.Sprintf("%s (%s)", an.Name, animalLabel(an))
		if animalHasNeeds(an) {
			line += fmt.Sprintf("  food %d%%", an.Food)
		}
		drawText(screen, ellipsize(line, tsSmall, float64(r.w)-32), fx+8, ly, tsSmall)
		ly += ss
	}
}
