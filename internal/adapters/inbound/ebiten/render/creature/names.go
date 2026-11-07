package creature

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
)

// Map name plates (UIS-088, UIS-089): a colonist's name under its cell at
// every zoom, and a tame animal's (traces.go), in the Small style (cap 8
// at 1x). Text, plate and gap are logical pixels times the UI scale,
// never the zoom, so a name reads the same at 9 and 49 px cells.
const (
	// NamePlatePad is the plate's width past the text, NamePlateH its
	// height: the colonists' and the tame animals' plates alike (UIS-089).
	NamePlatePad = 3
	NamePlateH   = 12
	// NamePlateGap is how far below a colonist's cell the plate's top
	// sits.
	NamePlateGap = 3
)

// DrawPawnName puts colonist name on a dark plate centred under the tile
// point (wx, wy) in tiles, the bottom centre of the cell it is drawn on:
// the plate's top NamePlateGap logical px below it. Every colonist gets
// one, always (no declutter); it is the map's own HUD representation of
// who stands where.
func DrawPawnName(c *Set, v *render.View, cull *Cull, screen *ebiten.Image, name string, wx, wy float64) {
	sx, sy := v.WorldToScreenF(wx, wy)
	k := float32(gfx.UIScale())
	drawNamePlate(c, v, cull, screen, name, sx, sy+NamePlateGap*k, "pawn-name-plate", "pawn-name-text")
}

// drawNamePlate draws name on its plate, centred on physical x sx with the
// plate's top at physical y top. The anchor is a world position (physical
// pixels): the text goes through WorldText, which converts it to logical
// once, and the plate is sized in physical pixels from the same logical
// text size, so both scale with the UI together. The glyph box (the line
// less its 2 px of leading) is centred on the plate, a pixel up as on the
// count tags (labels.go).
func drawNamePlate(c *Set, v *render.View, cull *Cull, screen *ebiten.Image, name string, sx, top float32, plateKind, textKind string) {
	k := float32(gfx.UIScale())
	pw, ph := (gfx.TextW(name, gfx.Small)+NamePlatePad)*k, float32(NamePlateH)*k
	ty := top + (ph-(gfx.LineStep(gfx.Small)-2)*k)/2 - k
	if v.Tracing() {
		v.MarkDrawn(plateKind, sx-pw/2, top, pw, ph)
		v.MarkDrawn(textKind, sx, ty, 0, 0)
	}
	c.solidRect(cull, screen, sx-pw/2, top, pw, ph, color.RGBA{16, 12, 8, 150}) // a literal: no per-call boxing
	st := gfx.Small
	st.Align = gfx.AlignCenter
	gfx.WorldText(screen, name, float64(sx), float64(ty), st)
}
