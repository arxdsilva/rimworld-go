package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Pawn figures and their animation live in render/creature (ARCH-004);
// the object pass's own extras over a pawn stay here.

// drawPawnArt draws a pawn in the object pass: its figure, then the
// extras that are not part of its shape (hostile marker, draft icon).
// It also does the pass's bookkeeping, which the figure draw never does.
func (r *Renderer) drawPawnArt(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, sw, sh int) {
	if p.CarriedBy != 0 {
		return // drawn over its rescuer
	}
	r.animState().DropCarry(p.ID)
	fp := r.pawnFigurePoseOf(p)
	r.drawPawnFigure(screen, p, wx, wy, tile, fp)
	if p.Lying {
		return
	}
	ps := fp.Ps
	if p.Hostile {
		r.drawHostileMarker(screen, p, wx+ps.Dx, wy+ps.Dy, pawnMarkerLift(r.art), tile)
	}
	if p.Drafted {
		if img := r.art.Image("ui_icon/draft", color.NRGBA{}); img != nil {
			sx, sy := r.worldToScreenF(wx+0.62+ps.Dx, wy-pawnMarkerLift(r.art)+ps.Dy)
			worldIcon(screen, img, float64(sx), float64(sy), tile*0.3/uiScale()) // tile is physical
		}
	}
}
