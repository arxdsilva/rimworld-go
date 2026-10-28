package ebiten

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Animation (ART spec §27): the drawing half — posed sprites, particles,
// footstep dust, the work glow and ground-item pops. The pure half (poses,
// gaits, easing, motion tracks and the animT clock) is render/motion.
// Everything here is presentation-only: it reads the read model and the
// presentation clock (r.present) and never feeds back into the
// simulation. Pseudo-randomness comes from mix64 of stable keys, so a
// replay looks the same.

// vec is a point in world tiles (combat marks, held items, turrets).
type vec struct{ X, Y float64 }

// drawSpritePose draws a sprite at tile (wx, wy) transformed by ps.
func (r *Renderer) drawSpritePose(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps pose) {
	m := r.art.MetaOf(key) // no decode before the cull
	if m == nil {
		return
	}
	sx, sy := r.worldToScreenF(wx, wy)
	geo := spritePoseGeo(*m, float64(sx), float64(sy), tile, ps)
	// cull on the posed canvas itself (PLAY-022: an XL body reaches 2+
	// tiles past its cell). A silhouette (r.cast) is projected away from
	// the sprite, so its caster list culls instead (creatureCasters).
	if r.cast == nil && !geoVisible(geo, float64(m.Canvas[0]), float64(m.Canvas[1]), screen.Bounds()) {
		return
	}
	img := r.art.Image(key, tint)
	if img == nil {
		return
	}
	cs := ps.Cs
	if cs == (colorScale{}) {
		cs = noCS
	}
	if strings.HasPrefix(key, "overlay/") { // selection rings and markers stay unlit
		op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
		if cs != noCS {
			op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
		}
		screen.DrawImage(img, op)
	} else {
		r.drawLit(screen, img, geo, cs, wx, wy)
	}
	if ps.Flash > 0 {
		f := ps.Flash
		fl := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
		fl.ColorScale.Scale(1.9*f, 0.75*f, 0.7*f, f*0.55)
		screen.DrawImage(img, fl)
	}
}

// spritePoseGeo places a sprite's canvas for a tile whose top-left is at
// screen (sx, sy): its footprint anchor at its own density (artOrigin,
// PLAY-022 ppt), then the pose about its pivot in tiles.
func spritePoseGeo(m artMeta, sx, sy, tile float64, ps pose) ebiten.GeoM {
	x, y, s := artOrigin(m, sx, sy, tile)
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

// stepDust puffs dust at the foot for every contact the stride passed
// since last time. The first sighting only records the stride.
func (r *Renderer) stepDust(st *animState, id entity.EntityID, m moveState, heavy bool) {
	prev, seen := st.PrevStride[id]
	st.PrevStride[id] = m.Stride
	if !seen || !m.Moving || m.Blend < 0.3 {
		return
	}
	n := contactsCrossed(prev, m.Stride)
	for i := 1; i <= n && i <= 2; i++ {
		parity := int(math.Floor(prev/contactEvery)) + i
		fp := footPoint(m.X, m.Y, m.Facing, parity)
		cnt := 1
		if heavy {
			cnt = 2
		}
		r.burst(partStep, fp[0], fp[1], cnt, uint64(id)*977+uint64(parity))
	}
}

// --- presentation state kept beside the Renderer ---------------------

// animState is per-Renderer bookkeeping for the animations here.
type animState struct {
	CarryBorn  map[entity.EntityID]uint64  // present frame a patient was lifted
	PrevStride map[entity.EntityID]float64 // last stride seen, for contact dust
	PrevWork   map[entity.EntityID]float64 // last animT seen while working
	Recs       map[[3]uint64]*reactRec
}

func (r *Renderer) animState() *animState {
	if r.anims == nil {
		r.anims = &animState{
			CarryBorn:  map[entity.EntityID]uint64{},
			PrevStride: map[entity.EntityID]float64{},
			PrevWork:   map[entity.EntityID]float64{},
			Recs:       map[[3]uint64]*reactRec{},
		}
	}
	return r.anims
}

// CarryEase is the lift progress of a carried patient (0 on the ground,
// 1 slung); the first call for a patient starts the lift.
func (st *animState) CarryEase(id entity.EntityID, now uint64) float64 {
	born, ok := st.CarryBorn[id]
	if !ok {
		st.CarryBorn[id] = now
		return 0
	}
	return smoothEase(now, born, easeFrames)
}

// DropCarry forgets a patient that is no longer carried.
func (st *animState) DropCarry(id entity.EntityID) { delete(st.CarryBorn, id) }

// --- particles ----------------------------------------------------------

type partKind uint8

const (
	partNone partKind = iota
	partChip
	partDust
	partLeaf
	partSoil
	partSteam
	partBlood
	partShaving
	partStep
	partSmoke
	partEmber
	partSpark   // pickaxe on rock, blade on blade
	partCasing  // a spent rifle case, ejected sideways
	partSawdust // saw and hammer work on wood
	partIchor   // hive insects' yellow-green ichor (SIM-020)
)

type particle struct {
	Kind   partKind
	X, Y   float64 // tiles
	Vx, Vy float64 // tiles per frame
	G      float64 // gravity, tiles per frame^2
	Grow   float64 // radius growth per frame
	Born   uint64
	Life   uint64
	Size   float64 // radius in tiles
	Clr    color.RGBA
}

const maxParticles = 900

var partColors = map[partKind][]color.RGBA{
	partChip:    {{176, 128, 78, 255}, {140, 98, 58, 255}, {205, 160, 104, 255}},
	partDust:    {{150, 146, 140, 255}, {120, 118, 112, 255}, {176, 172, 164, 255}},
	partLeaf:    {{112, 160, 86, 255}, {86, 132, 66, 255}, {150, 184, 92, 255}},
	partSoil:    {{100, 80, 58, 255}, {84, 66, 48, 255}},
	partSteam:   {{235, 235, 230, 255}},
	partBlood:   {{170, 38, 32, 255}, {140, 28, 24, 255}},
	partShaving: {{214, 178, 120, 255}, {190, 196, 204, 255}},
	partStep:    {{168, 148, 116, 255}},
	partSmoke:   {{200, 196, 186, 255}},
	partEmber:   {{255, 190, 80, 255}, {255, 140, 50, 255}, {255, 220, 130, 255}},
	partSpark:   {{255, 236, 160, 255}, {255, 200, 90, 255}},
	partCasing:  {{214, 170, 70, 255}, {186, 140, 52, 255}},
	partSawdust: {{222, 196, 150, 255}, {200, 172, 126, 255}},
	partIchor:   {{176, 168, 52, 255}, {140, 136, 40, 255}},
}

// burst spawns n particles of a kind at (x, y), deterministic in seed.
func (r *Renderer) burst(kind partKind, x, y float64, n int, seed uint64) {
	if n > 0 && r.fx > 0 && r.fx < 1 { // Options: effects detail
		n = int(math.Ceil(float64(n) * r.fx))
	}
	cols := partColors[kind]
	for i := 0; i < n; i++ {
		if len(r.parts) >= maxParticles {
			return
		}
		h := func(k uint64) float64 { return hash01(seed, uint64(i), k) }
		p := particle{Kind: kind, X: x, Y: y, Born: r.present, Clr: cols[int(h(1)*float64(len(cols)))%len(cols)]}
		switch kind {
		case partChip, partShaving:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.05, -0.03-h(3)*0.03, 0.0035
			p.Life, p.Size = 22+uint64(h(4)*10), 0.035+h(5)*0.02
		case partDust:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.04, -0.015-h(3)*0.02, 0.0015
			p.Life, p.Size, p.Grow = 30+uint64(h(4)*14), 0.04+h(5)*0.03, 0.0015
		case partLeaf:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.03, -0.02-h(3)*0.015, 0.0012
			p.Life, p.Size = 34+uint64(h(4)*14), 0.035
		case partSoil:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.035, -0.025-h(3)*0.015, 0.003
			p.Life, p.Size = 18+uint64(h(4)*8), 0.03
		case partSteam, partSmoke:
			p.Vx, p.Vy = (h(2)-0.5)*0.006, -0.012-h(3)*0.006
			p.Life, p.Size, p.Grow = 50+uint64(h(4)*20), 0.06, 0.003
		case partBlood, partIchor:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.06, -0.025-h(3)*0.03, 0.004
			p.Life, p.Size = 18+uint64(h(4)*8), 0.03+h(5)*0.015
		case partStep:
			p.Vx, p.Vy = (h(2)-0.5)*0.01, -0.004
			p.Life, p.Size, p.Grow = 18, 0.05, 0.004
		case partEmber:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.012, -0.016-h(3)*0.01, -0.0001
			p.Life, p.Size = 36+uint64(h(4)*24), 0.014
		case partSpark:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.09, -0.04-h(3)*0.04, 0.005
			p.Life, p.Size = 8+uint64(h(4)*6), 0.012
		case partCasing:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.02, -0.05-h(3)*0.02, 0.006
			p.Life, p.Size = 26, 0.016
		case partSawdust:
			p.Vx, p.Vy, p.G = (h(2)-0.5)*0.03, -0.012-h(3)*0.012, 0.0016
			p.Life, p.Size = 26+uint64(h(4)*12), 0.018
		}
		r.parts = append(r.parts, p)
	}
}

// burstDir is burst with every particle pushed along (dx, dy) by speed
// tiles per frame: blood sprays away from the attacker, cases fly out
// of the rifle's side.
func (r *Renderer) burstDir(kind partKind, x, y float64, n int, seed uint64, dx, dy, speed float64) {
	from := len(r.parts)
	r.burst(kind, x, y, n, seed)
	ux, uy := unit(dx, dy)
	for i := from; i < len(r.parts); i++ {
		r.parts[i].Vx += ux * speed
		r.parts[i].Vy += uy * speed
	}
}

// drawParticles draws live particles and drops expired ones.
func (r *Renderer) drawParticles(screen *ebiten.Image, tile float64) {
	b := screen.Bounds()
	kept := r.parts[:0]
	for _, p := range r.parts {
		age := float64(r.present - p.Born)
		if r.present < p.Born || age >= float64(p.Life) {
			continue
		}
		kept = append(kept, p)
		k := age / float64(p.Life)
		x := p.X + p.Vx*age
		y := p.Y + p.Vy*age + 0.5*p.G*age*age
		rad := (p.Size + p.Grow*age) * tile
		alpha := 1 - k
		switch p.Kind {
		case partSteam, partSmoke:
			alpha *= 0.45
		case partDust:
			alpha *= 0.6
		case partStep:
			alpha *= 0.3
		}
		if r.lod.tier == lodMap {
			continue // the map tier (< 16 px tiles) draws no decals (lod.go): kept, not drawn
		}
		sx, sy := r.worldToScreenF(x, y)
		if rr := float32(rad) + 1; sx+rr < float32(b.Min.X) || sx-rr > float32(b.Max.X) || sy+rr < float32(b.Min.Y) || sy-rr > float32(b.Max.Y) {
			continue // off screen: kept, not drawn (a vector fill costs a tessellation and a draw call)
		}
		vector.FillCircle(screen, sx, sy, float32(rad), fade(p.Clr, alpha), true)
	}
	r.parts = kept
}

// --- per-frame animation driver ------------------------------------------

// animate runs once per presentation frame after the simulation step:
// footsteps here, then work strikes (work_anim.go) and combat effects
// (combat_anim.go) spawn their particles.
func (r *Renderer) animate(w game.WorldView, pawns []game.PawnView, running bool) {
	if !running {
		return
	}
	r.emitFireParticles(w)
	r.emitBlazeParticles(w) // fire_world_draw.go
	st := r.animState()
	for _, p := range pawns {
		if p.Lying || p.CarriedBy != 0 {
			continue
		}
		r.stepDust(st, p.ID, r.pawnMotion(p.ID), false)
	}
	for i := range w.Animals {
		an := &w.Animals[i]
		switch {
		case an.Mode == "wandering":
		case r.lod.inView(an.X, an.Y, motionViewPad, motionViewPad):
			r.stepDust(st, an.ID, r.animalMotion(an.ID), true)
		default: // no motion track off screen (syncAnimals): start afresh
			delete(st.PrevStride, an.ID)
		}
	}
	r.animateWork(w, pawns)
	r.animateCombat(w, pawns)
}

// syncItems remembers when each ground item first appeared, so new
// drops pop in. Items present at the first sync never pop.
func (r *Renderer) syncItems(w game.WorldView) {
	if r.itemBorn == nil {
		r.itemBorn = map[entity.EntityID]uint64{}
	}
	if r.itemLive == nil {
		r.itemLive = map[entity.EntityID]bool{}
	}
	live := r.itemLive // reused: one set per update, no allocation
	clear(live)
	for i := range w.Items {
		it := &w.Items[i]
		live[it.ID] = true
		if _, ok := r.itemBorn[it.ID]; !ok {
			if r.itemsInit {
				r.itemBorn[it.ID] = r.present
			} else {
				r.itemBorn[it.ID] = 0
			}
		}
	}
	for id := range r.itemBorn {
		if !live[id] {
			delete(r.itemBorn, id)
		}
	}
	r.itemsInit = true
}

// drawWorkGlow flickers a warm glow over stoves that are cooking.
func (r *Renderer) drawWorkGlow(screen *ebiten.Image, tile float64) {
	for _, p := range r.view.Pawns {
		if p.Acting != "work at stove" {
			continue
		}
		b, ok := buildingAt(r.view.W, p.ActX, p.ActY)
		if !ok {
			continue
		}
		t := float64(r.present + uint64(p.ID)*31)
		f := 0.7 + 0.3*math.Sin(t*0.9)*math.Sin(t*0.37+1)
		cx := float64(b.X) + float64(max(b.W, 1))*0.35
		cy := float64(b.Y) + float64(max(b.H, 1))*0.5 - 0.35
		sx, sy := r.worldToScreenF(cx, cy)
		vector.FillCircle(screen, sx, sy, float32(tile*0.42), fade(color.RGBA{255, 170, 70, 255}, 0.2*f), true)
		vector.FillCircle(screen, sx, sy, float32(tile*0.18), fade(color.RGBA{255, 210, 120, 255}, 0.35*f), true)
	}
}

// drawParticlesNoScreen ages particles without drawing (tests).
func (r *Renderer) drawParticlesNoScreen() {
	kept := r.parts[:0]
	for _, p := range r.parts {
		if r.present >= p.Born && r.present-p.Born < p.Life {
			kept = append(kept, p)
		}
	}
	r.parts = kept
}
