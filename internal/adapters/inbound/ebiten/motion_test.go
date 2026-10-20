package ebiten

import (
	"math"
	"testing"
	"time"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// --- scripted simulation ----------------------------------------------------

type cell struct{ x, y int }

// scriptMover walks a fixed path with the sim's step timing (C1/C2): it
// enters path[k] at enter[k] and that step lasts dur[k] ticks.
type scriptMover struct {
	id      entity.EntityID
	path    []cell
	start   uint64          // tick path[1] is entered
	pace    func(k int) int // straight step duration for step k (nil: 15)
	blockAt int             // stop for good on path[blockAt] (0: never)
	enter   []uint64        // computed
	dur     []int           // computed
	animal  bool            // reported as an AnimalView
	blocker entity.EntityID // BlockedBy while blocked
}

func (m *scriptMover) init() {
	m.enter = make([]uint64, len(m.path))
	m.dur = make([]int, len(m.path))
	at := m.start
	for k := 1; k < len(m.path); k++ {
		base := 15
		if m.pace != nil {
			base = m.pace(k)
		}
		d := base
		if m.path[k].x != m.path[k-1].x && m.path[k].y != m.path[k-1].y {
			d = int(math.Round(float64(base) * 1.414))
		}
		m.enter[k], m.dur[k] = at, d
		at += uint64(d)
	}
	if m.blocker == 0 {
		m.blocker = 99
	}
}

// view is what the simulation reports at tick now.
func (m *scriptMover) view(now uint64) (x, y int, stepTick uint64, stepTicks int, nx, ny int, hasNext bool, blocked entity.EntityID) {
	k := 0
	last := len(m.path) - 1
	if m.blockAt > 0 {
		last = m.blockAt
	}
	for i := 1; i <= last; i++ {
		if m.enter[i] <= now {
			k = i
		}
	}
	x, y = m.path[k].x, m.path[k].y
	if k > 0 {
		stepTick, stepTicks = m.enter[k], m.dur[k]
	}
	if k+1 < len(m.path) {
		nx, ny, hasNext = m.path[k+1].x, m.path[k+1].y, true
	}
	if m.blockAt > 0 && k == m.blockAt && k > 0 && now >= m.enter[k]+uint64(m.dur[k]) {
		blocked = m.blocker
	}
	return
}

type motionSim struct {
	stubSim
	movers []*scriptMover
}

func newMotionSim(movers ...*scriptMover) *motionSim {
	for _, m := range movers {
		m.init()
	}
	return &motionSim{stubSim: stubSim{w: 64, h: 64}, movers: movers}
}

func (s *motionSim) Step(n int) { s.steps += n; s.tick += clock.Tick(n) }

func (s *motionSim) Snapshot() []game.PawnView {
	var out []game.PawnView
	for _, m := range s.movers {
		if m.animal {
			continue
		}
		x, y, st, sts, nx, ny, hn, b := m.view(uint64(s.tick))
		out = append(out, game.PawnView{ID: m.id, X: x, Y: y, StepTick: st, StepTicks: sts,
			NextX: nx, NextY: ny, HasNext: hn, BlockedBy: b})
	}
	return out
}

func (s *motionSim) World() game.WorldView {
	var w game.WorldView
	for _, m := range s.movers {
		if !m.animal {
			continue
		}
		x, y, st, sts, nx, ny, hn, b := m.view(uint64(s.tick))
		w.Animals = append(w.Animals, game.AnimalView{ID: m.id, Species: "dustbuck", X: x, Y: y, Mode: "wandering",
			Health: 100, StepTick: st, StepTicks: sts, NextX: nx, NextY: ny, HasNext: hn, BlockedBy: b})
	}
	return w
}

func line(x, y, dx, dy, n int) []cell {
	out := []cell{{x, y}}
	for i := 0; i < n; i++ {
		x, y = x+dx, y+dy
		out = append(out, cell{x, y})
	}
	return out
}

func speedFor(tpf int) speed {
	switch tpf {
	case 3:
		return speedFast
	case 6:
		return speedVeryFast
	case 0:
		return speedPaused
	}
	return speedNormal
}

type drawSample struct {
	t float64
	m moveState
}

// drive runs updates at tpf ticks each and samples motion of id once per
// alpha after every update, like Draws between Updates.
func drive(r *Renderer, id entity.EntityID, tpf, updates int, alphas []float64) []drawSample {
	var out []drawSample
	r.speed = speedFor(tpf)
	for i := 0; i < updates; i++ {
		r.advance()
		for _, a := range alphas {
			r.setAlpha(a)
			m := r.pawnMotion(id)
			if isAnimal(r, id) {
				m = r.animalMotion(id)
			}
			out = append(out, drawSample{t: r.animT(), m: m})
		}
	}
	return out
}

func isAnimal(r *Renderer, id entity.EntityID) bool {
	for _, an := range r.game.World().Animals {
		if an.ID == id {
			return true
		}
	}
	return false
}

func dist(a, b moveState) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

func alphaSteps(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i) / float64(n)
	}
	return out
}

// --- tests ------------------------------------------------------------------

func TestConstantSpeedNoStutter(t *testing.T) {
	t.Parallel()
	paths := map[string][]cell{"straight": line(5, 5, 1, 0, 14), "diagonal": line(5, 5, 1, 1, 14)}
	for name, path := range paths {
		for _, tpf := range []int{1, 3, 6} {
			mv := &scriptMover{id: 1, path: path, start: 10}
			sim := newMotionSim(mv)
			r := New(sim)
			samples := drive(r, 1, tpf, 400/tpf, []float64{0, 0.5})
			lo, hi := float64(mv.enter[2]), float64(mv.enter[len(path)-1])
			var ds []float64
			maxAll := 0.0
			for i := 1; i < len(samples); i++ {
				a, b := samples[i-1], samples[i]
				d := dist(a.m, b.m)
				// progress along the path must never go backwards
				dir := float64(path[1].x - path[0].x)
				if (b.m.X-a.m.X)*dir < -1e-9 {
					t.Fatalf("%s %dx: moved backwards at t=%.2f", name, tpf, b.t)
				}
				maxAll = math.Max(maxAll, d)
				if a.t >= lo && b.t <= hi {
					ds = append(ds, d)
				}
			}
			if len(ds) < 10 {
				t.Fatalf("%s %dx: only %d cruise samples", name, tpf, len(ds))
			}
			mean, cv := meanCV(ds)
			if cv > 0.05 {
				t.Fatalf("%s %dx: per-draw displacement CV %.4f > 5%%", name, tpf, cv)
			}
			for _, d := range ds {
				if d > 1.5*mean {
					t.Fatalf("%s %dx: jump %.4f > 1.5x mean %.4f", name, tpf, d, mean)
				}
			}
			if maxAll > 1.5*mean {
				t.Fatalf("%s %dx: jump %.4f anywhere > 1.5x cruise mean %.4f", name, tpf, maxAll, mean)
			}
			t.Logf("%s %dx: mean %.5f tiles/draw, CV %.5f", name, tpf, mean, cv)
		}
	}
}

func meanCV(xs []float64) (float64, float64) {
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	v := 0.0
	for _, x := range xs {
		v += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(v/float64(len(xs))) / mean
}

// cruiseSpeed is tiles per tick between the end of the first step and
// the start of the last.
func cruiseSpeed(t *testing.T, path []cell, tpf int) float64 {
	t.Helper()
	mv := &scriptMover{id: 1, path: path, start: 10}
	r := New(newMotionSim(mv))
	s := drive(r, 1, tpf, 400/tpf, []float64{0, 0.5})
	lo, hi := float64(mv.enter[2]), float64(mv.enter[len(path)-1])
	var a, b *drawSample
	for i := range s {
		if s[i].t >= lo && a == nil {
			a = &s[i]
		}
		if s[i].t <= hi {
			b = &s[i]
		}
	}
	return dist(a.m, b.m) / (b.t - a.t)
}

func TestDiagonalSameVisualSpeed(t *testing.T) {
	t.Parallel()
	for _, tpf := range []int{1, 3} {
		st := cruiseSpeed(t, line(5, 5, 1, 0, 12), tpf)
		dg := cruiseSpeed(t, line(5, 5, 1, 1, 12), tpf)
		ratio := dg / st
		t.Logf("%dx: straight %.5f, diagonal %.5f tiles/tick, ratio %.4f", tpf, st, dg, ratio)
		if math.Abs(ratio-1) > 0.03 {
			t.Fatalf("%dx: diagonal/straight speed ratio %.4f outside 3%%", tpf, ratio)
		}
	}
}

func TestCornerCurveContinuous(t *testing.T) {
	t.Parallel()
	path := []cell{{5, 5}, {6, 5}, {7, 5}, {7, 6}, {7, 7}, {8, 8}, {9, 8}, {9, 7}, {9, 6}, {8, 6}}
	for _, tpf := range []int{1, 3, 6} {
		mv := &scriptMover{id: 1, path: path, start: 10}
		r := New(newMotionSim(mv))
		s := drive(r, 1, tpf, 250/tpf, alphaSteps(10*tpf))
		for i := 1; i < len(s); i++ {
			if d := dist(s[i-1].m, s[i].m); d >= 0.02 {
				t.Fatalf("%dx: jump %.4f at t=%.2f", tpf, d, s[i].t)
			}
		}
		// the corner at (7,5) is cut: on arrival the pawn is not on the centre
		at := float64(mv.enter[2] + uint64(mv.dur[2]))
		for _, x := range s {
			if math.Abs(x.t-at) < 1e-6 {
				off := math.Hypot(x.m.X-7, x.m.Y-5)
				if off < 0.05 || off > 0.2 {
					t.Fatalf("%dx: corner offset %.3f, want a curve inside the turn", tpf, off)
				}
			}
		}
	}
}

func TestEaseStartStop(t *testing.T) {
	t.Parallel()
	path := line(5, 5, 1, 0, 5)
	mv := &scriptMover{id: 1, path: path, start: 10}
	r := New(newMotionSim(mv))
	s := drive(r, 1, 1, 120, []float64{0, 0.5})
	cruise := 1.0 / 15
	start, end := float64(mv.enter[1]), float64(mv.enter[5]+uint64(mv.dur[5]))
	var first, last []drawSample
	for _, x := range s {
		if x.t < start {
			if x.m.Moving || x.m.Blend != 0 || x.m.X != 5 {
				t.Fatalf("at rest before the start: %+v", x.m)
			}
		}
		if x.t >= start && x.t < start+15 {
			first = append(first, x)
		}
		if x.t > end-15 && x.t <= end {
			last = append(last, x)
		}
	}
	if first[0].m.Speed > 0.2*cruise || first[1].m.Speed > 0.4*cruise {
		t.Fatalf("start eases in: %.4f %.4f", first[0].m.Speed, first[1].m.Speed)
	}
	// exact sim timing means the eased step averages cruise speed, so the
	// ease rises (or falls) monotonically over most of the step and peaks
	// at most a third above cruise
	for i := 1; i < len(first)*3/5; i++ {
		if first[i].m.Speed < first[i-1].m.Speed-1e-9 {
			t.Fatal("speed rises through the first step")
		}
	}
	for i := len(last) * 2 / 5; i < len(last); i++ {
		if i > 0 && last[i].m.Speed > last[i-1].m.Speed+1e-9 {
			t.Fatal("speed falls through the last step")
		}
	}
	for _, x := range s {
		if x.m.Speed > 1.35*cruise {
			t.Fatalf("ease overshoots: %.4f", x.m.Speed)
		}
	}
	if sp := last[len(last)-1].m.Speed; sp > 0.2*cruise {
		t.Fatalf("arrives slowly, speed %.4f", sp)
	}
	fin := s[len(s)-1].m
	if fin.X != 10 || fin.Y != 5 || fin.Moving || fin.Speed != 0 || fin.Blend != 0 {
		t.Fatalf("rests on the goal: %+v", fin)
	}
	// blend reaches 1 after 0.3 tiles and is partial before
	for _, x := range s {
		walked := x.m.X - 5
		if walked > 0.31 && 10-x.m.X > 0.31 && x.m.Blend < 0.999 {
			t.Fatalf("full walk blend mid-run: %.3f at x=%.3f", x.m.Blend, x.m.X)
		}
		if walked > 1e-6 && walked < 0.15 && (x.m.Blend <= 0 || x.m.Blend >= 1) {
			t.Fatalf("partial blend early: %.3f at x=%.3f", x.m.Blend, x.m.X)
		}
	}
	if math.Abs(fin.Stride-5) > 1e-6 {
		t.Fatalf("stride is the path length: %.4f", fin.Stride)
	}
}

func TestBlockedEasesToStop(t *testing.T) {
	t.Parallel()
	for _, tpf := range []int{1, 3, 6} {
		mv := &scriptMover{id: 1, path: line(5, 5, 1, 0, 8), start: 10, blockAt: 4}
		r := New(newMotionSim(mv))
		s := drive(r, 1, tpf, 200/tpf, []float64{0, 0.5})
		nominal := float64(tpf) / 2 / 15
		for i := 1; i < len(s); i++ {
			if d := dist(s[i-1].m, s[i].m); d > 1.5*nominal+1e-9 {
				t.Fatalf("%dx: jump %.4f > 1.5x %.4f at t=%.2f", tpf, d, nominal, s[i].t)
			}
			if s[i].m.X > 9+0.05 {
				t.Fatalf("%dx: overshoots the blocked cell: x=%.4f", tpf, s[i].m.X)
			}
		}
		fin := s[len(s)-1].m
		if fin.X != 9 || fin.Y != 5 || fin.Moving || fin.Speed != 0 || fin.Blend != 0 {
			t.Fatalf("%dx: stops on its cell: %+v", tpf, fin)
		}
		if fin.Facing != faceEast {
			t.Fatalf("%dx: faces the blocker, got %v", tpf, fin.Facing)
		}
	}
}

func TestFacingHysteresisZigZag(t *testing.T) {
	t.Parallel()
	path := []cell{{5, 5}}
	for i := 0; i < 8; i++ {
		c := path[len(path)-1]
		path = append(path, cell{c.x + 1, c.y})
		path = append(path, cell{c.x + 1, c.y + 1})
	}
	path = append(path, cell{path[len(path)-1].x + 1, path[len(path)-1].y})
	for _, tpf := range []int{1, 3} {
		mv := &scriptMover{id: 1, path: path, start: 10}
		r := New(newMotionSim(mv))
		for _, x := range drive(r, 1, tpf, 400/tpf, []float64{0}) {
			if x.t > float64(mv.start) && x.m.Facing != faceEast {
				t.Fatalf("%dx: zig-zag flickers to %v at t=%.1f", tpf, x.m.Facing, x.t)
			}
		}
	}
	// sanity: the tile steps alone would flicker
	if facingFor(0, 1, faceEast) == faceEast {
		t.Fatal("per-step facing would not flicker; test is moot")
	}
}

func TestNorthSouthTurnViaSide(t *testing.T) {
	t.Parallel()
	path := []cell{{6, 9}, {5, 9}, {5, 8}, {5, 7}, {5, 6}, {5, 7}, {5, 8}, {5, 9}}
	mv := &scriptMover{id: 1, path: path, start: 10}
	r := New(newMotionSim(mv))
	var seq []facing
	for _, x := range drive(r, 1, 1, 160, []float64{0}) {
		if x.t > float64(mv.start) {
			seq = append(seq, x.m.Facing)
		}
	}
	i0 := -1
	for i, f := range seq {
		if f == faceNorth {
			i0 = i
			break
		}
	}
	if i0 < 0 {
		t.Fatalf("never faced north: %v", seq)
	}
	sawN, sawS, side := false, false, 0
	for i := i0; i < len(seq); i++ {
		switch f := seq[i]; f {
		case faceNorth:
			sawN = true
			if sawS || side > 0 {
				t.Fatal("back to north after the turn")
			}
		case faceSouth:
			sawS = true
			if seq[i-1] == faceNorth {
				t.Fatal("north flipped straight to south")
			}
		default:
			if sawS {
				t.Fatalf("side facing after the turn: %v", seq)
			}
			side++
		}
	}
	if !sawN || !sawS || side != 4 {
		t.Fatalf("north, 4 side frames, south: n=%v s=%v side=%d %v", sawN, sawS, side, seq)
	}
}

func TestPausedFreezes(t *testing.T) {
	t.Parallel()
	mv := &scriptMover{id: 1, path: line(5, 5, 1, 0, 10), start: 10}
	sim := newMotionSim(mv)
	r := New(sim)
	drive(r, 1, 3, 15, []float64{0})
	r.setAlpha(0.999)
	before := r.pawnMotion(1)
	tick := sim.tick
	paused := drive(r, 1, 0, 20, []float64{0, 0.3, 0.7, 0.99})
	if sim.tick != tick {
		t.Fatal("paused must not step")
	}
	ref := paused[0]
	if dist(before, ref.m) > 0.01 {
		t.Fatalf("pausing jumps %.4f", dist(before, ref.m))
	}
	for _, x := range paused {
		if x.t != ref.t || x.m != ref.m {
			t.Fatalf("paused motion changed: %+v vs %+v", x.m, ref.m)
		}
	}
	if ref.t != float64(tick) {
		t.Fatalf("paused animT = %v, want the tick %v", ref.t, tick)
	}
	// resuming continues from the frozen point
	r.speed = speedNormal
	r.advance()
	r.setAlpha(0)
	if d := dist(r.pawnMotion(1), ref.m); d > 1e-9 {
		t.Fatalf("resume jumps %.4f", d)
	}
}

func TestCaptureAlphaZero(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(100, 0)
	now := t0
	a := testApp(&stubSim{w: 8, h: 8})
	a.now = func() time.Time { return now }
	a.markUpdate()
	now = t0.Add(time.Second / 120)
	if got := a.frameAlpha(); math.Abs(got-0.5) > 0.01 {
		t.Fatalf("half an update interval: alpha %.3f", got)
	}
	now = t0.Add(time.Second)
	if got := a.frameAlpha(); got >= 1 || got < 0.99 {
		t.Fatalf("late draw clamps below 1: %v", got)
	}
	now = t0.Add(-time.Second)
	if got := a.frameAlpha(); got != 0 {
		t.Fatalf("clock skew clamps to 0: %v", got)
	}
	a.cfg.Capture = &Capture{}
	now = t0.Add(time.Second / 120)
	if got := a.frameAlpha(); got != 0 {
		t.Fatalf("capture alpha = %v, want 0", got)
	}

	// with alpha 0 two identical runs give identical motion
	run := func() []drawSample {
		r := New(newMotionSim(&scriptMover{id: 1, path: line(5, 5, 1, 1, 6), start: 4}))
		return drive(r, 1, 3, 40, []float64{0})
	}
	x, y := run(), run()
	for i := range x {
		if x[i] != y[i] {
			t.Fatalf("capture motion not deterministic at %d", i)
		}
	}
}

func TestCaptureSpeed(t *testing.T) {
	t.Parallel()
	cases := map[int]speed{0: speedNormal, 1: speedNormal, 2: speedFast, 3: speedVeryFast, 9: speedNormal}
	for in, want := range cases {
		if got := (&Capture{Speed: in}).speed(); got != want {
			t.Fatalf("Capture.Speed %d -> %v, want %v", in, got, want)
		}
	}
}

func TestAnimalPaceChangeNoJump(t *testing.T) {
	t.Parallel()
	for _, tpf := range []int{1, 3, 6} {
		mv := &scriptMover{id: 7, path: line(5, 5, 1, 0, 14), start: 10, animal: true,
			pace: func(k int) int {
				if k <= 5 {
					return 18 // wandering
				}
				return 10 // fleeing
			}}
		r := New(newMotionSim(mv))
		s := drive(r, 7, tpf, 300/tpf, []float64{0, 0.5})
		nominal := float64(tpf) / 2 / 10 // per draw at the fast pace
		for i := 1; i < len(s); i++ {
			d := s[i].m.X - s[i-1].m.X
			if d < -1e-9 || d > 1.5*nominal {
				t.Fatalf("%dx: step %.4f (fast nominal %.4f) at t=%.2f", tpf, d, nominal, s[i].t)
			}
		}
		if wx, _ := r.interpAnimal(r.game.World().Animals[0]); wx != s[len(s)-1].m.X {
			t.Fatalf("%dx: interpAnimal follows animalMotion", tpf)
		}
	}
}

func TestInterpWorldFollowsMotion(t *testing.T) {
	t.Parallel()
	mv := &scriptMover{id: 1, path: line(5, 5, 1, 0, 4), start: 2}
	sim := newMotionSim(mv)
	r := New(sim)
	unknown := game.PawnView{ID: 42, X: 3, Y: 4}
	if x, y := r.interpWorld(unknown); x != 3 || y != 4 {
		t.Fatal("unseen pawn sits on its tile")
	}
	drive(r, 1, 1, 10, []float64{0.5})
	p := sim.Snapshot()[0]
	m := r.pawnMotion(1)
	if x, y := r.interpWorld(p); x != m.X || y != m.Y || x <= 5 || x >= 6 {
		t.Fatalf("interpWorld = %v,%v motion %v,%v", x, y, m.X, m.Y)
	}
	r.settleVisuals()
	if x, _ := r.interpWorld(p); x != float64(p.X) {
		t.Fatalf("settled pawn is drawn on its tile, got %v", x)
	}
	if (&Renderer{}).animT() != 0 {
		t.Fatal("renderer without a sim has animT 0")
	}
}

func TestMotionFallbackWithoutStepTiming(t *testing.T) {
	t.Parallel()
	// a sim that predates C1: only X/Y change, every TicksPerCell ticks
	mv := &scriptMover{id: 1, path: line(5, 5, 1, 0, 6), start: 30}
	sim := &legacySim{motionSim: newMotionSim(mv)}
	r := New(sim)
	s := drive(r, 1, 1, 160, []float64{0, 0.5})
	for i := 1; i < len(s); i++ {
		if d := dist(s[i-1].m, s[i].m); d > 1.5*0.5/15+1e-9 {
			t.Fatalf("legacy jump %.4f at t=%.2f", d, s[i].t)
		}
	}
	if fin := s[len(s)-1].m; fin.X != 11 || fin.Moving {
		t.Fatalf("legacy pawn arrives: %+v", fin)
	}
}

type legacySim struct{ *motionSim }

func (s *legacySim) Snapshot() []game.PawnView {
	ps := s.motionSim.Snapshot()
	for i := range ps {
		ps[i].StepTick, ps[i].StepTicks, ps[i].HasNext, ps[i].NextX, ps[i].NextY = 0, 0, false, 0, 0
	}
	return ps
}

func TestTracerMinVisibleFrames(t *testing.T) {
	t.Parallel()
	shot := game.TraceView{FromX: 2, FromY: 5, ToX: 6, ToY: 5, Hit: true}
	// dropped by a fast sim 3 frames ago: still flying
	if a := traceAge(reactTrace{TraceView: shot, sim: math.Inf(1), frames: 3}, tracerTicks); a >= tracerTicks {
		t.Fatalf("tracer gone after 3 frames: age %.2f", a)
	}
	if a := traceAge(reactTrace{TraceView: shot, sim: math.Inf(1), frames: 8}, tracerTicks); a < tracerTicks {
		t.Fatalf("tracer still shown after 8 frames: age %.2f", a)
	}
	// a slow sim keeps its own, longer timing
	if a := traceAge(reactTrace{TraceView: shot, sim: 4, frames: 40}, tracerTicks); a != 4 {
		t.Fatalf("sim age governs when slower: %.2f", a)
	}
	// untracked traces (captures) are drawn by sim age, oldest first, in a
	// stable order
	r := New(newMotionSim())
	w := game.WorldView{Traces: []game.TraceView{{FromX: 9, Age: 2}, {FromX: 1, Age: 2}, {FromX: 1, Age: 1}}}
	got := r.traceList(w)
	if len(got) != 3 || !math.IsInf(got[0].frames, 1) || got[0].sim != 2 || got[0].FromX != 1 || got[1].FromX != 9 || got[2].sim != 1 {
		t.Fatalf("trace list %+v", got)
	}
}

func TestTracerFromInterpolatedShooter(t *testing.T) {
	t.Parallel()
	mv := &scriptMover{id: 1, path: line(5, 5, 1, 0, 4), start: 2}
	r := New(newMotionSim(mv))
	drive(r, 1, 1, 10, []float64{0.5})
	m := r.pawnMotion(1)
	w := r.game.World()
	from, to := traceEnds(r.traceSpots(w), game.TraceView{FromX: 6, FromY: 5, ToX: 9, ToY: 9})
	if from.x != m.X+0.5 || from.y != m.Y+0.3 {
		t.Fatalf("tracer leaves the drawn shooter: %+v vs %v,%v", from, m.X, m.Y)
	}
	if to.x != 9.5 || to.y != 9.4 {
		t.Fatalf("empty target cell: %+v", to)
	}
}

// TestRealSimStraightWalkConstantSpeed drives the real application game
// the way a capture does (alpha 0). The real sim stamps StepTick before the
// clock advances, so a step is first seen one tick later than in the fake.
func TestRealSimStraightWalkConstantSpeed(t *testing.T) {
	t.Parallel()
	for _, tpf := range []int{1, 3, 6} {
		// a walled 1-wide corridor (as -scenario corridor) so the path is straight
		g := game.NewColony(game.Config{Seed: 1, Width: 48, Height: 40})
		cx, cy := 24, 9
		g.ClearPlants(cx-10, cy-3, cx+10, cy+3)
		for x := cx - 6; x <= cx+6; x++ {
			g.BuildNow("wall", "wood", "south", x, cy-1)
			g.BuildNow("wall", "wood", "south", x, cy+1)
		}
		p := g.Snapshot()[0]
		g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: cx - 8, Y: cy})
		g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: cx + 8, Y: cy, Queue: true})
		p.X, p.Y = cx-8, cy
		r := New(g)
		type frame struct {
			m    moveState
			view game.PawnView
		}
		var fs []frame
		for i := 0; i < 900/tpf; i++ {
			r.speed = speedFor(tpf)
			r.advance()
			r.setAlpha(0)
			for _, q := range g.Snapshot() {
				if q.ID == p.ID {
					fs = append(fs, frame{r.pawnMotion(p.ID), q})
				}
			}
		}
		// cruise: two tiles after the start until two tiles before the goal,
		// on the row, unblocked
		var dx, ds []float64
		for i := 1; i < len(fs); i++ {
			a, b := fs[i-1], fs[i]
			if a.view.Y != cy || a.m.X < float64(cx-5) || b.m.X > float64(cx+5) || b.m.X < a.m.X {
				continue
			}
			if b.view.Y != p.Y || b.view.BlockedBy != 0 {
				t.Fatalf("%dx: walk left the row or was blocked: %+v", tpf, b.view)
			}
			dx = append(dx, b.m.X-a.m.X)
			ds = append(ds, b.m.Stride-a.m.Stride)
			if b.m.Blend < 1 || !b.m.Moving {
				t.Fatalf("%dx: walk blend dips mid-walk: blend %.3f at x=%.3f", tpf, b.m.Blend, b.m.X)
			}
		}
		if len(dx) < 8/tpf+4 {
			t.Fatalf("%dx: only %d cruise frames", tpf, len(dx))
		}
		for name, xs := range map[string][]float64{"position": dx, "stride": ds} {
			mean, cv := meanCV(xs)
			if cv > 0.05 {
				t.Fatalf("%dx: %s per frame CV %.4f > 5%%", tpf, name, cv)
			}
			for _, d := range xs {
				if d > 1.5*mean {
					t.Fatalf("%dx: %s jump %.4f > 1.5x mean %.4f", tpf, name, d, mean)
				}
			}
			t.Logf("%dx: %s %.5f tiles/frame, CV %.5f", tpf, name, mean, cv)
		}
	}
}
