package ebiten

import (
	"cmp"
	"image/color"
	"math"
	"slices"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Presentation of animals, held weapons and attack traces. Everything
// here reads the WorldView/PawnView; nothing writes back.

// syncAnimals advances per-animal presentation state (motion, facing).
// Footstep dust comes from animate, at the feet on contact frames.
func (r *Renderer) syncAnimals(w game.WorldView) {
	tick := r.game.Tick()
	running := r.mo.stepped > 0
	l := &r.lod
	for i := range w.Animals {
		an := &w.Animals[i]
		if !l.inView(an.X, an.Y, motionViewPad, motionViewPad) {
			// far off screen (a big map's wildlife): no motion track; it
			// starts afresh, on its cell, when it nears the view again
			delete(r.last, an.ID)
			continue
		}
		prev, seen := r.last[an.ID]
		v := nextVisualStep(prev, seen, an.X, an.Y, tick, r.present, an.StepTicks)
		k := r.mo.observe(an.ID, animalMoView(*an), v.facing)
		want, moving := k.wantFacing(r.mo.ref)
		if !moving {
			want = faceAttack(pawnVisual{facing: want}, w.Traces, an.X, an.Y).facing
		}
		v.facing = k.turn(want, an.ID, running)
		r.last[an.ID] = v
	}
}

// faceAttack turns an attacker standing at (x, y) towards its latest
// target so weapons and bites point the right way.
func faceAttack(v pawnVisual, traces []game.TraceView, x, y int) pawnVisual {
	for _, t := range traces {
		if t.FromX == x && t.FromY == y && (t.ToX != x || t.ToY != y) {
			v.facing = facingFor(t.ToX-x, t.ToY-y, v.facing)
		}
	}
	return v
}

// interpAnimal is where an animal is drawn: its motion at animT, or its
// tile before the first sync.
func (r *Renderer) interpAnimal(an game.AnimalView) (float64, float64) {
	m, ok := r.motionOf(an.ID)
	if !ok {
		return float64(an.X), float64(an.Y)
	}
	return m.X, m.Y
}

// animalMarkTrace, when set (tests only, never in parallel), sees each mark
// the animal overlays put in the world pass, in physical pixels:
// "hostile-marker" (a, b: the animal's top-left), "chase-line" (a, b the
// animal's centre, c, d the target's), "name-plate" (a, b, c, d: x, y,
// width, height) and "name-text" (a, b: the text's anchor, as handed to
// worldText).
var animalMarkTrace func(kind string, a, b, c, d float32)

// drawAnimalOverlays draws hunt marks and health bars above animals, then
// the sleepers' snores. The snores (vector strokes) go in a pass of their
// own: a vector stroke between two sprite draws flushes the vector batch,
// a path fill per animal (milliseconds a frame with the map in view).
func (r *Renderer) drawAnimalOverlays(screen *ebiten.Image, w game.WorldView, tile float64, sw, sh int) {
	for i := range w.Animals {
		an := &w.Animals[i]
		if !r.lod.inView(an.X, an.Y, 3, 4) { // lod.go: off screen, as the figure pass culls
			continue
		}
		// the map tier (< 16 px tiles) draws no decals (lod.go): a snore's
		// "z" would be under two pixels there
		if an.Sleeping && r.lod.tier != lodMap && r.animalResting(*an) {
			wx, wy := r.interpAnimal(*an)
			r.drawSnoreFor(screen, uint64(an.ID), wx+0.25, wy+0.1-an.Size*0.1, tile)
		}
	}
	r.drawChaseLines(screen, w, tile) // PLAY-017: under the marks
	for i := range w.Animals {
		an := &w.Animals[i]
		if !r.lod.inView(an.X, an.Y, 3, 4) {
			continue
		}
		wx, wy := r.interpAnimal(*an)
		switch {
		case an.Hostile || an.Manhunter: // PLAY-017: every hostile animal, not only manhunters
			if animalMarkTrace != nil {
				sx, sy := r.worldToScreenF(wx, wy)
				animalMarkTrace("hostile-marker", sx, sy, 0, 0)
			}
			r.drawHostileMarker(screen, game.PawnView{}, wx, wy, animalMarkerLift(an.Size), tile)
		case an.Designated:
			spin := pose{px: 0.5, py: 0.5, sx: 0.5, sy: 0.5, rot: float64(r.present) * 0.035}
			r.drawSpritePose(screen, "overlay/designation_hunt", color.NRGBA{}, wx, wy-animalMarkerLift(an.Size), tile, spin)
		case an.TameMark:
			bob := pose{px: 0.5, py: 0.5, sx: 0.5, sy: 0.5}
			r.drawSpritePose(screen, "overlay/designation_tame", color.NRGBA{}, wx, wy-animalMarkerLift(an.Size)-0.05*math.Sin(float64(r.present)*0.08), tile, bob)
		}
		if an.Tame && an.Name != "" && tile >= stackCountMinTile {
			r.drawAnimalName(screen, an.Name, wx+0.5, wy+1.0)
		}
		if an.Health < 100 {
			sx, sy := r.worldToScreenF(wx+0.15, wy-animalBarLift(an.Size))
			bw := float32(tile) * 0.7
			r.solidRect(screen, sx, sy, bw, 4, color.RGBA{30, 24, 20, 220}) // lod.go: a sprite, not a path fill
			r.solidRect(screen, sx, sy, bw*float32(an.Health)/100, 4, healthColor(an.Health))
		}
	}
}

// chaseLineClr is the chase line's colour: the hostile marker's red,
// translucent so it reads without hiding the ground.
var chaseLineClr = color.NRGBA{R: 220, G: 50, B: 40, A: 170}

const (
	// chaseDotStep is the gap between the line's dots, as a share of a
	// tile, and chaseDotSize a dot's side; chaseMaxDots caps a very long
	// line.
	chaseDotStep = 0.22
	chaseDotSize = 0.05
	chaseMaxDots = 120
)

// drawChaseLines draws a dotted red line from every hostile animal to the
// pawn it is chasing, between the positions they are drawn at (world
// pass, physical pixels). Dots are sprite draws of the white pixel
// (solidRect), not a vector stroke, which would flush the batch for every
// line. A line with both ends off screen is skipped.
func (r *Renderer) drawChaseLines(screen *ebiten.Image, w game.WorldView, tile float64) {
	for i := range w.Animals {
		an := &w.Animals[i]
		if !an.Hostile || an.TargetID == 0 {
			continue
		}
		p, ok := chasedPawn(r.drawPawns, an.TargetID) // chase_ui.go
		if !ok || (!r.lod.inView(an.X, an.Y, 3, 4) && !r.lod.inView(p.X, p.Y, 3, 4)) {
			continue
		}
		ax, ay := r.interpAnimal(*an)
		px, py := r.interpWorld(p)
		x0, y0 := r.worldToScreenF(ax+0.5, ay+0.5)
		x1, y1 := r.worldToScreenF(px+0.5, py+0.5)
		if animalMarkTrace != nil {
			animalMarkTrace("chase-line", x0, y0, x1, y1)
		}
		dist := math.Hypot(float64(x1-x0), float64(y1-y0))
		n := min(chaseMaxDots, max(1, int(math.Round(dist/(tile*chaseDotStep)))))
		d := float32(max(2.5, tile*chaseDotSize))
		for k := 0; k <= n; k++ {
			t := float32(k) / float32(n)
			r.solidRect(screen, x0+(x1-x0)*t-d/2, y0+(y1-y0)*t-d/2, d, d, chaseLineClr)
		}
	}
}

// Name plate metrics, in logical pixels like the text they back: the
// padding around the name, the plate's height, and how far the text sits
// above the plate's top edge.
const (
	namePlatePad  = 6
	namePlateH    = 14
	nameTextRaise = 2
)

// drawAnimalName puts a tame animal's name on a dark plate, centred
// under the tile point (wx, wy) in tiles. The anchor is a world
// position (physical pixels): the text goes through worldText, which
// converts it to logical once, and the plate is sized in physical pixels
// from the same logical text size, so both scale with the UI together.
func (r *Renderer) drawAnimalName(screen *ebiten.Image, name string, wx, wy float64) {
	sx, sy := r.worldToScreenF(wx, wy)
	k := float32(uiScale())
	pw, ph := (textW(name, tsSmall)+namePlatePad)*k, float32(namePlateH)*k
	if animalMarkTrace != nil {
		animalMarkTrace("name-plate", sx-pw/2, sy, pw, ph)
		animalMarkTrace("name-text", sx, sy-nameTextRaise*k, 0, 0)
	}
	r.solidRect(screen, sx-pw/2, sy, pw, ph, color.RGBA{16, 12, 8, 150})
	st := tsSmall
	st.Align = alignCenter
	worldText(screen, name, float64(sx), float64(sy-nameTextRaise*k), st)
}

// Trace styling.
var (
	tracerClr = color.RGBA{255, 226, 140, 255}
	flashClr  = color.RGBA{255, 240, 190, 255}
	bloodClr  = color.RGBA{190, 44, 36, 255}
	swingClr  = color.RGBA{245, 240, 230, 255}
)

func fade(c color.RGBA, k float64) color.RGBA {
	if k <= 0 {
		return color.RGBA{}
	}
	if k > 1 {
		k = 1
	}
	return color.RGBA{uint8(float64(c.R) * k), uint8(float64(c.G) * k), uint8(float64(c.B) * k), uint8(float64(c.A) * k)}
}

// Trace effect durations in sim ticks; each also lasts at least
// minReactionFrames present frames (C7).
const (
	tracerTicks = 6
	flashTicks  = 3
	swingTicks  = 10
	bloodTicks  = 12
	puffTicks   = 14
)

// traceAge is the age an effect of dur ticks is drawn at.
func traceAge(rt reactTrace, dur float64) float64 { return reactionAge(rt.sim, rt.frames, dur) }

// traceList is the traces to draw: the tracked ones (kept on screen after
// the sim drops them), or the sim's own when none are tracked (captures).
// Oldest first (newest on top), in a stable order so overlapping effects
// blend the same every frame.
func (r *Renderer) traceList(w game.WorldView) []reactTrace {
	rs := r.animState().reactions(r.present, nil)
	if len(rs) == 0 {
		for _, t := range w.Traces {
			born := float64(r.game.Tick()) - float64(t.Age)
			rs = append(rs, reactTrace{TraceView: t, sim: float64(t.Age), frames: math.Inf(1), born: born})
		}
	}
	slices.SortFunc(rs, func(a, b reactTrace) int {
		ka := [6]float64{-a.sim, -a.frames, float64(a.FromX), float64(a.FromY), float64(a.ToX), float64(a.ToY)}
		kb := [6]float64{-b.sim, -b.frames, float64(b.FromX), float64(b.FromY), float64(b.ToX), float64(b.ToY)}
		for i := range ka {
			if c := cmp.Compare(ka[i], kb[i]); c != 0 {
				return c
			}
		}
		return 0
	})
	return rs
}

// traceSpots maps each occupied cell to where its creature is drawn.
func (r *Renderer) traceSpots(w game.WorldView) map[[2]int]vec { return r.traceSpotsAt(w, nil) }

// traceSpotsAt is traceSpots for the cells in want only (nil: every
// cell): a frame samples the motion of just the creatures its traces
// touch, not of every creature on the map.
func (r *Renderer) traceSpotsAt(w game.WorldView, want map[[2]int]bool) map[[2]int]vec {
	spots := map[[2]int]vec{}
	put := func(x, y int, id entity.EntityID) {
		if want != nil && !want[[2]int{x, y}] {
			return
		}
		if m, ok := r.motionOf(id); ok {
			spots[[2]int{x, y}] = vec{m.X, m.Y}
		}
	}
	for _, p := range r.game.Snapshot() {
		if p.CarriedBy == 0 && !p.Lying {
			put(p.X, p.Y, p.ID)
		}
	}
	for i := range w.Animals {
		put(w.Animals[i].X, w.Animals[i].Y, w.Animals[i].ID)
	}
	return spots
}

// traceEnds is where a trace starts and lands in world tiles: at the drawn
// (interpolated) shooter and target when a creature stands on the cell.
func traceEnds(spots map[[2]int]vec, tr game.TraceView) (from, to vec) {
	from, ok := spots[[2]int{tr.FromX, tr.FromY}]
	if !ok {
		from = vec{float64(tr.FromX), float64(tr.FromY)}
	}
	to, ok = spots[[2]int{tr.ToX, tr.ToY}]
	if !ok {
		to = vec{float64(tr.ToX), float64(tr.ToY)}
	}
	return vec{from.x + 0.5, from.y + 0.3}, vec{to.x + 0.5, to.y + 0.4}
}

// drawTraces renders recent attacks: rifle tracers from the muzzle with
// a muzzle flash and a hit puff, or a trail along the weapon tip for
// melee. They stay visible for at least minReactionFrames frames.
func (r *Renderer) drawTraces(screen *ebiten.Image, w game.WorldView, tile float64) {
	t := float32(tile)
	rs := r.traceList(w)
	if len(rs) == 0 {
		return
	}
	want := make(map[[2]int]bool, 2*len(rs))
	for _, rt := range rs {
		want[[2]int{rt.FromX, rt.FromY}], want[[2]int{rt.ToX, rt.ToY}] = true, true
	}
	spots := r.traceSpotsAt(w, want)
	for _, rt := range rs {
		tr := rt.TraceView
		from, to := traceEnds(spots, tr)
		fx, fy := r.worldToScreenF(from.x, from.y)
		tx, ty := r.worldToScreenF(to.x, to.y)
		if tr.Melee {
			if age := traceAge(rt, swingTicks); age < swingTicks {
				r.drawMeleeTrail(screen, tr, 1-age/swingTicks, tx, ty, t)
			}
			if age := traceAge(rt, bloodTicks); tr.Hit && age < bloodTicks {
				vector.FillCircle(screen, tx, ty, t*0.08*float32(1+math.Floor(age/4)), fade(bloodClr, 1-age/bloodTicks), true)
			}
			continue
		}
		dx, dy := tx-fx, ty-fy
		n := float32(math.Hypot(float64(dx), float64(dy)))
		if n < 1 {
			n = 1
		}
		ux, uy := dx/n, dy/n
		ex, ey := tx, ty
		if !tr.Hit { // a miss flies past, off to one side
			side := float32(1)
			if (tr.FromX+tr.ToY)%2 == 0 {
				side = -1
			}
			ex, ey = tx+ux*t*0.8-uy*t*0.35*side, ty+uy*t*0.8+ux*t*0.35*side
		}
		mx, my := fx+ux*t*0.5, fy+uy*t*0.5 // muzzle
		if mz, ok := r.traceMuzzle(tr); ok {
			mx, my = r.worldToScreenF(mz.x, mz.y)
		}
		if age := traceAge(rt, tracerTicks); age < tracerTicks {
			k := 1 - age/tracerTicks
			// a short bright streak travelling along the line
			p := float32(age+1) / tracerTicks
			sx, sy := mx+(ex-mx)*(p-0.25), my+(ey-my)*(p-0.25)
			if p < 0.25 {
				sx, sy = mx, my
			}
			p = min(p, 1)
			vector.StrokeLine(screen, mx, my, ex, ey, 1, fade(tracerClr, k*0.35), true)
			vector.StrokeLine(screen, sx, sy, mx+(ex-mx)*p, my+(ey-my)*p, 3, fade(tracerClr, k), true)
		}
		if age := traceAge(rt, flashTicks); age < flashTicks {
			vector.FillCircle(screen, mx, my, t*0.1, fade(flashClr, 1-age/flashTicks), true)
		}
		if age := traceAge(rt, puffTicks); tr.Hit && age >= 3 && age < puffTicks {
			k := 1 - (age-3)/(puffTicks-3)
			vector.FillCircle(screen, tx, ty, t*(0.05+0.012*float32(age)), fade(bloodClr, k), true)
		}
	}
}

// traceMuzzle is the shooter's muzzle for a shot, in world tiles, when the
// shooter is a drawn pawn holding a rifle.
func (r *Renderer) traceMuzzle(tr game.TraceView) (vec, bool) {
	p, ok := pawnByStrike(r.drawPawns, tr)
	if !ok {
		return vec{}, false
	}
	return r.pawnHeldPoint(p, "muzzle")
}

// meleeTrailSteps are the moments a melee trail samples: the end of the
// wind-up (AttackCycle), then strike ages through the follow-through.
var meleeTrailSteps = []struct {
	cycle, age float64
	struck     bool
}{{0.95, 0, false}, {0, 0, true}, {0, 4, true}, {0, 6, true}, {0, 8, true}}

// meleeTrail is the path of a melee attacker's weapon tip (or fist)
// through its strike motion, in world tiles; nil when the attacker is
// not a drawn pawn or the motion does not move the tip.
func (r *Renderer) meleeTrail(tr game.TraceView) []vec {
	p, ok := pawnByStrike(r.drawPawns, tr)
	if !ok || p.Lying || p.CarriedBy != 0 || r.art == nil {
		return nil
	}
	wx, wy := r.interpWorld(p)
	m := r.pawnMotion(p.ID)
	m.Facing = r.poseFacing(p, m)
	ps, h := r.standingPose(p, m)
	rig := r.rigOf(p, m.Facing)
	if rig.mirror {
		ps = mirrored(ps)
	}
	attach := "tip"
	if h.key == "" {
		attach = "hold"
	}
	var pts []vec
	lo, hi := vec{math.Inf(1), math.Inf(1)}, vec{math.Inf(-1), math.Inf(-1)}
	for _, st := range meleeTrailSteps {
		_, h.angle, h.push = meleeMotion(p.Weapon, st.cycle, st.age, st.struck)
		pt, ok := r.heldPoint(rig, h, wx, wy, ps, attach)
		if !ok {
			return nil
		}
		pts = append(pts, pt)
		lo = vec{math.Min(lo.x, pt.x), math.Min(lo.y, pt.y)}
		hi = vec{math.Max(hi.x, pt.x), math.Max(hi.y, pt.y)}
	}
	if math.Hypot(hi.x-lo.x, hi.y-lo.y) < 0.08 {
		return nil
	}
	return pts
}

// drawMeleeTrail draws a fading swish along the weapon tip's strike path,
// or a short arc at the target when the path is unknown.
func (r *Renderer) drawMeleeTrail(screen *ebiten.Image, tr game.TraceView, k float64, tx, ty, t float32) {
	if pts := r.meleeTrail(tr); pts != nil {
		for i := 1; i < len(pts); i++ {
			ax, ay := r.worldToScreenF(pts[i-1].x, pts[i-1].y)
			bx, by := r.worldToScreenF(pts[i].x, pts[i].y)
			vector.StrokeLine(screen, ax, ay, bx, by, 3, fade(swingClr, k*float64(i)/float64(len(pts)-1)), true)
		}
		return
	}
	fx, fy := r.worldToScreenF(float64(tr.FromX)+0.5, float64(tr.FromY)+0.3)
	a0 := math.Atan2(float64(ty-fy), float64(tx-fx)) - 0.9
	for i := 0; i < 3; i++ {
		a1, a2 := a0+float64(i)*0.6, a0+float64(i+1)*0.6
		rad := float64(t) * 0.35
		vector.StrokeLine(screen,
			tx+float32(math.Cos(a1)*rad), ty+float32(math.Sin(a1)*rad),
			tx+float32(math.Cos(a2)*rad), ty+float32(math.Sin(a2)*rad),
			3, fade(swingClr, k), true)
	}
}

// nextVisualStep is nextVisual for movers with their own pace.
func nextVisualStep(prev pawnVisual, seen bool, x, y int, tick clock.Tick, present uint64, step int) pawnVisual {
	v := nextVisual(prev, seen, x, y, tick, present)
	if seen && prev.x == x && prev.y == y && tick-prev.moveTick < clock.Tick(max(step, 1)) {
		v.fromX, v.fromY = prev.fromX, prev.fromY
	}
	return v
}

// settleVisuals finishes every in-flight step. Captures freeze the
// simulation, so without this a mover is drawn mid-step.
func (r *Renderer) settleVisuals() {
	for id, v := range r.last {
		v.fromX, v.fromY = v.x, v.y
		r.last[id] = v
	}
	for _, k := range r.mo.tracks {
		k.settle()
	}
}

// animalFrame is the animal's body frame: a flinch while a hit is
// fresh, the bite or kick (strike frame 3 on the attack's trace, wind-up
// 1-2 by AttackCycle), the trot while moving, grazing.
func (r *Renderer) animalFrame(an game.AnimalView) string {
	m := r.animalMotion(an.ID)
	if age, ok := r.hitAge(an.ID, an.HitTick, an.X, an.Y); ok && age < staggerTicks {
		if age < 4 {
			return "_flinch1"
		}
		return "_flinch2"
	}
	st, age, struck := r.lastStrike(an.ID, an.X, an.Y, meleeTicks)
	moving := m.Blend >= minBlend
	if set := animalAttackSet(an, st, struck); set != "" {
		switch {
		case struck && age < 3:
			return set + "3"
		case struck && age < meleeTicks:
			return set + "4"
		case !moving && attackAction(an.Action):
			if an.AttackCycle < aimAt {
				return set + "1"
			}
			return set + "2"
		}
	}
	if moving {
		return trotSuffix(m)
	}
	if an.Action == game.ActionGraze {
		if math.Mod(r.animT()+float64(an.ID)*17, 90) < 45 {
			return "_graze1"
		}
		return "_graze2"
	}
	return ""
}

// attackAction reports an Action that is a natural-weapon attack.
func attackAction(a string) bool { return a == game.ActionBite || a == game.ActionKick }

// animalAttackSet is the attack pose set while the animal attacks or just
// struck: its natural weapon's set ("_claw", "_gore", ...). A view without
// an attack kind (an older sim) falls back to the action, then the weapon
// of its last strike.
func animalAttackSet(an game.AnimalView, st reactTrace, struck bool) string {
	if !struck && !attackAction(an.Action) {
		return ""
	}
	switch {
	case an.Attack != "":
		return "_" + an.Attack
	case an.Action == game.ActionKick:
		return "_kick"
	case an.Action == game.ActionBite:
		return "_bite"
	case st.Weapon == "hooves":
		return "_kick"
	}
	return "_bite"
}

// Overlay heights over an animal, in tiles above its tile origin. The
// former single-size values fit a 0.95-tile deer; bigger bodies lift
// them by their extra height (about 0.45 of the body length for a
// quadruped) so marks clear an elephant and hug a rat.
const (
	animalHeightK   = 0.45
	animalRefSize   = 0.95 // body length the base lifts were tuned for
	animalMarkBase  = 0.95
	animalBarBase   = 0.32
	animalMinMark   = 0.55
	animalMinBar    = 0.1
	animalMarkAbove = 0.4 // a mark floats at least this far over the height
)

// animalHeight is an animal's estimated height in tiles from its body
// length (an unknown size counts as the reference deer).
func animalHeight(size float64) float64 {
	if size <= 0 {
		size = animalRefSize
	}
	return animalHeightK * size
}

// animalMarkerLift is how far above its tile the hunt and tame marks hover.
func animalMarkerLift(size float64) float64 {
	extra := animalHeight(size) - animalHeight(animalRefSize)
	return max(animalMinMark, animalMarkBase+extra, animalHeight(size)+animalMarkAbove)
}

// animalBarLift is how far above its tile the health bar sits.
func animalBarLift(size float64) float64 {
	return max(animalMinBar, animalBarBase+animalHeight(size)-animalHeight(animalRefSize))
}

// animalPose: a trot bob (more when running), idle breathing, a lunge
// into each bite or kick, and a stagger away from the attacker when hit.
func (r *Renderer) animalPose(an game.AnimalView) pose {
	ps := pose{px: figurePivotX, py: figurePivotY}
	m := r.animalMotion(an.ID)
	if m.Blend >= minBlend {
		ps.dy = -trotBob(m, an.Mode != "wandering")
	} else {
		ps.sy = 1 + 0.015*math.Sin(2*math.Pi*float64(r.present+uint64(an.ID)*29)/120)
	}
	if st, age, struck := r.lastStrike(an.ID, an.X, an.Y, meleeTicks); struck && age < meleeTicks {
		k := math.Sin(math.Pi * age / meleeTicks)
		ux, uy := unit(float64(st.ToX-st.FromX), float64(st.ToY-st.FromY))
		ps.dx, ps.dy = ps.dx+ux*0.2*k, ps.dy+uy*0.2*k
	}
	if an.HitTick == 0 {
		dx, dy, fl := struckNudge(r.animState().reactions(r.present, nil), an.X, an.Y)
		ps.dx, ps.dy, ps.flash = ps.dx+dx, ps.dy+dy, fl
		return ps
	}
	if age, ok := r.hitAge(an.ID, an.HitTick, an.X, an.Y); ok && age < hitTicks {
		ux, uy := awayFrom(an.X, an.Y, an.HitFromX, an.HitFromY, m.Facing)
		k := staggerK(age)
		ps.dx, ps.dy = ps.dx+ux*staggerDist*k, ps.dy+uy*staggerDist*k
		ps.flash = float32(1 - age/hitTicks)
	}
	return ps
}
