package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Vector stand-ins for the ENG-003 buildings whose art is not generated
// yet (archToolsPendingArt; ART_DIRECTION "Social & host systems asset
// list"), so nothing placed is invisible. Each kind's colour hints at it:
// a brewing bench's barrel brown, a drug table's ash green, the two floor
// spots a ribbon pink and a lantern gold.
var pendingArtKinds = map[string]color.RGBA{
	"brewing bench": {120, 82, 46, 255},
	"drug table":    {96, 112, 84, 255},
	"marriage spot": {196, 120, 150, 255},
	"party spot":    {214, 172, 72, 255},
}

// pendingArtFallback draws a pending-art building (or its blueprint, at a
// lower alpha) over its footprint when key has no generated sprite; false
// leaves every other kind, and any kind once its art exists, to the
// sprite path.
func (r *Renderer) pendingArtFallback(dst *ebiten.Image, kind, key string, x, y, w, h int, tile float64, alpha uint8) bool {
	c, ok := pendingArtKinds[kind]
	if !ok || (key != "" && r.art.has(key)) {
		return false
	}
	sx, sy := r.worldToScreenF(float64(x), float64(y))
	fw, fh := float32(max(w, 1))*float32(tile), float32(max(h, 1))*float32(tile)
	pad := float32(tile) * 0.12
	c.A = alpha
	edge := color.RGBA{c.R / 2, c.G / 2, c.B / 2, alpha}
	if kind == "marriage spot" || kind == "party spot" { // floor markings: a ring, no body
		vector.StrokeRect(dst, sx+pad, sy+pad, fw-2*pad, fh-2*pad, float32(tile)*0.08, c, true)
		return true
	}
	vector.FillRect(dst, sx+pad, sy+pad, fw-2*pad, fh-2*pad, c, true)
	vector.StrokeRect(dst, sx+pad, sy+pad, fw-2*pad, fh-2*pad, 1, edge, true)
	return true
}
