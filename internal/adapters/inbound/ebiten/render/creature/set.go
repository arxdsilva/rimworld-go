// Package creature draws pawns and animals: figure layers and poses, the
// animal body player, held items, work and combat animation, bursts and
// particles, graves and pyres, the overlays above creatures (hunt marks,
// chase lines, name plates, health bars), floating text and the attack
// traces. It reads the frame's *render.View, the motion state, the
// painter and the light, never the renderer (SOLID_GO.md §7); what it
// needs of the root (a pawn's look and blood, its visual record, sounds,
// marks) comes through Host (deps.go).
package creature

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Set is the creature layer: the animation bookkeeping (carry lifts,
// contact dust, work strikes, tracked attack reactions), the particles,
// bursts and floating numbers, and the item pops. Everything here is
// presentation-only: it reads the read model and the presentation clocks
// and never feeds back into the simulation.
type Set struct {
	CarryBorn  map[entity.EntityID]uint64  // present frame a patient was lifted
	PrevStride map[entity.EntityID]float64 // last stride seen, for contact dust
	PrevWork   map[entity.EntityID]float64 // last animT seen while working
	Recs       map[[3]uint64]*reactRec

	// Parts, Floaters and ItemBorn are the live particles, the floating
	// numbers and each ground item's first frame. They point at the
	// renderer's own fields, which other root code still reads (ARCH-004:
	// they move here when the renderer's struct is trimmed).
	Parts    *[]Particle
	Floaters *[]Floater
	ItemBorn *map[entity.EntityID]uint64

	// Host is what the layer reads of the root (deps.go).
	Host Host
	// Paint draws the plain sprites (graves, pyres, bones).
	Paint *sprite.Painter

	mo  *motion.State[Body]
	lit *light.Lit

	itemLive  map[entity.EntityID]bool // SyncItems' reused live set
	itemsInit bool
	seenTrace map[[3]uint64]bool // traces whose effects already spawned
	bursts    BurstTracker
	layers    []Layer // FigureLayers' stack (mark and release)
	worn      WornKeys
	burstBuf  []BurstEvent // EmitWorldBursts' reused events
	rectOp    ebiten.DrawImageOptions
}

// New is the creature layer over the painter, the renderer's motion state
// and its light layer. Host, Parts, Floaters and ItemBorn are set by the
// root before the first use.
func New(p *sprite.Painter, mo *motion.State[Body], lit *light.Lit) *Set {
	return &Set{
		CarryBorn:  map[entity.EntityID]uint64{},
		PrevStride: map[entity.EntityID]float64{},
		PrevWork:   map[entity.EntityID]float64{},
		Recs:       map[[3]uint64]*reactRec{},
		Parts:      new([]Particle),
		Floaters:   new([]Floater),
		ItemBorn:   new(map[entity.EntityID]uint64),
		Paint:      p,
		mo:         mo,
		lit:        lit,
		itemLive:   map[entity.EntityID]bool{},
		seenTrace:  map[[3]uint64]bool{},
	}
}

// Visual is the part of a creature's visual record (the root's per-frame
// facing and lying state) the figures read.
type Visual struct {
	Facing     art.Facing
	LyingSince uint64 // present frame it went down
}

// Layer is one sprite of a pawn's figure, bottom first.
type Layer struct {
	Key  string
	Tint color.NRGBA
}

// Cull is the frame's culling for creature overlays and effects: the
// visible tiles, the level of detail and the pads around them.
type Cull struct {
	X0, Y0, X1, Y1 int  // the visible tiles, inclusive
	ViewSet        bool // a frame has set the view; unset, every tile counts
	MapTier        bool // the map tier (< 16 px tiles) draws no decals
	Pad, PadS      int  // an animal's pads at this tile size (sides, south)
	MotionPad      int  // animals within this many tiles keep a motion track
	// MinTile is the smallest tile size creatures are drawn at.
	MinTile float64
	// White is a white pixel for solid rectangles (nil: none drawn).
	White *ebiten.Image
}

// InView reports tile (x, y) within pad tiles of the view (south by
// padS); before a frame has set the view every tile counts.
func (cl *Cull) InView(x, y, pad, padS int) bool {
	return !cl.ViewSet || x >= cl.X0-pad && x <= cl.X1+pad && y >= cl.Y0-pad && y <= cl.Y1+padS
}

// motionOf is a creature's motion at the frame's animT; false without a
// track.
func (c *Set) motionOf(v *render.View, id entity.EntityID) (motion.MoveState, bool) {
	return c.mo.MotionOf(id, v.AnimT)
}

// motion is a creature's on-screen motion at the frame's animT.
func (c *Set) motion(v *render.View, id entity.EntityID) motion.MoveState {
	m, _ := c.mo.MotionOf(id, v.AnimT)
	return m
}

// pawnAt is where a pawn is drawn: its motion at animT, or its tile
// before the first sync.
func (c *Set) pawnAt(v *render.View, p game.PawnView) (float64, float64) {
	m, ok := c.motionOf(v, p.ID)
	if !ok {
		return float64(p.X), float64(p.Y)
	}
	return m.X, m.Y
}

// solidRect fills a rectangle with the white pixel (a sprite draw, not a
// path fill, so it batches with the sprites around it).
func (c *Set) solidRect(cl *Cull, dst *ebiten.Image, x, y, w, h float32, clr color.Color) {
	if w <= 0 || h <= 0 || cl.White == nil {
		return
	}
	op := &c.rectOp
	op.GeoM.Reset()
	op.GeoM.Scale(float64(w), float64(h))
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Reset()
	op.ColorScale.ScaleWithColor(clr)
	dst.DrawImage(cl.White, op)
}

// figureLayers is p's figure in art facing fs from the root, kept on the
// layer stack until release(mark): nested figures (a rescuer and the
// patient on its back) stack their layers without allocating.
func (c *Set) figureLayers(p game.PawnView, fs string) []Layer {
	from := len(c.layers)
	c.layers = c.Host.FigureLayers(c.layers, p, fs)
	var cat *art.Catalog
	if c.Paint != nil {
		cat = c.Paint.Art
	}
	c.layers = c.worn.Dress(c.layers, from, cat, c.Paint, &p, fs) // worn.go: armor and helmets (DEF-2)
	return c.layers[from:len(c.layers):len(c.layers)]
}

// mark and release bracket figureLayers calls.
func (c *Set) mark() int        { return len(c.layers) }
func (c *Set) release(mark int) { c.layers = c.layers[:mark] }
