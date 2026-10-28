package ebiten

import (
	"math"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// moveState is how a creature moves on screen right now (ART §27). It is
// presentation-only: derived from read models and the presentation clocks,
// never written back to the simulation.
type moveState struct {
	X, Y   float64 // world tiles, sub-tile, sampled at animT
	VX, VY float64 // tiles per tick, smoothed
	Speed  float64 // |V|; 0 when idle
	Stride float64 // distance walked along the drawn path, in tiles
	Blend  float64 // 0..1 walk weight (eases idle <-> walk)
	Facing facing  // hysteresis-smoothed
	Moving bool    // inside a step (false while blocked or idle)
	// Left is the stride left until the current walk ends at rest, in
	// tiles: +Inf while the path goes on (PLAY-022: a stopping animal's
	// walk cycle steers onto a neutral frame by then).
	Left float64
}

// Motion tuning (ART §27.1).
const (
	blendTiles  = 0.3   // walk blend eases in/out over this much stride
	turnFrames  = 4     // a north<->south reversal shows a side facing this long
	axisHyst    = 1.2   // 20% axis hysteresis on the smoothed velocity
	chordMin    = 0.15  // tiles of look-around travel that count as moving
	diagFactor  = 1.414 // diagonal step cost factor (C2)
	maxSegs     = 6     // steps of history kept per creature
	alphaMax    = 0.999 // alpha stays inside [0, 1)
	corrMin     = 2.0   // shortest correction blend, ticks
	corrFrames  = 6     // longest correction blend, in updates
	movingSpeed = 1e-4  // tiles/tick below which a creature is standing
	velH        = 1e-3  // finite-difference step, ticks
	// drawnReach bounds how far (tiles) a drawn creature can be from its
	// read-model cell (the trailing step plus a correction): cell-based
	// culls pad by it before paying for the motion sample (an animal's add
	// its body's reach, artCatalog.animalExtent).
	drawnReach = 3.0
	// motionViewPad: animals within this many tiles of the last frame's
	// view keep a motion track (syncAnimals); every pass that samples
	// their motion culls tighter (objects, shadows, plant contacts).
	motionViewPad = contactViewPad + 2
)

// sideBias weighs an animal's east-west travel against north-south when it
// picks a facing (PLAY-022): a long body on a diagonal shows its side, not
// a front walking crabwise.
const sideBias = 1.4

// motionState is the Renderer's presentation motion bookkeeping: the
// clocks (C7) and one track per creature.
type motionState struct {
	tracks  map[entity.EntityID]*moTrack
	alpha   float64 // fraction of the current update interval elapsed
	stepped int     // ticks the latest advance stepped (0 paused)
	now     float64 // sim tick at the latest sync
	ref     float64 // animT the previous draws ended at
	synced  bool
	gen     uint64
}

// setAlpha records the Draw's position inside the update interval.
func (r *Renderer) setAlpha(a float64) {
	if a < 0 || math.IsNaN(a) {
		a = 0
	}
	r.mo.alpha = math.Min(a, alphaMax)
}

// animT is sim-scaled presentation time in ticks: it freezes when paused and
// runs faster at higher speeds. Movement, walk cycles, work swings and
// combat reactions use it; ambient effects use present.
//
// It trails the simulation by one update (Tick - (1-alpha)*stepped), so
// every step it draws has already happened: positions interpolate between
// known states instead of extrapolating, and the clock stays continuous
// and monotonic across speed changes and pauses.
func (r *Renderer) animT() float64 {
	if r.game == nil {
		return 0
	}
	return float64(r.game.Tick()) - (1-r.mo.alpha)*float64(r.mo.stepped)
}

// pawnMotion is the on-screen motion of a colonist or raider.
func (r *Renderer) pawnMotion(id entity.EntityID) moveState {
	m, _ := r.motionOf(id)
	return m
}

// animalMotion is the on-screen motion of an animal.
func (r *Renderer) animalMotion(id entity.EntityID) moveState {
	m, _ := r.motionOf(id)
	return m
}

func (r *Renderer) motionOf(id entity.EntityID) (moveState, bool) {
	k := r.mo.tracks[id]
	if k == nil {
		return moveState{}, false
	}
	t := r.animT()
	if k.memo.ok && k.memo.t == t && k.memo.gen == r.mo.gen {
		return k.memo.m, true // a draw samples a creature many times
	}
	m := k.state(t)
	k.memo = moMemo{m: m, t: t, gen: r.mo.gen, ok: true}
	return m, true
}

// moMemo is a track's last sampled state: valid for its animT within one
// sync generation (observe and turn run only in a sync; settle clears it).
type moMemo struct {
	m   moveState
	t   float64
	gen uint64
	ok  bool
}

// --- sync -------------------------------------------------------------

// moView is the part of a pawn or animal read model motion needs.
type moView struct {
	x, y      int
	stepTick  uint64
	stepTicks int
	nx, ny    int
	hasNext   bool
	blocked   entity.EntityID
	legacy    bool // the sim fills no step timing: estimate it
	defDur    int  // step duration when the view has none
	animal    bool // an animal: diagonals face sideways (sideBias)
}

func pawnMoView(p game.PawnView) moView {
	return moView{x: p.X, y: p.Y, stepTick: p.StepTick, stepTicks: p.StepTicks,
		nx: p.NextX, ny: p.NextY, hasNext: p.HasNext, blocked: p.BlockedBy,
		legacy: p.StepTick == 0 && p.StepTicks == 0, defDur: game.TicksPerCell}
}

func animalMoView(an game.AnimalView) moView {
	return moView{x: an.X, y: an.Y, stepTick: an.StepTick, stepTicks: an.StepTicks,
		nx: an.NextX, ny: an.NextY, hasNext: an.HasNext, blocked: an.BlockedBy,
		legacy: an.StepTick == 0, defDur: max(an.StepTicks, 1), animal: true}
}

// beginSync starts a sync at sim tick now.
func (mo *motionState) beginSync(now clock.Tick) {
	if mo.tracks == nil {
		mo.tracks = map[entity.EntityID]*moTrack{}
	}
	t := float64(now)
	mo.ref = t
	if mo.synced {
		mo.ref = mo.now
	}
	mo.now, mo.synced = t, true
	mo.gen++
}

// endSync drops creatures that are gone.
func (mo *motionState) endSync() {
	for id, k := range mo.tracks {
		if k.gen != mo.gen {
			delete(mo.tracks, id)
		}
	}
}

// corrMax is the longest correction blend at the current speed.
func (mo *motionState) corrMax() float64 {
	return float64(corrFrames * max(mo.stepped, 1))
}

// observe folds one read model into the creature's track. Anything that
// changes what was already drawn is blended out from the previous draw
// (position, velocity, stride and walk blend stay continuous).
func (mo *motionState) observe(id entity.EntityID, v moView, face facing) *moTrack {
	k := mo.tracks[id]
	if k == nil {
		k = &moTrack{x: v.x, y: v.y, fac: face, turnTo: face, side: v.animal}
		mo.tracks[id] = k
		k.plan(v, mo.now)
		k.gen = mo.gen
		return k
	}
	k.gen = mo.gen
	ref := mo.ref
	oldDrawn := k.drawn(ref)
	oldDrawnV := oldDrawn.sub(k.drawn(ref - velH)).scale(1 / velH)
	oldModel := k.model(ref)
	oldModelV := k.model(ref + velH).sub(oldModel).scale(1 / velH)

	if v.x != k.x || v.y != k.y {
		if abs(v.x-k.x) > 1 || abs(v.y-k.y) > 1 {
			// teleported or several steps unseen: snap
			k.rest = k.model(mo.now).stride
			k.segs, k.corr = nil, moCorr{}
			k.x, k.y = v.x, v.y
			k.plan(v, mo.now)
			return k
		}
		k.step(v, mo)
	}
	k.plan(v, mo.now)

	newModel := k.model(ref)
	newModelV := k.model(ref + velH).sub(newModel).scale(1 / velH)
	if oldModel.differs(newModel) || oldModelV.differs(newModelV) {
		dp, dv := oldDrawn.sub(newModel), oldDrawnV.sub(newModelV)
		tc := mo.corrMax()
		if s := math.Hypot(dv.x, dv.y); s > 1e-9 {
			tc = 2 * math.Hypot(dp.x, dp.y) / s
		}
		tc = math.Max(corrMin, math.Min(tc, mo.corrMax()))
		k.corr = moCorr{dp: dp, dv: dv, t0: ref, tc: tc, on: true}
	}
	return k
}

// step appends the step into the view's cell.
func (k *moTrack) step(v moView, mo *motionState) {
	var last *moSeg
	if n := len(k.segs); n > 0 {
		last = &k.segs[n-1]
	}
	start := mo.now
	switch {
	case !v.legacy && v.stepTick > 0 && float64(v.stepTick) <= mo.now:
		start = float64(v.stepTick)
	case last != nil && !last.final && last.end() >= mo.ref && last.end() <= mo.now:
		start = last.end() // a walker steps the moment its last step ends
	}
	dur := float64(v.defDur)
	if !v.legacy && v.stepTicks > 0 {
		dur = float64(v.stepTicks)
	}
	s := moSeg{fx: k.x, fy: k.y, tx: v.x, ty: v.y, start: start, dur: math.Max(dur, 1), base: k.rest}
	if last != nil {
		if s.start <= last.start {
			s.start = last.start + 1
		}
		if s.start < last.end() { // stepped early: the last step was shorter
			last.dur = s.start - last.start
		}
		if !last.final && s.start == last.end() {
			s.cont, s.px, s.py, s.pdur = true, last.fx, last.fy, last.dur
		} else {
			last.final = true
		}
	}
	k.segs = append(k.segs, s)
	n := len(k.segs)
	cur := &k.segs[n-1]
	if n > 1 {
		prev := &k.segs[n-2]
		cur.base = prev.base + k.segLen(n-2)
		if cur.cont {
			cur.runStart = prev.runStart
		} else {
			cur.runStart = cur.base
		}
	} else {
		cur.runStart = cur.base
	}
	if n > maxSegs {
		k.segs = append(k.segs[:0], k.segs[n-maxSegs:]...)
	}
	k.x, k.y = v.x, v.y
}

// plan takes the view's next cell and decides whether the latest step
// ends at rest.
func (k *moTrack) plan(v moView, now float64) {
	k.blocked = v.blocked
	k.nx, k.ny, k.hasNext = v.nx, v.ny, v.hasNext
	n := len(k.segs)
	if n == 0 {
		return
	}
	s := &k.segs[n-1]
	if v.legacy {
		// no plan in the view: assume it walks on until seen stopping
		k.nx, k.ny, k.hasNext = 2*s.tx-s.fx, 2*s.ty-s.fy, true
		k.blocked = 0
	}
	s.final = now > s.end() || !k.hasNext || k.blocked != 0
}

// --- facing -------------------------------------------------------------

// wantFacing is the movement facing at the latest sync: towards whatever
// blocks the way, else along the travel over a window two steps long (a
// zig-zag's full period, so it averages out), with axis hysteresis. The
// window ends no later than the known path, centred when it can be.
// moving is false when the creature is not travelling; the caller then
// decides.
func (k *moTrack) wantFacing(ref float64) (facing, bool) {
	if k.blocked != 0 && k.hasNext && (k.nx != k.x || k.ny != k.y) {
		return facingFor(k.nx-k.x, k.ny-k.y, k.turnTo), true
	}
	n := len(k.segs)
	if n == 0 {
		return k.turnTo, false
	}
	last := &k.segs[n-1]
	if last.final && ref >= last.end() {
		return k.turnTo, false // standing: keep the last walking facing
	}
	d := last.dur
	f := math.Max(0, math.Min(d, last.end()-ref))
	a, b := k.model(ref+f-2*d), k.model(ref+f)
	cx, cy := b.x-a.x, b.y-a.y
	if math.Hypot(cx, cy) < chordMin {
		return k.turnTo, false
	}
	k.chordX = cx
	if k.side {
		cx *= sideBias
	}
	return hystFacing(cx, cy, k.turnTo), true
}

// hystFacing keeps the current axis until the other one leads by 20%.
func hystFacing(vx, vy float64, prev facing) facing {
	ax, ay := math.Abs(vx), math.Abs(vy)
	horiz := prev == faceEast || prev == faceWest
	if horiz {
		horiz = ay <= axisHyst*ax
	} else {
		horiz = ax > axisHyst*ay
	}
	switch {
	case horiz && vx >= 0:
		return faceEast
	case horiz:
		return faceWest
	case vy >= 0:
		return faceSouth
	default:
		return faceNorth
	}
}

// turn moves the shown facing towards want. A north<->south reversal shows
// a side facing for turnFrames updates first; the countdown only runs
// while the game does (paused freezes it).
func (k *moTrack) turn(want facing, id entity.EntityID, running bool) facing {
	if k.turnLeft > 0 {
		if want != k.turnTo {
			k.fac, k.turnTo, k.turnLeft = want, want, 0
			return k.fac
		}
		if running {
			k.turnLeft--
		}
		if k.turnLeft == 0 {
			k.fac = k.turnTo
		}
		return k.fac
	}
	if (k.fac == faceNorth && want == faceSouth) || (k.fac == faceSouth && want == faceNorth) {
		side := faceEast
		if k.chordX < 0 || (k.chordX == 0 && id%2 == 1) {
			side = faceWest
		}
		k.fac, k.turnTo, k.turnLeft = side, want, turnFrames
		return k.fac
	}
	k.fac, k.turnTo = want, want
	return k.fac
}

// settle finishes in-flight motion: the creature rests on its cell.
func (k *moTrack) settle() {
	if n := len(k.segs); n > 0 {
		k.rest = k.segs[n-1].base + k.segLen(n-1)
	}
	k.segs, k.corr = nil, moCorr{}
	k.turnLeft, k.fac = 0, k.turnTo
	k.body = animalBody{} // the next draw starts the animal's pose afresh, at rest
	k.memo = moMemo{}
}

// --- the drawn path -----------------------------------------------------

// moSeg is one simulation step from (fx, fy) into (tx, ty), drawn during
// [start, start+dur) sim ticks.
type moSeg struct {
	fx, fy, tx, ty int
	start, dur     float64
	base           float64 // stride at the start of the step
	runStart       float64 // stride where its walk began (from rest)
	cont           bool    // continues a moving step without stopping
	px, py         int     // the cell before (fx, fy), when cont
	pdur           float64 // that step's duration, when cont
	final          bool    // ends at rest
}

func (s *moSeg) end() float64 { return s.start + s.dur }

func (s *moSeg) diag() bool { return s.fx != s.tx && s.fy != s.ty }

// moTrack is one creature's presentation motion.
type moTrack struct {
	segs        []moSeg
	x, y        int // current cell
	nx, ny      int // planned next cell, when hasNext
	hasNext     bool
	blocked     entity.EntityID
	rest        float64 // stride while there are no steps
	corr        moCorr
	fac, turnTo facing
	turnLeft    int
	chordX      float64
	side        bool // an animal: favour side facings (sideBias)
	gen         uint64
	lens        [maxSegs + 1]geomLen // arc-length memo per step index (lengths)
	// body is an animal's walk phase and idle pose player (animal_figure.go).
	body animalBody
	memo moMemo // motionOf's last sample
}

// moSample is the corrected part of the motion.
type moSample struct{ x, y, stride, blend float64 }

func (a moSample) sub(b moSample) moSample {
	return moSample{a.x - b.x, a.y - b.y, a.stride - b.stride, a.blend - b.blend}
}

func (a moSample) add(b moSample) moSample {
	return moSample{a.x + b.x, a.y + b.y, a.stride + b.stride, a.blend + b.blend}
}

func (a moSample) scale(k float64) moSample {
	return moSample{a.x * k, a.y * k, a.stride * k, a.blend * k}
}

func (a moSample) differs(b moSample) bool {
	const eps = 1e-7
	return math.Abs(a.x-b.x) > eps || math.Abs(a.y-b.y) > eps ||
		math.Abs(a.stride-b.stride) > eps || math.Abs(a.blend-b.blend) > eps
}

// moCorr blends a past discontinuity out with a cubic Hermite: at t0 it
// adds dp and dv (value and rate), after t0+tc nothing.
type moCorr struct {
	dp, dv moSample
	t0, tc float64
	on     bool
}

func (c moCorr) at(t float64) moSample {
	if !c.on || t >= c.t0+c.tc {
		return moSample{}
	}
	u := math.Max((t-c.t0)/c.tc, 0)
	h00 := 2*u*u*u - 3*u*u + 1
	h10 := u*u*u - 2*u*u + u
	return c.dp.scale(h00).add(c.dv.scale(c.tc * h10))
}

// state is the drawn motion at animT t.
func (k *moTrack) state(t float64) moveState {
	d := k.drawn(t)
	a, b := k.drawn(t-velH), k.drawn(t+velH)
	m := moveState{X: d.x, Y: d.y, Stride: d.stride, Blend: math.Max(0, math.Min(d.blend, 1)), Facing: k.fac}
	m.VX, m.VY = (b.x-a.x)/(2*velH), (b.y-a.y)/(2*velH)
	m.Speed = math.Hypot(m.VX, m.VY)
	if m.Speed < 1e-9 {
		m.VX, m.VY, m.Speed = 0, 0, 0
	}
	m.Moving = m.Speed > movingSpeed
	if !m.Moving && m.Blend < 1e-9 {
		m.Blend = 0
	}
	m.Left = math.Max(0, k.walkEnd(t)-d.stride)
	return m
}

// walkEnd is the stride at which the walk under way at t ends at rest
// (the end of the first final step from there), +Inf while the path goes
// on; standing, the stride it rests at.
func (k *moTrack) walkEnd(t float64) float64 {
	n := len(k.segs)
	if n == 0 {
		return k.rest
	}
	i := n - 1
	for i > 0 && k.segs[i].start > t {
		i--
	}
	for j := i; j < n; j++ {
		if s := &k.segs[j]; s.final || (j+1 < n && !k.segs[j+1].cont) {
			return s.base + k.segLen(j)
		}
	}
	return math.Inf(1)
}

func (k *moTrack) drawn(t float64) moSample { return k.model(t).add(k.corr.at(t)) }

// model is the path the read models describe, before corrections.
func (k *moTrack) model(t float64) moSample {
	if len(k.segs) == 0 {
		return moSample{x: float64(k.x), y: float64(k.y), stride: k.rest}
	}
	first := &k.segs[0]
	if t <= first.start {
		return moSample{x: float64(first.fx), y: float64(first.fy), stride: first.base}
	}
	i := len(k.segs) - 1
	for i > 0 && k.segs[i].start > t {
		i--
	}
	s := &k.segs[i]
	g := k.geom(i)
	l1, ln := k.lengths(i, g)
	if t < s.end() {
		smp := g.evalL(s, t, l1)
		smp.blend = k.blendAt(i, smp.stride)
		return smp
	}
	done := moSample{x: float64(s.tx), y: float64(s.ty), stride: s.base + ln}
	if i+1 < len(k.segs) {
		return done // waiting between steps
	}
	nx, ny, nd, ok := k.next(i)
	if !ok {
		return done
	}
	// The walk goes on into the planned step. Normally only look-ahead
	// (facing) reaches here, but the real sim's step is first seen one tick
	// after it starts, so the previous draw ended exactly on s.end(): the
	// sample there must match the step that follows, walk blend included.
	vs := moSeg{fx: s.tx, fy: s.ty, tx: nx, ty: ny, start: s.end(), dur: nd, base: done.stride,
		cont: true, px: s.fx, py: s.fy, pdur: s.dur, final: true}
	if t >= vs.end() {
		return moSample{x: float64(nx), y: float64(ny), stride: vs.base}
	}
	smp := segGeom(&vs, 0, 0, 0, false).eval(&vs, t)
	smp.blend = smoothstep01((smp.stride - s.runStart) / blendTiles)
	return smp
}

// next is the step after segs[i] when the walk continues into it: the
// actual one, or the planned one with its predicted duration.
func (k *moTrack) next(i int) (nx, ny int, nd float64, ok bool) {
	if i+1 < len(k.segs) {
		n := &k.segs[i+1]
		return n.tx, n.ty, n.dur, n.cont
	}
	s := &k.segs[i]
	if s.final || !k.hasNext {
		return 0, 0, 0, false
	}
	return k.nx, k.ny, predictDur(s, k.nx, k.ny), true
}

// predictDur is the next step's duration at the current pace (C2: a
// diagonal costs round(straight*1.414)).
func predictDur(s *moSeg, nx, ny int) float64 {
	base := s.dur
	if s.diag() {
		base = math.Round(s.dur / diagFactor)
	}
	if nx != s.tx && ny != s.ty {
		return math.Max(math.Round(base*diagFactor), 1)
	}
	return math.Max(base, 1)
}

func (k *moTrack) geom(i int) moGeom {
	nx, ny, nd, ok := k.next(i)
	return segGeom(&k.segs[i], nx, ny, nd, ok)
}

func (k *moTrack) segLen(i int) float64 {
	_, l := k.lengths(i, k.geom(i))
	return l
}

// geomLen memoises one step's arc lengths (h1 alone and the whole path):
// a creature's few steps are sampled many times per update and frame,
// and the Gauss-Legendre sums dominate. Keyed by the geometry itself, so
// any change to the step or what follows it misses.
type geomLen struct {
	g      moGeom
	l1, l  float64
	cached bool
}

// lengths is g's h1 arc length and total length (g = k.geom(i)), from
// the memo when its geometry matches; the same values length() gives.
func (k *moTrack) lengths(i int, g moGeom) (l1, l float64) {
	if i < len(k.lens) {
		c := &k.lens[i]
		if c.cached && c.g == g {
			return c.l1, c.l
		}
		l1 = g.h1.arc(1)
		l = l1 + g.h2.arc(1)
		*c = geomLen{g: g, l1: l1, l: l, cached: true}
		return l1, l
	}
	l1 = g.h1.arc(1)
	return l1, l1 + g.h2.arc(1)
}

// blendAt is the walk weight at stride st inside segs[i]'s walk: it eases
// in over the first blendTiles and out over the last.
func (k *moTrack) blendAt(i int, st float64) float64 {
	end := math.Inf(1)
	for j := i; j < len(k.segs); j++ {
		s := &k.segs[j]
		if s.final || (j+1 < len(k.segs) && !k.segs[j+1].cont) {
			end = s.base + k.segLen(j)
			break
		}
	}
	d := math.Min(st-k.segs[i].runStart, end-st) / blendTiles
	return smoothstep01(d)
}

func smoothstep01(x float64) float64 {
	x = math.Max(0, math.Min(x, 1))
	return x * x * (3 - 2*x)
}

// moGeom is a step's drawn path: two quadratic pieces meeting at the
// midpoint of the step, plus the end slopes of its time warp. Corners are
// the quadratic Bézier between the midpoints of the incoming and outgoing
// steps with the tile centre as control point; each step draws the half
// on its side of the midpoint.
type moGeom struct {
	h1, h2 quad
	m0, m1 float64
}

func segGeom(s *moSeg, nx, ny int, nd float64, hasNext bool) moGeom {
	p, c := vec{float64(s.fx), float64(s.fy)}, vec{float64(s.tx), float64(s.ty)}
	m := p.mid(c)
	var g moGeom
	if s.cont {
		m0 := vec{float64(s.px), float64(s.py)}.mid(p)
		g.h1 = quad{m0.add(p.scale(2)).add(m).scale(0.25), p.mid(m), m}
		g.m0 = 2 * s.dur / (s.pdur + s.dur)
	} else {
		g.h1 = quad{p, p.mid(m), m} // from rest: straight, eased in
	}
	if hasNext {
		m2 := c.mid(vec{float64(nx), float64(ny)})
		g.h2 = quad{m, m.mid(c), m.add(c.scale(2)).add(m2).scale(0.25)}
		g.m1 = 2 * s.dur / (s.dur + nd)
	} else {
		g.h2 = quad{m, m.mid(c), c} // to rest: straight, eased out
	}
	return g
}

// warp maps step progress u to path progress with a cubic Hermite whose
// end slopes match the neighbouring steps' speeds (0 at rest, so a lone
// step is exactly smoothstep).
func warp(u, m0, m1 float64) float64 {
	u = math.Max(0, math.Min(u, 1))
	u2, u3 := u*u, u*u*u
	w := m0*(u3-2*u2+u) + (3*u2 - 2*u3) + m1*(u3-u2)
	return math.Max(0, math.Min(w, 1))
}

func (g moGeom) eval(s *moSeg, t float64) moSample {
	w := warp((t-s.start)/s.dur, g.m0, g.m1)
	if w < 0.5 {
		return g.evalL(s, t, 0)
	}
	return g.evalL(s, t, g.h1.arc(1))
}

// evalL is eval with h1's arc length (g.h1.arc(1)) known.
func (g moGeom) evalL(s *moSeg, t, l1 float64) moSample {
	w := warp((t-s.start)/s.dur, g.m0, g.m1)
	var p vec
	var l float64
	if w < 0.5 {
		p, l = g.h1.at(2*w), g.h1.arc(2*w)
	} else {
		p, l = g.h2.at(2*w-1), l1+g.h2.arc(2*w-1)
	}
	return moSample{x: p.x, y: p.y, stride: s.base + l}
}

type vec struct{ x, y float64 }

func (a vec) add(b vec) vec             { return vec{a.x + b.x, a.y + b.y} }
func (a vec) scale(k float64) vec       { return vec{a.x * k, a.y * k} }
func (a vec) mid(b vec) vec             { return vec{(a.x + b.x) / 2, (a.y + b.y) / 2} }
func (a vec) lerp(b vec, t float64) vec { return vec{a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t} }

// quad is a quadratic Bézier.
type quad struct{ a, b, c vec }

func (q quad) at(s float64) vec { return q.a.lerp(q.b, s).lerp(q.b.lerp(q.c, s), s) }

func (q quad) speed(s float64) float64 {
	d := q.b.add(q.a.scale(-1)).scale(2 * (1 - s)).add(q.c.add(q.b.scale(-1)).scale(2 * s))
	return math.Hypot(d.x, d.y)
}

// Gauss-Legendre nodes and weights on [-1, 1].
var glX = [5]float64{0, -0.5384693101056831, 0.5384693101056831, -0.9061798459386640, 0.9061798459386640}
var glW = [5]float64{0.5688888888888889, 0.4786286704993665, 0.4786286704993665, 0.2369268850561891, 0.2369268850561891}

// arc is the length of q from 0 to s.
func (q quad) arc(s float64) float64 {
	if s <= 0 {
		return 0
	}
	sum := 0.0
	for i, x := range glX {
		sum += glW[i] * q.speed(s*(x+1)/2)
	}
	return sum * s / 2
}
