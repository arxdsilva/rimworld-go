// Package sprite draws one world sprite — by key, tint, colour scale and
// scale — lit through render/light, and owns the keys around it: the
// linked-building grid and its keys (walls, rock, fences, conduits), the
// stone and stuff tints, the fence sprites and their vector fallback, and
// the small sprite-key helpers (tile hash, grass tints, two-digit
// suffixes). A Painter reads the frame's *render.View, its catalog, its
// light layer and its own caches, never the renderer (SOLID_GO.md §7).
package sprite

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/application/game"
)

// Painter draws world sprites from Art, lit by Lit; Stone memoises the
// stone tints and resolved linked keys for Art.
type Painter struct {
	Art   *art.Catalog
	Lit   *light.Lit
	Stone *StoneCache
}

// NewPainter is a painter of catalog c lit by lit, with its own caches.
func NewPainter(c *art.Catalog, lit *light.Lit) *Painter {
	return &Painter{Art: c, Lit: lit, Stone: &StoneCache{}}
}

// Pass is the pass a sprite is drawn in, the renderer's to say: Cast,
// while set, turns draws into shadow silhouettes; Baking draws the plain
// albedo into a bake (lit when the bake is drawn); Blood stains the
// figure layer being drawn (light.Lit.Draw).
type Pass struct {
	Cast   *light.Cast
	Baking bool
	Blood  float32
}

// Draw draws sprite key at world (wx, wy), tinted (zero: none), in pass ps.
func (p *Painter) Draw(v *render.View, ps Pass, dst *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int) {
	p.DrawScaled(v, ps, dst, key, tint, wx, wy, tile, sw, sh, NoCS, 1)
}

// DrawCS is Draw with colour scale cs.
func (p *Painter) DrawCS(v *render.View, ps Pass, dst *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs ColorScale) {
	p.DrawScaled(v, ps, dst, key, tint, wx, wy, tile, sw, sh, cs, 1)
}

// DrawScaled draws a sprite scaled by k about its footprint's
// bottom-centre (ground origin), e.g. young trees. Positions come from
// v's camera; dst is sw x sh map pixels with tile pixels per tile.
func (p *Painter) DrawScaled(v *render.View, ps Pass, dst *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs ColorScale, k float64) {
	sp := p.Art.Sprite(key)
	if sp == nil {
		return
	}
	sx, sy := v.WorldToScreenF(wx, wy)
	x, y, s := art.Origin(sp.Meta, float64(sx), float64(sy), tile)
	if k != 1 {
		px := float64(sx) + float64(sp.Meta.Footprint[0])*tile/2
		py := float64(sy) + float64(sp.Meta.Footprint[1])*tile
		x, y, s = px+(x-px)*k, py+(y-py)*k, s*k
	}
	w, h := float64(sp.Meta.Canvas[0])*s, float64(sp.Meta.Canvas[1])*s
	if !Visible(float32(x), float32(y), float32(w), float32(h), sw, sh) {
		return
	}
	img := p.Art.Image(key, tint)
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	if !strings.HasPrefix(key, "overlay/") { // zones and markers stay unlit
		if light.IsGroundKey(key) { // terrain edges: the lamp light ramps like the tile under them
			p.Lit.DrawGround(v, dst, img, op.GeoM, cs, wx, wy, ps.Cast, ps.Baking, ps.Blood)
			return
		}
		p.Lit.Draw(v, dst, img, op.GeoM, cs, wx, wy, ps.Cast, ps.Baking, ps.Blood)
		return
	}
	if cs != NoCS {
		op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
	}
	dst.DrawImage(img, op)
}

// Visible reports whether the rectangle (x, y, w, h) meets a sw x sh
// destination.
func Visible(x, y, w, h float32, sw, sh int) bool {
	return x+w >= 0 && y+h >= 0 && x < float32(sw) && y < float32(sh)
}

// ColorScale is a per-draw RGBA multiplier (ghosts, blueprints).
type ColorScale [4]float32

var (
	NoCS        = ColorScale{1, 1, 1, 1}
	BlueprintCS = ColorScale{0.55, 0.85, 1.25, 0.5}
	GhostOKCS   = ColorScale{0.7, 1.25, 0.7, 0.65}
	GhostBadCS  = ColorScale{1.3, 0.45, 0.4, 0.6}
	GhostSameCS = ColorScale{0.9, 0.9, 0.9, 0.5}
	// DriedCS browns and darkens food that came off a drying rack;
	// RottenCS sickens a rotten corpse.
	DriedCS  = ColorScale{0.86, 0.68, 0.48, 1}
	RottenCS = ColorScale{0.62, 0.74, 0.5, 1}
)

// ItemCS is the colour treatment for a ground stack.
func ItemCS(it game.ItemView) ColorScale {
	switch {
	case it.Dried:
		return DriedCS
	case it.Rotten:
		return RottenCS
	}
	return NoCS
}

// LerpCS blends from a to b by t in [0, 1].
func LerpCS(a, b ColorScale, t float64) ColorScale {
	var out ColorScale
	for i := range out {
		out[i] = a[i] + (b[i]-a[i])*float32(t)
	}
	return out
}

// DoorAxisEW is the door sprite axis for a stored facing: South/North
// doors span east-west ("ew"), East/West doors north-south ("ns").
func DoorAxisEW(facing string) bool { return facing != "east" && facing != "west" }

// TintCS converts a tint to a multiplying colour scale (identity for none).
func TintCS(t color.NRGBA) ColorScale {
	if t.A == 0 {
		return NoCS
	}
	return ColorScale{float32(t.R) / 255, float32(t.G) / 255, float32(t.B) / 255, 1}
}

// MulCS multiplies two colour scales channel by channel.
func MulCS(a, b ColorScale) ColorScale {
	return ColorScale{a[0] * b[0], a[1] * b[1], a[2] * b[2], a[3] * b[3]}
}
