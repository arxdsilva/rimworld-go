// Package overlay draws the map overlays over the ground and under or over
// the objects: blood and dirt stains, fog, roof, home area, plan marks,
// paths, zones and their labels, bridges, turret ranges, arrivals, fire,
// the power grid and its badges, the snow-clearing and allowed areas. Each
// overlay is a toggle the HUD flips and a tool may force. A Set reads the
// frame's *render.View, its painter and its light layer, never the
// renderer (SOLID_GO.md §7); what it needs of the renderer's other layers
// (sound, particles, the procedural flame, building sprite keys, the
// weather flash) comes in through Host, wired once.
package overlay

import (
	"image"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Toggle is one overlay's visibility: On is the player's toggle, Forced
// holds it on while a tool that needs it is in hand.
type Toggle struct{ On, Forced bool }

// Shown reports the overlay is showing (toggled or forced).
func (t Toggle) Shown() bool { return t.On || t.Forced }

// Toggle is the overlay's key and HUD button action.
func (t *Toggle) Toggle() { t.On = !t.On }

// Particle is a particle the fires emit (the renderer's particle kinds).
type Particle uint8

const (
	Ember Particle = iota
	Smoke
)

// Host is what the overlays ask of the renderer's other layers, wired
// once by the renderer: plain funcs, a few calls a frame.
type Host struct {
	// Cue sends a positioned sound cue.
	Cue func(id string, x, y, vol float64)
	// Burst emits n particles of kind p at world (x, y).
	Burst func(p Particle, x, y float64, n int, seed uint64)
	// Flame draws the procedural flame at screen point (sx, sy).
	Flame func(dst *ebiten.Image, sx, sy float32, t, tile, k float64)
	// Puff draws steam puff frame f at screen point (sx, sy), unlit.
	Puff func(dst *ebiten.Image, f int, sx, sy float32, tile, k float64, cs sprite.ColorScale) bool
	// PuffAt is the puff frame and alpha t ticks into an exhaust period
	// (ok false once its frames have played).
	PuffAt func(t uint64) (frame int, alpha float64, ok bool)
	// BuildKey is the world sprite of a building kind.
	BuildKey func(kind, facing string, mask int, ew, open bool) string
	// HitAge is how many ticks ago creature id was hit (ok false: never).
	HitAge func(id entity.EntityID, hitTick uint64, x, y int) (float64, bool)
	// Flash shows a sim lightning strike of tick: the flash and the
	// thunder that rolls in after it.
	Flash func(tick uint64)
}

// Chunks is the frame's chunk lists and visible tile range, the world
// pipeline's (lod.go): the overlays walk only the chunks in view.
type Chunks struct {
	Views              []game.ChunkView
	VX0, VY0, VX1, VY1 int
	ViewSet            bool // a frame has set the view; before, every tile counts
	Live               bool // the live tier (48 px a tile and up)
	WholeMap           bool // the whole-map bake tier
	Baking             bool // drawing into a bake: no live parts
}

// InView reports tile (x, y) is within pad tiles of the view (south by
// padS), or that no frame has set the view yet (a pass drawn alone).
func (ch *Chunks) InView(x, y, pad, padS int) bool {
	return !ch.ViewSet || x >= ch.VX0-pad && x <= ch.VX1+pad && y >= ch.VY0-pad && y <= ch.VY1+padS
}

// Links is the renderer's linked-building grid over LinkKinds.
type Links = sprite.LinkGrid[LinkKinds]

// base is what every overlay layer shares: the painter, the light layer,
// the renderer's hooks and the batches' white texel.
type base struct {
	P    *sprite.Painter
	Lit  *light.Lit
	Host Host

	white  *ebiten.Image           // the batches' white texel
	pixel  *ebiten.Image           // solidRect's white pixel
	rectOp ebiten.DrawImageOptions // solidRect's, reused
}

// Set is the renderer's map overlays: their toggles, caches and the
// three larger layers (zones, fires, the power grid).
type Set struct {
	*base

	Roof, Home, Plan, Power, SnowClear Toggle
	Fertility                          Toggle // growing_draw.go: ground fertility and growing zones

	Area     AreaOverlay        // the armed or hovered allowed area (area_overlay.go)
	RoofDraw RoofDrawState      // eaves and the Roof overlay (roof_draw.go)
	HomeDraw HomeDrawState      // the Home overlay's cached outline (home_draw.go)
	PlanDraw PlanDrawState      // the Plan overlay's cached geometry (plan_draw.go)
	SnowDraw SnowClearDrawState // the snow clearing overlay (snow_clear_draw.go)
	Fog      FogCache           // fog_draw.go
	Bridges  BridgeCache        // bridge_draw.go: the foundation layer's cell set
	Caskets  CasketMemo         // vault_draw.go: sealed state seen last frame

	Zones  Zones  // zones_draw.go: fills, outlines and labels
	Fires  Fires  // fire.go, fire_world_draw.go: flames, ash, strikes
	Combat Combat // combat_draw.go: weapon range, line of sight, cover, projectiles
	Wiring Wiring // power_draw.go, power_overlay.go, turret_draw.go: the power layer
}

// Zones draws the zones: their fills, outlines and labels.
type Zones struct {
	*base
	geoms   map[uint16]*ZoneGeom // the outline cache
	Hide    bool                 // the zones overlay is off: no zone fill, outline or name
	ShowAll bool                 // the zones overlay: every zone, hidden and shelf zones too, each in its palette colour
	Batch   gfx.TriBatch         // every zone outline of a frame, one draw
	plants  map[[2]int]bool      // cells holding their zone's own plant, this frame (PLAY-020)
	verts   []ebiten.Vertex      // zoneFill's reused batch
	idx     []uint32
}

// Fires draws the campfires and the burning cells, their ash and the sim's
// lightning, and places the blaze's crackle.
type Fires struct {
	*base
	fx FireFx // clusters and shown strikes
}

// Wiring draws the power layer: conduits, device glows and parts, the
// no-power badges, the power overlay and the turret's gun and range.
type Wiring struct {
	*base
	Nets  PowerOverlay // the power overlay's net colours and transmitters
	Parts PowerDraw    // turbine blade phases, glow keys, vent covers
}

// New is the overlays of painter p and light layer lit, the Plan overlay
// on (it shows by default).
func New(p *sprite.Painter, lit *light.Lit) *Set {
	b := &base{P: p, Lit: lit}
	return &Set{base: b, Plan: Toggle{On: true}, Zones: Zones{base: b}, Fires: Fires{base: b}, Wiring: Wiring{base: b}, Combat: Combat{base: b}}
}

// DrawPowerOverlay draws the power overlay when it is shown (Wiring.DrawOverlay).
func (o *Set) DrawPowerOverlay(v *render.View, ch *Chunks, links *Links, screen *ebiten.Image, w *game.WorldView, sw, sh int, tile float64) {
	if o.Power.Shown() {
		o.Wiring.DrawOverlay(v, ch, links, screen, w, sw, sh, tile)
	}
}

// whiteImg is a white texel at (1.5, 1.5), for the triangle batches.
func (o *base) whiteImg() *ebiten.Image {
	if o.white == nil {
		img := ebiten.NewImage(3, 3)
		img.Fill(color.White)
		o.white = img.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
	}
	return o.white
}

// solidRect fills the screen rectangle x, y, w, h with c as one sprite
// draw of a white pixel (no vector path fill: the overlay's marks are
// many and small).
func (o *base) solidRect(dst *ebiten.Image, x, y, w, h float32, c color.Color) {
	if w <= 0 || h <= 0 {
		return
	}
	if o.pixel == nil {
		o.pixel = ebiten.NewImage(1, 1)
		o.pixel.Fill(color.White)
	}
	op := &o.rectOp
	op.GeoM.Reset()
	op.GeoM.Scale(float64(w), float64(h))
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Reset()
	op.ColorScale.ScaleWithColor(c)
	dst.DrawImage(o.pixel, op)
}

// drawUnlit draws img plain, colour-scaled by cs (glows, markings, puffs).
func drawUnlit(dst, img *ebiten.Image, geo ebiten.GeoM, cs sprite.ColorScale) {
	op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
	if cs != sprite.NoCS && cs != (sprite.ColorScale{}) {
		op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
	}
	dst.DrawImage(img, op)
}

// visibleTiles is the on-map tile range of a sw x sh view at tile pixels
// a tile under v's camera.
func visibleTiles(v *render.View, sw, sh int, tile float64) (x0, y0, x1, y1 int) {
	if tile <= 0 {
		return 0, 0, -1, -1
	}
	ox, oy := v.Cam.OffsetX, v.Cam.OffsetY
	x0, y0 = max(int(math.Floor(ox/tile)), 0), max(int(math.Floor(oy/tile)), 0)
	x1 = min(int(math.Ceil((ox+float64(sw))/tile)), v.MapW-1)
	y1 = min(int(math.Ceil((oy+float64(sh))/tile)), v.MapH-1)
	return
}

// valueNoise is smooth value noise over a unit lattice (the stain
// textures' mottling).
func valueNoise(fx, fy float64, salt uint64) float64 {
	ix, iy := math.Floor(fx), math.Floor(fy)
	u, w := motion.Smoothstep01(fx-ix), motion.Smoothstep01(fy-iy)
	at := func(dx, dy float64) float64 { return motion.Hash01(uint64(int64(ix+dx)), uint64(int64(iy+dy)), salt) }
	top := at(0, 0) + (at(1, 0)-at(0, 0))*u
	bot := at(0, 1) + (at(1, 1)-at(0, 1))*u
	return top + (bot-top)*w
}

// drawIconCS draws img size logical px wide at logical (x, y), multiplied
// by cs.
func drawIconCS(dst, img *ebiten.Image, x, y, size float64, cs sprite.ColorScale) {
	if img == nil {
		return
	}
	s := size / float64(img.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	if cs != sprite.NoCS {
		op.ColorScale.Scale(cs[0], cs[1], cs[2], cs[3])
	}
	gfx.DrawImage(dst, img, op)
}

// onScreenTile reports whether a tile near (x, y) may be visible; generous
// (a vent's puff rises about three tiles).
func onScreenTile(v *render.View, screen *ebiten.Image, x, y int, tile float64) bool {
	sx, sy := v.WorldToScreenF(float64(x), float64(y))
	m := tile * 5
	b := screen.Bounds()
	return float64(sx) > -m && float64(sy) > -m && float64(sx) < float64(b.Dx())+m && float64(sy) < float64(b.Dy())+m*2
}
