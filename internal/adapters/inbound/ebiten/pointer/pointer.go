// Package pointer draws the game's own mouse pointer (OPT-028): with the
// Custom cursor option on, the OS cursor is hidden and the cursor/pointer
// sprite is drawn last in App.Draw, over everything.
package pointer

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

const (
	key     = "cursor/pointer"
	busyKey = "cursor/pointer_busy"
	// Logical is the pointer's size in logical px at UI scale 100 %.
	Logical = 32.0
)

// Placement is the sprite's top-left corner and uniform scale for a pointer
// at (mx, my) in logical px: canvas px is the sprite's width and hot its
// hotspot in canvas px. The sprite is Logical logical px wide (so Logical x
// UI scale on screen) and its hotspot lands on the pointer.
func Placement(mx, my float64, hot [2]float64, canvas float64) (x, y, k float64) {
	k = Logical / canvas
	return mx - hot[0]*k, my - hot[1]*k, k
}

// Draw draws the pointer at (mx, my) in logical px (the busy one while the
// game loads) and reports whether it did. False means the art is missing:
// the caller keeps the OS cursor. No allocation per call.
func Draw(dst *ebiten.Image, c *art.Catalog, mx, my float64, busy bool) bool {
	k := key
	if busy {
		k = busyKey
	}
	m, img := c.MetaOf(k), c.Image(k, color.NRGBA{})
	if m == nil || img == nil {
		return false
	}
	x, y, s := Placement(mx, my, m.Attach["hotspot"], float64(img.Bounds().Dx()))
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	gfx.DrawImage(dst, img, &op)
	return true
}
