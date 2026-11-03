package overlay

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The wires on the map, overlay or not (PWR-1, PWR-5). Presentation only:
// the read model's link for what is built, GhostWire for what is planned.
// Graphical only, like the link they show: nothing here is a building.

const (
	cordWidth = 0.04 // a tile's share, at least 1 px
	cordPlug  = 0.1  // the plug's side, a tile's share, at least 2.5 widths
	// blueprintCordA is a planned wire's alpha: dimmer than a built one's.
	blueprintCordA = 0.6
	// cordRimA is the pale rim's share of a wire's alpha: enough to find a dark
	// wire on a dark floor, too little to read as a second line on a light one.
	cordRimA = 0.4
	cordRimW = 2 // the rim's width over the wire's, px
)

// Cords is the wire pass's state (Wiring.Cords).
type Cords struct {
	batch gfx.TriBatch             // every wire of a frame, one draw
	stubs gfx.TriBatch             // their starts under the objects, one draw
	foot  [][2]int                 // a blueprint's footprint, reused
	plans map[entity.EntityID]plan // blueprint links, until the transmitters change
	gen   uint64                   // the transmitter build plans was made at
}

// plan is a blueprint's wire: from and to its target's net (wired false:
// it would make none).
type plan struct {
	from, to [2]int
	net      int
	wired    bool
}

// DrawCords draws the wire of every linked device in ch (from the edge of
// the from cell to the centre of the to cell, a plug at the wiring
// end, a faint pale rim so it shows on a dark floor) and, dimmer in the target grid's colour, of every planned device:
// over the objects, so a wall hides no wire to a conduit under it or a
// cooler set in it, and under the weather and the fog. One batched draw
// and none with no wire. Not on the whole-map tier: buildings are baked.
func (o *Wiring) DrawCords(v *render.View, ch *Chunks, screen *ebiten.Image, w *game.WorldView, sw, sh int, tile float64) {
	if w == nil || ch.WholeMap || ch.Baking {
		return
	}
	v.LayerDrawn("power cords")
	o.fillCords(v, ch, w, sw, sh, tile)
	o.Cords.batch.Flush(screen, o.whiteImg(), ebiten.BlendSourceOver)
}

// DrawCordStubs draws each built wire's start, from the centre of the from
// cell to where DrawCords takes over: under the objects, so a device's sprite
// covers as much of it as it needs and the wire reaches the device whatever
// the sprite's size. One batched draw and none with no wire.
func (o *Wiring) DrawCordStubs(v *render.View, ch *Chunks, screen *ebiten.Image, w *game.WorldView, sw, sh int, tile float64) {
	if w == nil || ch.WholeMap || ch.Baking {
		return
	}
	v.LayerDrawn("power cord stubs")
	o.fillCordStubs(v, ch, w, sw, sh, tile)
	o.Cords.stubs.Flush(screen, o.whiteImg(), ebiten.BlendSourceOver)
}

// fillCordStubs batches the wire starts of ch's chunks, cut to the view.
func (o *Wiring) fillCordStubs(v *render.View, ch *Chunks, w *game.WorldView, sw, sh int, tile float64) {
	wd := max(1, tile*cordWidth)
	b := &o.Cords.stubs
	for _, cv := range ch.Views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			d := &w.Buildings[i]
			if !d.HasLink || (d.LinkFromX == d.LinkX && d.LinkFromY == d.LinkY) {
				continue
			}
			sx, sy := cordStart([2]int{d.LinkFromX, d.LinkFromY}, [2]int{d.LinkX, d.LinkY})
			x0, y0 := v.WorldToScreenF(float64(d.LinkFromX)+0.5, float64(d.LinkFromY)+0.5)
			x1, y1 := v.WorldToScreenF(sx, sy)
			bx, by := min(x0, x1)-cordRimW, min(y0, y1)-cordRimW
			if !sprite.Visible(bx, by, max(x0, x1)+cordRimW-bx, max(y0, y1)+cordRimW-by, sw, sh) {
				continue
			}
			cordLine(b, float64(x0), float64(y0), float64(x1), float64(y1), wd+cordRimW, uikit.PowerCordRim, cordRimA)
			cordLine(b, float64(x0), float64(y0), float64(x1), float64(y1), wd, uikit.PowerCord, 1)
		}
	}
}

// fillCords batches the wires of ch's chunks, cut to the view.
func (o *Wiring) fillCords(v *render.View, ch *Chunks, w *game.WorldView, sw, sh int, tile float64) {
	g := &o.Nets
	synced := false
	for _, cv := range ch.Views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			if !b.HasLink || (b.LinkFromX == b.LinkX && b.LinkFromY == b.LinkY) {
				continue
			}
			from := [2]int{b.LinkFromX, b.LinkFromY}
			if o.addCord(v, from, [2]int{b.LinkX, b.LinkY}, uikit.PowerCord, 1, sw, sh, tile) {
				g.Emit("cord", from[0], from[1], TokenNRGBA(uikit.PowerCord))
			}
		}
		for _, i := range cv.Frames {
			if int(i) >= len(w.Frames) {
				continue
			}
			f := &w.Frames[i]
			if !PowerDeviceKinds[f.Kind] {
				continue
			}
			if !synced {
				g.SyncNets(w.PowerNets)
				synced = true
			}
			p := o.planOf(v, w, f)
			if !p.wired || p.from == p.to {
				continue
			}
			c := g.NetColour(p.net)
			if o.addCord(v, p.from, p.to, color.RGBA(c), blueprintCordA, sw, sh, tile) {
				g.Emit("blueprint cord", p.from[0], p.from[1], c)
			}
		}
	}
}

// planOf is frame f's wire, kept until the transmitters change: GhostWire
// reads a square of cells around the footprint, too many for each planned
// device each frame.
func (o *Wiring) planOf(v *render.View, w *game.WorldView, f *game.FrameView) plan {
	g, c := &o.Nets, &o.Cords
	g.syncTransmitters(w)
	if c.gen != g.tr.gen {
		clear(c.plans)
		c.gen = g.tr.gen
	}
	if p, ok := c.plans[f.ID]; ok {
		return p
	}
	foot := c.foot[:0]
	for y := f.Y; y < f.Y+max(f.H, 1); y++ {
		for x := f.X; x < f.X+max(f.W, 1); x++ {
			foot = append(foot, [2]int{x, y})
		}
	}
	c.foot = foot
	var p plan
	p.from, p.to, p.wired, _ = g.GhostWire(w, f.Kind, foot, v.MapW, v.MapH)
	p.net = g.NetAt(p.to)
	if c.plans == nil {
		c.plans = map[entity.EntityID]plan{}
	}
	c.plans[f.ID] = p
	return p
}

// addCord batches a wire unless all of it is off the view.
func (o *Wiring) addCord(v *render.View, from, to [2]int, c color.RGBA, a float64, sw, sh int, tile float64) bool {
	sx, sy := cordStart(from, to)
	x0, y0 := v.WorldToScreenF(sx, sy)
	x1, y1 := v.WorldToScreenF(float64(to[0])+0.5, float64(to[1])+0.5)
	wd := max(1, tile*cordWidth)
	plug := max(wd*2.5, tile*cordPlug)
	m := float32(plug)
	bx, by := min(x0, x1)-m, min(y0, y1)-m
	if !sprite.Visible(bx, by, max(x0, x1)+m-bx, max(y0, y1)+m-by, sw, sh) {
		return false
	}
	b := &o.Cords.batch
	ra, rp := a*cordRimA, plug+cordRimW
	cordLine(b, float64(x0), float64(y0), float64(x1), float64(y1), wd+cordRimW, uikit.PowerCordRim, ra)
	b.Rect(float64(x1)-rp/2, float64(y1)-rp/2, rp, rp, uikit.PowerCordRim, ra, ra, ra, ra)
	cordLine(b, float64(x0), float64(y0), float64(x1), float64(y1), wd, c, a)
	b.Rect(float64(x1)-plug/2, float64(y1)-plug/2, plug, plug, c, a, a, a, a)
	return true
}

// cordStart is where a wire leaves its from cell: the cell's edge on the way
// to the to cell's centre, so it doesn't cross the device it powers (the stub
// under the objects joins it to the sprite). The two cells differ (the callers
// skip the others).
func cordStart(from, to [2]int) (x, y float64) {
	dx, dy := float64(to[0]-from[0]), float64(to[1]-from[1])
	t := 0.5 / max(math.Abs(dx), math.Abs(dy))
	return float64(from[0]) + 0.5 + dx*t, float64(from[1]) + 0.5 + dy*t
}

// cordLine adds a line wd px wide: full alpha along a core 1 px narrower,
// fading to nothing 1 px wider, so it covers wd px and a thin diagonal
// does not stair-step (the batch has no antialiasing).
func cordLine(b *gfx.TriBatch, x0, y0, x1, y1, wd float64, c color.RGBA, a float64) {
	dx, dy := x1-x0, y1-y0
	n := math.Hypot(dx, dy)
	if n == 0 {
		return
	}
	nx, ny := -dy/n, dx/n
	first := uint32(len(b.Verts))
	for k, off := range [4]float64{-(wd + 1) / 2, -(wd - 1) / 2, (wd - 1) / 2, (wd + 1) / 2} {
		al := a
		if k == 0 || k == 3 {
			al = 0
		}
		b.Vert(x0+nx*off, y0+ny*off, 1.5, 1.5, c, al)
		b.Vert(x1+nx*off, y1+ny*off, 1.5, 1.5, c, al)
	}
	for k := uint32(0); k < 3; k++ {
		i := first + k*2
		b.Tri(i, i+1, i+2)
		b.Tri(i+1, i+3, i+2)
	}
}
