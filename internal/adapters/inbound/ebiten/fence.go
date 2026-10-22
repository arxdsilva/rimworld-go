package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Fences and the forbidden marker (UI-017, ART fences). Fences autotile
// among themselves, their gates and walls (one way, linkgrid.go); a gate is drawn open
// while a creature stands in it. Every key may be missing from the art
// catalog: the vector fallbacks below keep nothing invisible.

const (
	fenceKind = "fence"
	gateKind  = "fence gate"
)

func isFenceKind(kind string) bool { return kind == fenceKind || kind == gateKind }

// fenceSprite is structure/fence_<wood|stone>_<mask>.
func fenceSprite(stuff string, mask int) string {
	m := "wood"
	if isStoneStuff(stuff) {
		m = "stone"
	}
	return "structure/fence_" + m + "_" + twoDigits(mask)
}

// gateSprite is structure/fence_gate[_open]_<facing>.
func gateSprite(facing string, open bool) string {
	if open {
		return "structure/fence_gate_open_" + facingOr(facing)
	}
	return "structure/fence_gate_" + facingOr(facing)
}

// creatureCells marks tiles holding a colonist or an animal (a gate there
// is drawn open).
func creatureCells(w game.WorldView, pawns []game.PawnView) map[[2]int]bool {
	out := map[[2]int]bool{}
	for _, p := range pawns {
		out[[2]int{p.X, p.Y}] = true
	}
	for _, an := range w.Animals {
		out[[2]int{an.X, an.Y}] = true
	}
	return out
}

// fenceLabelAt is the hover/inspector text for a fence or gate tile.
func fenceLabelAt(w game.WorldView, x, y int) string {
	for _, b := range w.Buildings {
		if b.X != x || b.Y != y {
			continue
		}
		switch b.Kind {
		case fenceKind:
			return "Fence: keeps animals out (wild) / in (tame)"
		case gateKind:
			return "Fence gate: tame animals and colonists pass"
		}
	}
	return ""
}

// drawFenceVector is the plain-shape fence used when the art has no
// sprite for it: a post with rails toward each joined neighbour; a gate
// is two posts with a bar across (swung aside when open).
func drawFenceVector(screen *ebiten.Image, sx, sy, tile float32, kind string, mask int, open, stone bool, alpha uint8) {
	wood := color.RGBA{R: 140, G: 100, B: 60, A: alpha}
	if stone {
		wood = color.RGBA{R: 140, G: 140, B: 145, A: alpha}
	}
	cx, cy := sx+tile/2, sy+tile/2
	th := max(tile/10, 2)
	if kind == gateKind {
		vector.FillRect(screen, sx+tile*0.08, sy+tile*0.2, th*1.5, tile*0.6, wood, false)
		vector.FillRect(screen, sx+tile*0.92-th*1.5, sy+tile*0.2, th*1.5, tile*0.6, wood, false)
		if open {
			vector.FillRect(screen, sx+tile*0.08, sy+tile*0.2, th, tile*0.2, wood, false)
		} else {
			vector.FillRect(screen, sx+tile*0.08, cy-th/2, tile*0.84, th, wood, false)
		}
		return
	}
	vector.FillRect(screen, cx-th, cy-th, th*2, th*2, wood, false)
	if mask&wallN != 0 {
		vector.FillRect(screen, cx-th/2, sy, th, tile/2, wood, false)
	}
	if mask&wallS != 0 {
		vector.FillRect(screen, cx-th/2, cy, th, tile/2, wood, false)
	}
	if mask&wallW != 0 {
		vector.FillRect(screen, sx, cy-th/2, tile/2, th, wood, false)
	}
	if mask&wallE != 0 {
		vector.FillRect(screen, cx, cy-th/2, tile/2, th, wood, false)
	}
}

// forbiddenKey is the red no-entry chip drawn over a forbidden stack.
const forbiddenKey = "overlay/forbidden"

// drawForbiddenMark draws the forbidden marker on tile (wx, wy): the
// sprite when present, else a red ring with a slash.
func (r *Renderer) drawForbiddenMark(screen *ebiten.Image, wx, wy int, tile float64, sw, sh int) {
	if r.art.has(forbiddenKey) {
		r.drawSprite(screen, forbiddenKey, color.NRGBA{}, float64(wx), float64(wy), tile, sw, sh)
		return
	}
	sx, sy := r.worldToScreen(wx, wy)
	t := float32(tile)
	cx, cy, rad := sx+t*0.75, sy+t*0.25, t*0.2
	vector.StrokeCircle(screen, cx, cy, rad, max(t/16, 2), alertClr, true)
	vector.StrokeLine(screen, cx-rad*0.7, cy+rad*0.7, cx+rad*0.7, cy-rad*0.7, max(t/16, 2), alertClr, true)
}
