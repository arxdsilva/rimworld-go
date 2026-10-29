package world

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game"
)

// Pawn figures and their animation live in render/creature (ARCH-004);
// the object pass's own extras over a pawn stay here.

// DrawPawnArt draws a pawn in the object pass: its figure, then the
// extras that are not part of its shape (hostile marker, draft icon).
// It also does the pass's bookkeeping, which the figure draw never does.
func (o *Objects) DrawPawnArt(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, sw, sh int) {
	if p.CarriedBy != 0 {
		return // drawn over its rescuer
	}
	v := o.v
	o.Creatures.DropCarry(p.ID)
	fp := o.Creatures.FigurePoseOf(v, p)
	o.Creatures.DrawPawnFigure(v, o.Pass(), screen, p, wx, wy, tile, fp)
	if p.Lying {
		return
	}
	ps := fp.Ps
	if p.Hostile {
		o.Lb.DrawHostileMarker(screen, p, wx+ps.Dx, wy+ps.Dy, PawnMarkerLift(o.art()), tile)
	}
	if p.Drafted {
		if img := o.art().Image("ui_icon/draft", color.NRGBA{}); img != nil {
			sx, sy := o.worldToScreenF(wx+0.62+ps.Dx, wy-PawnMarkerLift(o.art())+ps.Dy)
			gfx.WorldIcon(screen, img, float64(sx), float64(sy), tile*0.3/gfx.UIScale()) // tile is physical
		}
	}
}
