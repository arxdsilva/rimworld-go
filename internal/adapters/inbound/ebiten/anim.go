package ebiten

import (
	"image/color"
	"math"

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
	op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
	cs := ps.cs
	if cs == (colorScale{}) {
		cs = noCS
	}
	if cs != noCS {
		op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
	}
	screen.DrawImage(img, op)
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

// walkCycle maps 8 phases to frame suffixes (two tiles per full cycle,
// so each tile is one stride: the feet stay planted).
var walkCycle = [8]string{"", "_walk1", "_walk2", "_walk1", "", "_walk3", "_walk4", "_walk3"}

// walkPhase is the stride position in tiles walked (fractional).
func walkPhase(v pawnVisual, progress float64) float64 {
	if v.steps == 0 {
		return 0
	}
	return float64(v.steps-1) + progress
}

func walkFrame(phase float64) string {
	i := int(math.Floor(phase*4)) % 8
	if i < 0 {
		i += 8
	}
	return walkCycle[i]
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

// --- reactions from attack traces ---------------------------------------

// attackReaction offsets an actor at (x, y): rifle recoil, melee lunge,
// and knockback plus a flash when struck.
func attackReaction(traces []game.TraceView, x, y int) (dx, dy float64, flash float32) {
	for _, t := range traces {
		if t.FromX == x && t.FromY == y && (t.ToX != x || t.ToY != y) {
			ux, uy := unit(float64(t.ToX-x), float64(t.ToY-y))
			switch {
			case t.Melee && t.Age < 10:
				k := math.Sin(math.Pi * float64(t.Age) / 10)
				dx, dy = dx+ux*0.22*k, dy+uy*0.22*k
			case !t.Melee && t.Age < 6:
				k := 1 - float64(t.Age)/6
				dx, dy = dx-ux*0.08*k, dy-uy*0.08*k
			}
		}
		if t.ToX == x && t.ToY == y && t.Hit && t.Age < 12 && (t.FromX != x || t.FromY != y) {
			k := 1 - float64(t.Age)/12
			ux, uy := unit(float64(x-t.FromX), float64(y-t.FromY))
			dx, dy = dx+ux*0.1*k, dy+uy*0.1*k
			if f := float32(k); f > flash {
				flash = f
			}
		}
	}
	return dx, dy, flash
}

func unit(x, y float64) (float64, float64) {
	n := math.Hypot(x, y)
	if n == 0 {
		return 0, 0
	}
	return x / n, y / n
}

// --- work motion --------------------------------------------------------

// actStyle is how a kind of hands-on work moves and what it throws up.
type actStyle struct {
	period int     // frames per swing
	reach  float64 // lean towards the worked tile, tiles
	burst  partKind
	count  int
	motion motion
	tool   string // sprite key of the held tool, "" for bare hands
}

// motion is the arm pattern for a kind of work.
type motion uint8

const (
	motionNone  motion = iota // no arm frames (eating)
	motionSwing               // overhead: wind-up, strike, recover
	motionReach               // hands working low in front
)

// strikeAt is where in a swing the tool lands (phase 0..1).
const strikeAt = 0.48

func styleFor(act string) (actStyle, bool) {
	switch act {
	case "cut":
		return actStyle{period: 46, reach: 0.1, burst: partChip, count: 4, motion: motionSwing, tool: "tool/axe"}, true
	case "construct", "deconstruct":
		return actStyle{period: 34, reach: 0.08, burst: partChip, count: 3, motion: motionSwing, tool: "tool/hammer"}, true
	case "mine":
		return actStyle{period: 50, reach: 0.12, burst: partDust, count: 5, motion: motionSwing, tool: "tool/pickaxe"}, true
	case "work at workbench":
		return actStyle{period: 30, reach: 0.05, burst: partShaving, count: 2, motion: motionSwing, tool: "tool/hammer"}, true
	case "refuel":
		return actStyle{period: 40, reach: 0.06, burst: partChip, count: 1, motion: motionReach}, true
	case "work at tailoring bench":
		return actStyle{period: 44, reach: 0.05, burst: partShaving, count: 1, motion: motionReach}, true
	case "work at drying rack":
		return actStyle{period: 52, reach: 0.06, burst: partLeaf, count: 1, motion: motionReach}, true
	case "work at butcher table":
		return actStyle{period: 36, reach: 0.06, burst: partBlood, count: 2, motion: motionSwing}, true
	case "harvest":
		return actStyle{period: 44, reach: 0.07, burst: partLeaf, count: 3, motion: motionReach}, true
	case "sow":
		return actStyle{period: 48, reach: 0.05, burst: partSoil, count: 3, motion: motionReach}, true
	case "work at stove":
		return actStyle{period: 40, reach: 0.04, burst: partSteam, count: 1, motion: motionReach}, true
	case "tend":
		return actStyle{period: 60, reach: 0.05, motion: motionReach}, true
	case "eat":
		return actStyle{period: 50, reach: 0.03}, true
	}
	return actStyle{}, false
}

// swingPhase is the 0..1 position within the current swing.
func swingPhase(present, id uint64, period int) float64 {
	if period <= 0 {
		return 0
	}
	return float64((present+id*13)%uint64(period)) / float64(period)
}

// workArms is the arm frame suffix for a motion at a swing phase:
// swings wind up (work1), snap down to the strike (work3) and recover;
// reaching alternates low (work3) and mid (work2).
func workArms(m motion, ph float64) string {
	switch m {
	case motionSwing:
		switch {
		case ph < 0.12:
			return "_work2"
		case ph < 0.40:
			return "_work1"
		case ph < strikeAt:
			return "_work2"
		case ph < 0.78:
			return "_work3"
		default:
			return "_work2"
		}
	case motionReach:
		if ph < 0.55 {
			return "_work3"
		}
		return "_work2"
	}
	return ""
}

// workSwing is the body lean for a swing phase: back a little while
// winding up, hard forward on the strike, easing out.
func workSwing(present, id uint64, period int) float64 {
	ph := swingPhase(present, id, period)
	switch {
	case ph < 0.40:
		return -0.3 * easeOut(ph/0.40)
	case ph < strikeAt:
		return -0.3 + 1.3*easeOut((ph-0.40)/(strikeAt-0.40))
	default:
		return 1 - easeOut((ph-strikeAt)/(1-strikeAt))
	}
}

// workStrike reports the frame where the tool lands.
func workStrike(present, id uint64, period int) bool {
	return period > 0 && (present+id*13)%uint64(period) == uint64(float64(period)*strikeAt)
}

// toolAngle is the held tool's screen rotation (radians, clockwise; the
// sprite points east) for a facing and arm frame.
func toolAngle(f facing, frame string) float64 {
	type key struct {
		f  facing
		fr string
	}
	a := map[key]float64{
		{faceEast, "_work1"}: -2.0, {faceEast, "_work2"}: -0.7, {faceEast, "_work3"}: 0.55,
		{faceSouth, "_work1"}: -1.57, {faceSouth, "_work2"}: -0.45, {faceSouth, "_work3"}: 1.1,
		{faceNorth, "_work1"}: -1.57, {faceNorth, "_work2"}: -0.9, {faceNorth, "_work3"}: 0.9,
	}
	if f == faceWest {
		return -a[key{faceEast, frame}]
	}
	return a[key{f, frame}]
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
}

// burst spawns n particles of a kind at (x, y), deterministic in seed.
func (r *Renderer) burst(kind partKind, x, y float64, n int, seed uint64) {
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
		case partBlood:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.06, -0.025-h(3)*0.03, 0.004
			p.life, p.size = 18+uint64(h(4)*8), 0.03+h(5)*0.015
		case partStep:
			p.vx, p.vy = (h(2)-0.5)*0.01, -0.004
			p.life, p.size, p.grow = 18, 0.05, 0.004
		case partEmber:
			p.vx, p.vy, p.g = (h(2)-0.5)*0.012, -0.016-h(3)*0.01, -0.0001
			p.life, p.size = 36+uint64(h(4)*24), 0.014
		}
		r.parts = append(r.parts, p)
	}
}

// drawParticles draws live particles and drops expired ones.
func (r *Renderer) drawParticles(screen *ebiten.Image, tile float64) {
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
		sx, sy := r.worldToScreenF(x, y)
		vector.FillCircle(screen, sx, sy, float32(rad), fade(p.clr, alpha), true)
	}
	r.parts = kept
}

// --- per-frame animation driver ------------------------------------------

// animate runs once per presentation frame after the simulation step:
// work strikes, footsteps and hit splatter spawn particles.
func (r *Renderer) animate(w game.WorldView, pawns []game.PawnView, running bool) {
	if !running {
		return
	}
	r.emitFireParticles(w)
	for _, p := range pawns {
		st, ok := styleFor(p.Acting)
		if !ok || st.count == 0 || !workStrike(r.present, uint64(p.ID), st.period) {
			continue
		}
		x, y := float64(p.ActX)+0.5, float64(p.ActY)+0.45
		switch p.Acting {
		case "work at stove":
			x, y = float64(p.ActX)+0.7, float64(p.ActY)-0.1
		case "sow", "eat":
			x, y = float64(p.X)+0.5, float64(p.Y)+0.6
		}
		r.burst(st.burst, x, y, st.count, uint64(p.ID)*7919+r.present)
	}
	// hit splatter and muzzle smoke, once per trace
	now := uint64(r.game.Tick())
	for _, t := range w.Traces {
		key := [3]uint64{uint64(t.FromX)<<20 | uint64(t.FromY), uint64(t.ToX)<<20 | uint64(t.ToY), now - uint64(t.Age)}
		if r.seenTrace[key] {
			continue
		}
		r.seenTrace[key] = true
		if t.Hit {
			r.burst(partBlood, float64(t.ToX)+0.5, float64(t.ToY)+0.35, 5, key[0]^key[1]^key[2])
		}
		if !t.Melee {
			ux, uy := unit(float64(t.ToX-t.FromX), float64(t.ToY-t.FromY))
			r.burst(partSmoke, float64(t.FromX)+0.5+ux*0.5, float64(t.FromY)+0.3+uy*0.5, 2, key[2])
		}
	}
	if len(r.seenTrace) > 256 {
		r.seenTrace = map[[3]uint64]bool{}
	}
}

// footstep puffs a little dust where a walker just stepped.
func (r *Renderer) footstep(x, y int, seed uint64, heavy bool) {
	n := 1
	if heavy {
		n = 2
	}
	r.burst(partStep, float64(x)+0.5, float64(y)+0.6, n, seed)
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
