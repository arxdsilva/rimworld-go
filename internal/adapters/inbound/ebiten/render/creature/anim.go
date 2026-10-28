package creature

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Animation (ART spec §27): the drawing half — posed sprites, particles,
// footstep dust, the work glow and ground-item pops. The pure half (poses,
// gaits, easing, motion tracks and the animT clock) is render/motion.
// Everything here is presentation-only: it reads the read model and the
// presentation clock (v.Present) and never feeds back into the
// simulation. Pseudo-randomness comes from mix64 of stable keys, so a
// replay looks the same.

// Vec is a point in world tiles (combat marks, held items, turrets).
type Vec struct{ X, Y float64 }

// DrawSpritePose draws a sprite at tile (wx, wy) transformed by ps.
func (c *Set) DrawSpritePose(v *render.View, pass sprite.Pass, screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps motion.Pose) {
	m := v.Art.MetaOf(key) // no decode before the cull
	if m == nil {
		return
	}
	sx, sy := v.WorldToScreenF(wx, wy)
	geo := SpritePoseGeo(*m, float64(sx), float64(sy), tile, ps)
	// cull on the posed canvas itself (PLAY-022: an XL body reaches 2+
	// tiles past its cell). A silhouette (pass.Cast) is projected away from
	// the sprite, so its caster list culls instead (creatureCasters).
	if pass.Cast == nil && !geoVisible(geo, float64(m.Canvas[0]), float64(m.Canvas[1]), screen.Bounds()) {
		return
	}
	img := v.Art.Image(key, tint)
	if img == nil {
		return
	}
	cs := ps.Cs
	if cs == (sprite.ColorScale{}) {
		cs = sprite.NoCS
	}
	if strings.HasPrefix(key, "overlay/") { // selection rings and markers stay unlit
		op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
		if cs != sprite.NoCS {
			op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
		}
		screen.DrawImage(img, op)
	} else {
		c.lit.Draw(v, screen, img, geo, cs, wx, wy, pass.Cast, pass.Baking, pass.Blood)
	}
	if ps.Flash > 0 {
		f := ps.Flash
		fl := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
		fl.ColorScale.Scale(1.9*f, 0.75*f, 0.7*f, f*0.55)
		screen.DrawImage(img, fl)
	}
}

// SpritePoseGeo places a sprite's canvas for a tile whose top-left is at
// screen (sx, sy): its footprint anchor at its own density (artOrigin,
// PLAY-022 ppt), then the pose about its pivot in tiles.
func SpritePoseGeo(m art.Meta, sx, sy, tile float64, ps motion.Pose) ebiten.GeoM {
	x, y, s := art.Origin(m, sx, sy, tile)
	geo := ebiten.GeoM{}
	geo.Scale(s, s)
	geo.Translate(x, y)
	geo.Concat(ps.Geo(sx, sy, tile))
	return geo
}

// geoVisible reports whether the w x h source rectangle placed by g
// overlaps the bounds b: its four corners' bounding box, so rotation,
// mirroring and scale are covered.
func geoVisible(g ebiten.GeoM, w, h float64, b image.Rectangle) bool {
	x0, y0 := math.Inf(1), math.Inf(1)
	x1, y1 := math.Inf(-1), math.Inf(-1)
	for _, c := range [4][2]float64{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		x, y := g.Apply(c[0], c[1])
		x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x), math.Max(y1, y)
	}
	return x1 >= float64(b.Min.X) && y1 >= float64(b.Min.Y) && x0 < float64(b.Max.X) && y0 < float64(b.Max.Y)
}

// --- footstep dust ---------------------------------------------------

// StepDust puffs dust at the foot for every contact the stride passed
// since last time. The first sighting only records the stride.
func (c *Set) StepDust(v *render.View, id entity.EntityID, m motion.MoveState, heavy bool) {
	prev, seen := c.PrevStride[id]
	c.PrevStride[id] = m.Stride
	if !seen || !m.Moving || m.Blend < 0.3 {
		return
	}
	n := motion.ContactsCrossed(prev, m.Stride)
	for i := 1; i <= n && i <= 2; i++ {
		parity := int(math.Floor(prev/motion.ContactEvery)) + i
		fp := motion.FootPoint(m.X, m.Y, m.Facing, parity)
		cnt := 1
		if heavy {
			cnt = 2
		}
		c.Burst(v, PartStep, fp[0], fp[1], cnt, uint64(id)*977+uint64(parity))
	}
}

// CarryEase is the lift progress of a carried patient (0 on the ground,
// 1 slung); the first call for a patient starts the lift.
func (c *Set) CarryEase(id entity.EntityID, now uint64) float64 {
	born, ok := c.CarryBorn[id]
	if !ok {
		c.CarryBorn[id] = now
		return 0
	}
	return motion.SmoothEase(now, born, motion.EaseFrames)
}

// DropCarry forgets a patient that is no longer carried.
func (c *Set) DropCarry(id entity.EntityID) { delete(c.CarryBorn, id) }

// --- particles ----------------------------------------------------------

// PartKind is a particle's look and motion.
type PartKind uint8

const (
	PartNone PartKind = iota
	PartChip
	PartDust
	PartLeaf
	PartSoil
	PartSteam
	PartBlood
	PartShaving
	PartStep
	PartSmoke
	PartEmber
	PartSpark   // pickaxe on rock, blade on blade
	PartCasing  // a spent rifle case, ejected sideways
	PartSawdust // saw and hammer work on wood
	PartIchor   // hive insects' yellow-green ichor (SIM-020)
)

// Particle is one short-lived speck: chips, dust, blood, smoke, embers.
type Particle struct {
	Kind   PartKind
	X, Y   float64 // tiles
	Vx, Vy float64 // tiles per frame
	G      float64 // gravity, tiles per frame^2
	Grow   float64 // radius growth per frame
	Born   uint64
	Life   uint64
	Size   float64 // radius in tiles
	Clr    color.RGBA
}

// MaxParticles caps the live particles.
const MaxParticles = 900

var partColors = map[PartKind][]color.RGBA{
	PartChip:    {{176, 128, 78, 255}, {140, 98, 58, 255}, {205, 160, 104, 255}},
	PartDust:    {{150, 146, 140, 255}, {120, 118, 112, 255}, {176, 172, 164, 255}},
	PartLeaf:    {{112, 160, 86, 255}, {86, 132, 66, 255}, {150, 184, 92, 255}},
	PartSoil:    {{100, 80, 58, 255}, {84, 66, 48, 255}},
	PartSteam:   {{235, 235, 230, 255}},
	PartBlood:   {{170, 38, 32, 255}, {140, 28, 24, 255}},
	PartShaving: {{214, 178, 120, 255}, {190, 196, 204, 255}},
	PartStep:    {{168, 148, 116, 255}},
	PartSmoke:   {{200, 196, 186, 255}},
	PartEmber:   {{255, 190, 80, 255}, {255, 140, 50, 255}, {255, 220, 130, 255}},
	PartSpark:   {{255, 236, 160, 255}, {255, 200, 90, 255}},
	PartCasing:  {{214, 170, 70, 255}, {186, 140, 52, 255}},
	PartSawdust: {{222, 196, 150, 255}, {200, 172, 126, 255}},
	PartIchor:   {{176, 168, 52, 255}, {140, 136, 40, 255}},
}

// Burst spawns n particles of a kind at (x, y), deterministic in seed.
func (c *Set) Burst(v *render.View, kind PartKind, x, y float64, n int, seed uint64) {
	if n > 0 && v.Fx > 0 && v.Fx < 1 { // Options: effects detail
		n = int(math.Ceil(float64(n) * v.Fx))
	}
	cols := partColors[kind]
	for i := 0; i < n; i++ {
		if len(*c.Parts) >= MaxParticles {
			return
		}
		h := func(k uint64) float64 { return motion.Hash01(seed, uint64(i), k) }
		p := Particle{Kind: kind, X: x, Y: y, Born: v.Present, Clr: cols[int(h(1)*float64(len(cols)))%len(cols)]}
		switch kind {
		case PartChip, PartShaving:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.05, -0.03-h(3)*0.03, 0.0035
			p.Life, p.Size = 22+uint64(h(4)*10), 0.035+h(5)*0.02
		case PartDust:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.04, -0.015-h(3)*0.02, 0.0015
			p.Life, p.Size, p.Grow = 30+uint64(h(4)*14), 0.04+h(5)*0.03, 0.0015
		case PartLeaf:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.03, -0.02-h(3)*0.015, 0.0012
			p.Life, p.Size = 34+uint64(h(4)*14), 0.035
		case PartSoil:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.035, -0.025-h(3)*0.015, 0.003
			p.Life, p.Size = 18+uint64(h(4)*8), 0.03
		case PartSteam, PartSmoke:
			p.Vx, p.Vy = (h(2)-0.5)*0.006, -0.012-h(3)*0.006
			p.Life, p.Size, p.Grow = 50+uint64(h(4)*20), 0.06, 0.003
		case PartBlood, PartIchor:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.06, -0.025-h(3)*0.03, 0.004
			p.Life, p.Size = 18+uint64(h(4)*8), 0.03+h(5)*0.015
		case PartStep:
			p.Vx, p.Vy = (h(2)-0.5)*0.01, -0.004
			p.Life, p.Size, p.Grow = 18, 0.05, 0.004
		case PartEmber:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.012, -0.016-h(3)*0.01, -0.0001
			p.Life, p.Size = 36+uint64(h(4)*24), 0.014
		case PartSpark:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.09, -0.04-h(3)*0.04, 0.005
			p.Life, p.Size = 8+uint64(h(4)*6), 0.012
		case PartCasing:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.02, -0.05-h(3)*0.02, 0.006
			p.Life, p.Size = 26, 0.016
		case PartSawdust:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.03, -0.012-h(3)*0.012, 0.0016
			p.Life, p.Size = 26+uint64(h(4)*12), 0.018
		}
		*c.Parts = append(*c.Parts, p)
	}
}

// BurstDir is Burst with every particle pushed along (dx, dy) by speed
// tiles per frame: blood sprays away from the attacker, cases fly out
// of the rifle's side.
func (c *Set) BurstDir(v *render.View, kind PartKind, x, y float64, n int, seed uint64, dx, dy, speed float64) {
	from := len(*c.Parts)
	c.Burst(v, kind, x, y, n, seed)
	ux, uy := motion.Unit(dx, dy)
	for i := from; i < len(*c.Parts); i++ {
		ps := *c.Parts
		ps[i].Vx += ux * speed
		ps[i].Vy += uy * speed
	}
}

// DrawParticles draws live particles and drops expired ones.
func (c *Set) DrawParticles(v *render.View, cull *Cull, screen *ebiten.Image, tile float64) {
	b := screen.Bounds()
	kept := (*c.Parts)[:0]
	for _, p := range *c.Parts {
		age := float64(v.Present - p.Born)
		if v.Present < p.Born || age >= float64(p.Life) {
			continue
		}
		kept = append(kept, p)
		k := age / float64(p.Life)
		x := p.X + p.Vx*age
		y := p.Y + p.Vy*age + 0.5*p.G*age*age
		rad := (p.Size + p.Grow*age) * tile
		alpha := 1 - k
		switch p.Kind {
		case PartSteam, PartSmoke:
			alpha *= 0.45
		case PartDust:
			alpha *= 0.6
		case PartStep:
			alpha *= 0.3
		}
		if cull.MapTier {
			continue // the map tier (< 16 px tiles) draws no decals (lod.go): kept, not drawn
		}
		sx, sy := v.WorldToScreenF(x, y)
		if rr := float32(rad) + 1; sx+rr < float32(b.Min.X) || sx-rr > float32(b.Max.X) || sy+rr < float32(b.Min.Y) || sy-rr > float32(b.Max.Y) {
			continue // off screen: kept, not drawn (a vector fill costs a tessellation and a draw call)
		}
		vector.FillCircle(screen, sx, sy, float32(rad), Fade(p.Clr, alpha), true)
	}
	*c.Parts = kept
}

// --- per-frame animation driver ------------------------------------------

// Animate runs once per presentation frame after the simulation step
// (the root emits the fire particles first): footsteps here, then work
// strikes (work.go) and combat effects (combat.go) spawn their particles.
func (c *Set) Animate(v *render.View, cull *Cull, w game.WorldView, pawns []game.PawnView, running bool) {
	if !running {
		return
	}
	for _, p := range pawns {
		if p.Lying || p.CarriedBy != 0 {
			continue
		}
		c.StepDust(v, p.ID, c.motion(v, p.ID), false)
	}
	for i := range w.Animals {
		an := &w.Animals[i]
		switch {
		case an.Mode == "wandering":
		case cull.InView(an.X, an.Y, cull.MotionPad, cull.MotionPad):
			c.StepDust(v, an.ID, c.motion(v, an.ID), true)
		default: // no motion track off screen (the root's syncAnimals): start afresh
			delete(c.PrevStride, an.ID)
		}
	}
	c.AnimateWork(v, w, pawns)
	c.AnimateCombat(v, w, pawns)
}

// SyncItems remembers when each ground item first appeared, so new
// drops pop in. Items present at the first sync never pop.
func (c *Set) SyncItems(v *render.View, w game.WorldView) {
	if *c.ItemBorn == nil {
		*c.ItemBorn = map[entity.EntityID]uint64{}
	}
	if c.itemLive == nil {
		c.itemLive = map[entity.EntityID]bool{}
	}
	live := c.itemLive // reused: one set per update, no allocation
	clear(live)
	for i := range w.Items {
		it := &w.Items[i]
		live[it.ID] = true
		if _, ok := (*c.ItemBorn)[it.ID]; !ok {
			if c.itemsInit {
				(*c.ItemBorn)[it.ID] = v.Present
			} else {
				(*c.ItemBorn)[it.ID] = 0
			}
		}
	}
	for id := range *c.ItemBorn {
		if !live[id] {
			delete(*c.ItemBorn, id)
		}
	}
	c.itemsInit = true
}

// DrawWorkGlow flickers a warm glow over stoves that are cooking.
func (c *Set) DrawWorkGlow(v *render.View, screen *ebiten.Image, tile float64) {
	for _, p := range v.Pawns {
		if p.Acting != "work at stove" {
			continue
		}
		b, ok := c.Host.BuildingAt(v.W, p.ActX, p.ActY)
		if !ok {
			continue
		}
		t := float64(v.Present + uint64(p.ID)*31)
		f := 0.7 + 0.3*math.Sin(t*0.9)*math.Sin(t*0.37+1)
		cx := float64(b.X) + float64(max(b.W, 1))*0.35
		cy := float64(b.Y) + float64(max(b.H, 1))*0.5 - 0.35
		sx, sy := v.WorldToScreenF(cx, cy)
		vector.FillCircle(screen, sx, sy, float32(tile*0.42), Fade(color.RGBA{255, 170, 70, 255}, 0.2*f), true)
		vector.FillCircle(screen, sx, sy, float32(tile*0.18), Fade(color.RGBA{255, 210, 120, 255}, 0.35*f), true)
	}
}

// AgeParticles ages particles without drawing (tests).
func (c *Set) AgeParticles(v *render.View) {
	kept := (*c.Parts)[:0]
	for _, p := range *c.Parts {
		if v.Present >= p.Born && v.Present-p.Born < p.Life {
			kept = append(kept, p)
		}
	}
	*c.Parts = kept
}
