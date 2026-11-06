package creature

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// Far-zoom pawn highlight (OPT-016). At the map tier a pawn is a few
// pixels, so the player can ask for a marker: a dot, a solid silhouette or
// both, for hostile creatures only or for every pawn. The values are
// config.FarPawnModes and config.FarStyles, spelt out here because this
// layer does not import platform/config.
const (
	farHostile    = "hostile"
	farAll        = "all"
	farSilhouette = "silhouettes"
	farBoth       = "both"
)

// Marker sizes in logical pixels (scaled by the UI scale like the plates).
const (
	farDot    = 6 // a dot's side
	farBodyW  = 7 // the silhouette's body
	farBodyH  = 8
	farHeadW  = 5 // and head, sitting on the body
	farHeadH  = 5
	farBorder = 1
)

var (
	farHostileClr = color.RGBA{232, 56, 44, 255}
	farFriendClr  = color.RGBA{236, 226, 150, 255}
	farEdgeClr    = color.RGBA{16, 12, 8, 220}
)

// DrawFarPawns marks the pawns (and, for any mode but none, the hostile
// animals) at the map tier per v.Prefs.FarPawns / FarStyle. It does
// nothing below the map tier, with the mode "none" or an unknown one. It
// is a plain function over the layer rather than a method (the layer is at
// its method budget); it loops the frame's creature lists once with the
// view culled, so no per-tile work and no allocation.
func DrawFarPawns(c *Set, v *render.View, cull *Cull, screen *ebiten.Image, w game.WorldView) {
	mode, style := v.Prefs.FarPawns, v.Prefs.FarStyle
	if !cull.MapTier || (mode != farHostile && mode != farAll) {
		return
	}
	for i := range v.Pawns {
		p := &v.Pawns[i]
		if p.Embodied || (mode == farHostile && !p.Hostile) || !cull.InView(p.X, p.Y, 1, 1) {
			continue
		}
		wx, wy := c.pawnAt(v, *p)
		farMark(c, v, cull, screen, style, wx, wy, p.Hostile)
	}
	for i := range w.Animals {
		an := &w.Animals[i]
		if !(an.Hostile || an.Manhunter) || !cull.InView(an.X, an.Y, 1, 1) {
			continue
		}
		wx, wy := c.AnimalAt(v, *an)
		farMark(c, v, cull, screen, style, wx, wy, true)
	}
}

// farMark draws one marker centred on the tile point (wx, wy).
func farMark(c *Set, v *render.View, cull *Cull, screen *ebiten.Image, style string, wx, wy float64, hostile bool) {
	sx, sy := v.WorldToScreenF(wx+0.5, wy+0.5)
	k := float32(gfx.UIScale())
	clr := farFriendClr
	if hostile {
		clr = farHostileClr
	}
	if style == farSilhouette || style == farBoth {
		if v.Tracing() {
			v.MarkDrawn("far-silhouette", sx, sy, b2f(hostile), 0)
		}
		bw, bh, hw, hh, e := farBodyW*k, farBodyH*k, farHeadW*k, farHeadH*k, farBorder*k
		top := sy - (bh+hh)/2
		c.solidRect(cull, screen, sx-hw/2-e, top-e, hw+2*e, hh+2*e, farEdgeClr)
		c.solidRect(cull, screen, sx-bw/2-e, top+hh-e, bw+2*e, bh+2*e, farEdgeClr)
		c.solidRect(cull, screen, sx-hw/2, top, hw, hh, clr)
		c.solidRect(cull, screen, sx-bw/2, top+hh, bw, bh, clr)
		sy = top - (farDot*k)/2 - 2*k // a dot in "both" rides above the head
	}
	if style != farSilhouette { // dots, both, and the default
		if v.Tracing() {
			v.MarkDrawn("far-dot", sx, sy, b2f(hostile), 0)
		}
		d, e := farDot*k, farBorder*k
		c.solidRect(cull, screen, sx-d/2-e, sy-d/2-e, d+2*e, d+2*e, farEdgeClr)
		c.solidRect(cull, screen, sx-d/2, sy-d/2, d, d, clr)
	}
}

// b2f is a mark argument for a flag.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
