package ebiten

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
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
	dx, dy float64 // offset in tiles
	rot    float64 // radians (screen, clockwise) about the pivot
	px, py float64 // pivot in tiles from the sprite's tile top-left
	sx, sy float64 // scale about the pivot; 0 means 1
	cs     colorScale
	flash  float32 // 0..1 hit flash
	skew   float64 // horizontal shear about the pivot (wind sway)
}

// geo is the pose part of a sprite transform, for a tile whose
// top-left is at screen (sx, sy).
func (p pose) geo(sx, sy, tile float64) ebiten.GeoM {
	px, py := sx+p.px*tile, sy+p.py*tile
	kx, ky := p.scale()
	g := ebiten.GeoM{}
	g.Translate(-px, -py)
	if p.skew != 0 {
		g.Skew(p.skew, 0)
	}
	g.Scale(kx, ky)
	g.Rotate(p.rot)
	g.Translate(px+p.dx*tile, py+p.dy*tile)
	return g
}

func (p pose) scale() (float64, float64) {
	sx, sy := p.sx, p.sy
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	return sx, sy
}

// figurePivot is where a standing figure's feet meet the ground, in
// tiles from its tile's top-left (ART: figures stand at y = -0.52).
const figurePivotX, figurePivotY = 0.5, 0.55

// drawSpritePose draws a sprite at tile (wx, wy) transformed by ps.
func (r *Renderer) drawSpritePose(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps pose) {
	sp := r.art.sprite(key)
	if sp == nil {
		return
	}
	img := r.art.image(key, tint)
	if img == nil {
		return
	}
	sx, sy := r.worldToScreenF(wx, wy)
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	if !visible(sx-float32(tile)*2, sy-float32(tile)*2, float32(tile)*5, float32(tile)*5, sw, sh) {
		return
	}
	x, y, s := artOrigin(sp.meta, float64(sx), float64(sy), tile)
	geo := ebiten.GeoM{}
	geo.Scale(s, s)
	geo.Translate(x, y)
	geo.Concat(ps.geo(float64(sx), float64(sy), tile))
	cs := ps.cs
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
		r.drawLit(screen, img, geo, cs, int(math.Floor(wx)), int(math.Floor(wy)))
	}
	if ps.flash > 0 {
		f := ps.flash
		fl := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
		fl.ColorScale.Scale(1.9*f, 0.75*f, 0.7*f, f*0.55)
		screen.DrawImage(img, fl)
	}
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

func clamp01(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
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

// Frame counts and gait length (ART §27.1): a pawn walk cycle is 12 poses
// over strideLen tiles of stride, an animal trot 8; work swings are 6.
const (
	walkFrames = 12
	workFrames = 6
	trotFrames = 8
	strideLen  = 1.6
	// contactEvery is the stride between two foot contacts (frames 01 and 07).
	contactEvery = strideLen / 2

	// gait amplitudes, in tiles / radians at normal walking speed
	bobAmp    = pawnBobPx * 1.4
	swayAmp   = 0.015
	leanAmp   = 0.06
	leanBack  = 0.05
	minBlend  = 0.05 // below this the static pose is used
	trotBobPx = 0.03
)

// cycleIndex is floor(stride/strideLen*n) mod n, also for negative stride.
func cycleIndex(stride float64, n int) int {
	i := int(math.Floor(stride/strideLen*float64(n))) % n
	if i < 0 {
		i += n
	}
	return i
}

// walkIndex is the 0-based pawn walk frame for a stride in tiles.
func walkIndex(stride float64) int { return cycleIndex(stride, walkFrames) }

// walkSuffix is the walk frame suffix ("_walk01".."_walk12") for a mover,
// or "" (the static pose) when it is not walking.
func walkSuffix(m moveState) string {
	if m.Blend < minBlend {
		return ""
	}
	return fmt.Sprintf("_walk%02d", walkIndex(m.Stride)+1)
}

// trotSuffix is the animal trot frame suffix ("_walk01".."_walk08").
func trotSuffix(m moveState) string {
	if m.Blend < minBlend {
		return ""
	}
	return fmt.Sprintf("_walk%02d", cycleIndex(m.Stride, trotFrames)+1)
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

// trotBob is the animal's bounce (tiles, up); a fleeing animal gallops
// with a 1.6x bounce.
func trotBob(m moveState, fleeing bool) float64 {
	b := trotBobPx * humps(m.Stride, trotFrames) * gaitScale(m)
	if fleeing {
		b *= 1.6
	}
	return b
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
	ps := pose{px: figurePivotX, py: figurePivotY}
	if m.Blend >= minBlend {
		ps.dy = -bobRaise(m.Stride, m.Speed, m.Blend)
	}
	if f == faceNorth || f == faceSouth {
		ps.dx = gaitSway(m)
	}
	ps.rot = gaitLean(f, m)
	return ps
}

// headDelta is how far (tiles) the head moved from the static pose in
// this body frame, from the "head" attachments. Hair and hats follow it.
// Missing data (older art) means no offset.
func headDelta(c *artCatalog, f facing, suffix string) (dx, dy float64) {
	base := "pawn/body_" + f.String()
	st, fr := c.sprite(base), c.sprite(base+suffix)
	if suffix == "" || st == nil || fr == nil {
		return 0, 0
	}
	h0, ok0 := st.meta.Attach["head"]
	h1, ok1 := fr.meta.Attach["head"]
	if !ok0 || !ok1 {
		return 0, 0
	}
	return (h1[0] - h0[0]) / artTilePx, (h1[1] - h0[1]) / artTilePx
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
	prev, seen := st.prevStride[id]
	st.prevStride[id] = m.Stride
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
	carryBorn  map[entity.EntityID]uint64  // present frame a patient was lifted
	prevStride map[entity.EntityID]float64 // last stride seen, for contact dust
	prevWork   map[entity.EntityID]float64 // last animT seen while working
	recs       map[[3]uint64]*reactRec
}

func (r *Renderer) animState() *animState {
	if r.anims == nil {
		r.anims = &animState{
			carryBorn:  map[entity.EntityID]uint64{},
			prevStride: map[entity.EntityID]float64{},
			prevWork:   map[entity.EntityID]float64{},
			recs:       map[[3]uint64]*reactRec{},
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

// carryEase is the lift progress of a carried patient (0 on the ground,
// 1 slung); the first call for a patient starts the lift.
func (st *animState) carryEase(id entity.EntityID, now uint64) float64 {
	born, ok := st.carryBorn[id]
	if !ok {
		st.carryBorn[id] = now
		return 0
	}
	return smoothEase(now, born, easeFrames)
}

// dropCarry forgets a patient that is no longer carried.
func (st *animState) dropCarry(id entity.EntityID) { delete(st.carryBorn, id) }

// lerpPose blends two poses (same pivot assumed from a).
func lerpPose(a, b pose, k float64) pose {
	if k <= 0 {
		return a
	}
	if k >= 1 {
		return b
	}
	l := func(x, y float64) float64 { return x*(1-k) + y*k }
	ax, ay := a.scale()
	bx, by := b.scale()
	return pose{
		dx: l(a.dx, b.dx), dy: l(a.dy, b.dy), rot: l(a.rot, b.rot),
		px: a.px, py: a.py,
		sx: l(ax, bx), sy: l(ay, by),
		cs: b.cs, flash: b.flash, skew: l(a.skew, b.skew),
	}
}

// framed returns key+suffix when that frame exists, else key.
func (r *Renderer) framed(key, suffix string) string {
	if suffix == "" {
		return key
	}
	if k := key + suffix; r.art.has(k) {
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
	kind   partKind
	x, y   float64 // tiles
	vx, vy float64 // tiles per frame
	g      float64 // gravity, tiles per frame^2
	grow   float64 // radius growth per frame
	born   uint64
	life   uint64
	size   float64 // radius in tiles
	clr    color.RGBA
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
		p := particle{kind: kind, x: x, y: y, born: r.present, clr: cols[int(h(1)*float64(len(cols)))%len(cols)]}
		switch kind {
		case partChip, partShaving:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.05, -0.03-h(3)*0.03, 0.0035
			p.life, p.size = 22+uint64(h(4)*10), 0.035+h(5)*0.02
		case partDust:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.04, -0.015-h(3)*0.02, 0.0015
			p.life, p.size, p.grow = 30+uint64(h(4)*14), 0.04+h(5)*0.03, 0.0015
		case partLeaf:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.03, -0.02-h(3)*0.015, 0.0012
			p.life, p.size = 34+uint64(h(4)*14), 0.035
		case partSoil:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.035, -0.025-h(3)*0.015, 0.003
			p.life, p.size = 18+uint64(h(4)*8), 0.03
		case partSteam, partSmoke:
			p.vx, p.vy = (h(2)-0.5)*0.006, -0.012-h(3)*0.006
			p.life, p.size, p.grow = 50+uint64(h(4)*20), 0.06, 0.003
		case partBlood, partIchor:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.06, -0.025-h(3)*0.03, 0.004
			p.life, p.size = 18+uint64(h(4)*8), 0.03+h(5)*0.015
		case partStep:
			p.vx, p.vy = (h(2)-0.5)*0.01, -0.004
			p.life, p.size, p.grow = 18, 0.05, 0.004
		case partEmber:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.012, -0.016-h(3)*0.01, -0.0001
			p.life, p.size = 36+uint64(h(4)*24), 0.014
		case partSpark:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.09, -0.04-h(3)*0.04, 0.005
			p.life, p.size = 8+uint64(h(4)*6), 0.012
		case partCasing:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.02, -0.05-h(3)*0.02, 0.006
			p.life, p.size = 26, 0.016
		case partSawdust:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.03, -0.012-h(3)*0.012, 0.0016
			p.life, p.size = 26+uint64(h(4)*12), 0.018
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
		r.parts[i].vx += ux * speed
		r.parts[i].vy += uy * speed
	}
}

// drawParticles draws live particles and drops expired ones.
func (r *Renderer) drawParticles(screen *ebiten.Image, tile float64) {
	b := screen.Bounds()
	kept := r.parts[:0]
	for _, p := range r.parts {
		age := float64(r.present - p.born)
		if r.present < p.born || age >= float64(p.life) {
			continue
		}
		kept = append(kept, p)
		k := age / float64(p.life)
		x := p.x + p.vx*age
		y := p.y + p.vy*age + 0.5*p.g*age*age
		rad := (p.size + p.grow*age) * tile
		alpha := 1 - k
		switch p.kind {
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
		vector.FillCircle(screen, sx, sy, float32(rad), fade(p.clr, alpha), true)
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
	st := r.animState()
	for _, p := range pawns {
		if p.Lying || p.CarriedBy != 0 {
			continue
		}
		r.stepDust(st, p.ID, r.pawnMotion(p.ID), false)
	}
	for _, an := range w.Animals {
		if an.Mode != "wandering" {
			r.stepDust(st, an.ID, r.animalMotion(an.ID), true)
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
	live := make(map[entity.EntityID]bool, len(w.Items))
	for _, it := range w.Items {
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
	for _, p := range r.drawPawns {
		if p.Acting != "work at stove" {
			continue
		}
		b, ok := buildingAt(r.drawW, p.ActX, p.ActY)
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
		if r.present >= p.born && r.present-p.born < p.life {
			kept = append(kept, p)
		}
	}
	r.parts = kept
}
