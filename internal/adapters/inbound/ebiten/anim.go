package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Animation (ART spec §27). Everything here is presentation-only: it
// reads the read model and the presentation clock (r.present) and never
// feeds back into the simulation. Pseudo-randomness comes from mix64 of
// stable keys, so a replay looks the same.

// pose is a transform applied to a whole figure (all of its layers).
type pose struct {
	Dx, Dy float64 // offset in tiles
	Rot    float64 // radians (screen, clockwise) about the pivot
	Px, Py float64 // pivot in tiles from the sprite's tile top-left
	Sx, Sy float64 // scale about the pivot; 0 means 1
	Cs     colorScale
	Flash  float32 // 0..1 hit flash
	Skew   float64 // horizontal shear about the pivot (wind sway)
}

// Geo is the pose part of a sprite transform, for a tile whose
// top-left is at screen (sx, sy).
func (p pose) Geo(sx, sy, tile float64) ebiten.GeoM {
	px, py := sx+p.Px*tile, sy+p.Py*tile
	kx, ky := p.Scale()
	g := ebiten.GeoM{}
	g.Translate(-px, -py)
	if p.Skew != 0 {
		g.Skew(p.Skew, 0)
	}
	g.Scale(kx, ky)
	g.Rotate(p.Rot)
	g.Translate(px+p.Dx*tile, py+p.Dy*tile)
	return g
}

func (p pose) Scale() (float64, float64) {
	sx, sy := p.Sx, p.Sy
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	return sx, sy
}

// figurePivot is where a standing animal's feet meet the ground, in
// tiles from its tile's top-left (pawns: pawnFeetX, pawnFeetY).
const figurePivotX, figurePivotY = 0.5, 0.55

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

func easeOut(t float64) float64 {
	t = clamp01(t)
	return 1 - (1-t)*(1-t)
}

// easeOutBack overshoots slightly: items "pop" in.
func easeOutBack(t float64) float64 {
	t = clamp01(t)
	const c1 = 1.70158
	const c3 = c1 + 1
	return 1 + c3*math.Pow(t-1, 3) + c1*math.Pow(t-1, 2)
}

// hash01 is a stable pseudo-random value in [0, 1) for a key.
func hash01(keys ...uint64) float64 {
	h := uint64(0x9E3779B97F4A7C15)
	for _, k := range keys {
		h = mix64(h ^ k)
	}
	return float64(h>>11) / float64(1<<53)
}

// --- walk cycle -------------------------------------------------------

// Gait (ART §27.1): a pawn's cycle (two footfalls) spans strideLen tiles
// of stride. Pawns have no walk frames: walkFrames only phases the bob so
// its peaks fall where the old cycle's contact poses were (01 and 07). An
// animal trot has trotFrames frames and its own ground travel per cycle
// (metadata "gait", PLAY-022; animal_figure.go).
const (
	walkFrames = 12
	trotFrames = 8
	strideLen  = 1.6
	// contactEvery is the stride between two foot contacts.
	contactEvery = strideLen / 2

	// gait amplitudes, in tiles / radians at normal walking speed
	bobAmp   = pawnBobPx * 1.4
	swayAmp  = 0.015
	leanAmp  = 0.06
	leanBack = 0.05
	minBlend = 0.05 // below this the static pose is used
)

// trotSuffix is the animal trot frame suffix ("_walk01".."_walk08") for
// phase ph, in cycles (any real; the fraction picks the frame).
func trotSuffix(ph float64) string {
	return fmt.Sprintf("_walk%02d", trotFrame(ph)+1)
}

// trotFrame is the 0-based trot frame at phase ph.
func trotFrame(ph float64) int {
	f := ph - math.Floor(ph)
	return min(int(f*trotFrames), trotFrames-1)
}

// gaitScale is how strongly the gait shows: speed relative to a straight
// step, capped (diagonals are faster), times the walk blend.
func gaitScale(m moveState) float64 {
	return math.Min(m.Speed*float64(game.TicksPerCell), 1) * clamp01(m.Blend)
}

// humps is |cos| with two humps per cycle whose peaks sit on the centres
// of frames 01 and 07 (foot contact); n is the cycle's frame count.
func humps(stride float64, n int) float64 {
	cyc := stride/strideLen - 0.5/float64(n)
	return math.Abs(math.Cos(2 * math.Pi * cyc))
}

// bobRaise is how far (tiles, up) the body is lifted: A*|cos| scaled by
// speed and walk blend, peaking at foot contact.
func bobRaise(stride, speed, blend float64) float64 {
	return bobAmp * humps(stride, walkFrames) * gaitScale(moveState{Speed: speed, Blend: blend})
}

// gaitSway is the lateral body sway (tiles) for N/S walkers: weight
// shifts onto the stance foot once per cycle.
func gaitSway(m moveState) float64 {
	return swayAmp * math.Sin(2*math.Pi*m.Stride/strideLen) * gaitScale(m)
}

// gaitLean is the forward lean (radians, clockwise) of an E/W walker:
// proportional to speed, and a short lean back while slowing to a stop.
func gaitLean(f facing, m moveState) float64 {
	var k float64
	switch {
	case m.Moving:
		k = leanAmp * gaitScale(m)
	case m.Blend > 0:
		k = -leanBack * clamp01(m.Blend)
	}
	switch f {
	case faceEast:
		return k
	case faceWest:
		return -k
	}
	return 0
}

// walkPose is the gait's share of a figure transform.
func walkPose(f facing, m moveState) pose {
	ps := standPose()
	if m.Blend >= minBlend {
		ps.Dy = -bobRaise(m.Stride, m.Speed, m.Blend)
	}
	if f == faceNorth || f == faceSouth {
		ps.Dx = gaitSway(m)
	}
	ps.Rot = gaitLean(f, m)
	return ps
}

// blockedFacing turns a blocked creature towards whatever is in its way.
func blockedFacing(p game.PawnView, pawns []game.PawnView, animals []game.AnimalView, prev facing) facing {
	if p.BlockedBy == 0 {
		return prev
	}
	for _, q := range pawns {
		if q.ID == p.BlockedBy {
			return facingFor(q.X-p.X, q.Y-p.Y, prev)
		}
	}
	for _, a := range animals {
		if a.ID == p.BlockedBy {
			return facingFor(a.X-p.X, a.Y-p.Y, prev)
		}
	}
	return prev
}

// --- footstep dust ---------------------------------------------------

// contactsCrossed counts foot contacts (every contactEvery of stride)
// passed going from one stride to the next.
func contactsCrossed(prev, cur float64) int {
	if cur <= prev {
		return 0
	}
	return int(math.Floor(cur/contactEvery) - math.Floor(prev/contactEvery))
}

// footPoint is where the foot of a given contact parity lands, in world
// tiles, for a creature whose tile corner is (x, y).
func footPoint(x, y float64, f facing, parity int) [2]float64 {
	side := 0.12
	if parity%2 != 0 {
		side = -side
	}
	if f == faceEast || f == faceWest {
		return [2]float64{x + 0.5 + side, y + 0.62}
	}
	return [2]float64{x + 0.5 + side, y + 0.6}
}

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

// easeFrames is how long bed entry and carry lifts take, in present frames.
const easeFrames = 12

// smoothEase is smoothstep over frames present frames since since.
func smoothEase(now, since uint64, frames float64) float64 {
	if now <= since {
		return 0
	}
	t := clamp01(float64(now-since) / frames)
	return t * t * (3 - 2*t)
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

// lerpPose blends two poses (same pivot assumed from a).
func lerpPose(a, b pose, k float64) pose {
	if k <= 0 {
		return a
	}
	if k >= 1 {
		return b
	}
	l := func(x, y float64) float64 { return x*(1-k) + y*k }
	ax, ay := a.Scale()
	bx, by := b.Scale()
	return pose{
		Dx: l(a.Dx, b.Dx), Dy: l(a.Dy, b.Dy), Rot: l(a.Rot, b.Rot),
		Px: a.Px, Py: a.Py,
		Sx: l(ax, bx), Sy: l(ay, by),
		Cs: b.Cs, Flash: b.Flash, Skew: l(a.Skew, b.Skew),
	}
}

// framed returns key+suffix when that frame exists, else key.
func (r *Renderer) framed(key, suffix string) string {
	if suffix == "" {
		return key
	}
	if k := key + suffix; r.art.Has(k) {
		return k
	}
	return key
}

func unit(x, y float64) (float64, float64) {
	n := math.Hypot(x, y)
	if n == 0 {
		return 0, 0
	}
	return x / n, y / n
}

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

// itemPop is the pop-in scale of a ground item.
func (r *Renderer) itemPop(id entity.EntityID) float64 {
	born, ok := r.itemBorn[id]
	if !ok || born == 0 {
		return 1
	}
	return easeOutBack(float64(r.present-born) / 14)
}

// sway is the wind shear for a plant at (x, y): a slow breeze with
// gusts, offset per tile so a field ripples.
func (r *Renderer) sway(x, y int, amount float64) float64 {
	t := float64(r.present)
	gust := 0.6 + 0.4*math.Sin(t/260+float64(x)*0.12)
	return amount * gust * math.Sin(t/38+float64(x)*0.55+float64(y)*0.35)
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
