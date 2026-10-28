package world

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// Fences and the forbidden marker (UI-017, ART fences). Fences autotile
// among themselves, their gates and walls (one way, sprite.LinkGrid); a
// gate is drawn open while a creature stands in it. Every key may be
// missing from the art catalog: the vector fallbacks keep nothing
// invisible.

// GateSprite is structure/fence_gate[_open]_<facing>.
func GateSprite(facing string, open bool) string {
	if open {
		return "structure/fence_gate_open_" + FacingOr(facing)
	}
	return "structure/fence_gate_" + FacingOr(facing)
}

// FenceLabelAt is the hover/inspector text for a fence or gate tile.
func FenceLabelAt(w game.WorldView, x, y int) string {
	for _, b := range w.Buildings {
		if b.X != x || b.Y != y {
			continue
		}
		switch b.Kind {
		case sprite.FenceKind:
			return "Fence: keeps animals out (wild) / in (tame)"
		case sprite.GateKind:
			return "Fence gate: tame animals and colonists pass"
		}
	}
	return ""
}

// ForbiddenKey is the red no-entry chip drawn over a forbidden stack.
const ForbiddenKey = "overlay/forbidden"

// DrawForbiddenMark draws the forbidden marker on tile (wx, wy): the
// sprite when present, else a red ring with a slash.
func (o *Objects) DrawForbiddenMark(screen *ebiten.Image, wx, wy int, tile float64, sw, sh int) {
	if o.art().Has(ForbiddenKey) {
		o.drawSprite(screen, ForbiddenKey, color.NRGBA{}, float64(wx), float64(wy), tile, sw, sh)
		return
	}
	sx, sy := o.worldToScreen(wx, wy)
	t := float32(tile)
	cx, cy, rad := sx+t*0.75, sy+t*0.25, t*0.2
	vector.StrokeCircle(screen, cx, cy, rad, max(t/16, 2), uikit.Danger, true)
	vector.StrokeLine(screen, cx-rad*0.7, cy+rad*0.7, cx+rad*0.7, cy-rad*0.7, max(t/16, 2), uikit.Danger, true)
}
