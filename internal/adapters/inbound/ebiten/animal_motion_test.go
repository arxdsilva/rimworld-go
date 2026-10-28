package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-022 B: an animal's walk cycle keeps its feet planted, starts and
// stops on a neutral frame, grazes in seconds-long bouts and never flaps.

// gaitSim is a motionSim whose animals are one species with a pose class
// the test drives by tick (Action, Sleeping), a size Scale and a Gait.
type gaitSim struct {
	*motionSim
	species string
	scale   float64
	gait    string
	action  func(tick uint64) string
	sleep   func(tick uint64) bool
}

func (s *gaitSim) World() game.WorldView {
	w := s.motionSim.World()
	for i := range w.Animals {
		an := &w.Animals[i]
		an.Species, an.Scale, an.Gait = s.species, s.scale, s.gait
		if s.action != nil {
			an.Action = s.action(uint64(s.tick))
		}
		if s.sleep != nil {
			an.Sleeping = s.sleep(uint64(s.tick))
		}
	}
	return w
}

// gaitArt is a catalog with one species' standing facings carrying a gait
// (metadata only: nothing here draws).
func gaitArt(species string, gait float64) *artCatalog {
	c := &artCatalog{Sprites: map[string]*artSprite{}, Tinted: map[tintKey]*ebiten.Image{}}
	for _, f := range []string{"south", "east", "north"} {
		c.Sprites["animal/"+species+"_"+f] = &artSprite{Meta: artMeta{Canvas: [2]int{256, 192}, Origin: [2]float64{64, 128}, Gait: gait}}
	}
	return c
}

// gaitRenderer walks animal 7 along path (15-tick steps from tick 10) as
// species with the given gait.
func gaitRenderer(species string, gait float64, path []cell) (*Renderer, *gaitSim) {
	mv := &scriptMover{id: 7, path: path, start: 10, animal: true}
	sim := &gaitSim{motionSim: newMotionSim(mv), species: species}
	r := New(sim)
	r.art = gaitArt(species, gait)
	return r, sim
}

type gaitSample struct {
	t     float64
	m     moveState
	show  bodyShow
	frame string
}

// driveGait runs updates at tpf ticks each and samples animal 7 once per
// alpha after every update, like Draws between Updates.
func driveGait(r *Renderer, tpf, updates int, alphas []float64) []gaitSample {
	var out []gaitSample
	r.speed = speedFor(tpf)
	for i := 0; i < updates; i++ {
		r.advance()
		an := r.game.World().Animals[0]
		for _, a := range alphas {
			r.setAlpha(a)
			out = append(out, gaitSample{t: r.animT(), m: r.animalMotion(an.ID), show: r.animalShow(an), frame: r.animalFrame(an)})
		}
	}
	return out
}

// Planted feet (B.1): while the path goes on, the phase advances by the
// distance walked over the set's gait, so the body moves exactly one gait
// per cycle and a planted foot does not drift (rat, deer and elephant
// gaits as ART measured them). A juvenile's gait shrinks with its Scale.
func TestAnimalFeetStayPlanted(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		species string
		gait    float64
	}{{"rat", 0.234}, {"deer", 0.816}, {"elephant", 1.136}} {
		r, _ := gaitRenderer(c.species, c.gait, line(5, 5, 1, 0, 30))
		s := driveGait(r, 1, 300, []float64{0, 0.5})
		cycles, steady := 0.0, 0
		var dist float64
		for i := 1; i < len(s); i++ {
			a, b := s[i-1], s[i]
			if !a.show.walk || !b.show.walk || !math.IsInf(b.m.Left, 1) || a.m.Blend < 1 || b.m.Blend < 1 {
				continue // starting, steering onto the stop, or stopped
			}
			dS, dPh := b.m.Stride-a.m.Stride, b.show.ph-a.show.ph
			if drift := dS - c.gait*dPh; math.Abs(drift) > 1e-9 {
				t.Fatalf("%s: a planted foot drifts %.2g tiles at t=%.1f (dS %.4f, dph %.4f)", c.species, drift, b.t, dS, dPh)
			}
			cycles += dPh
			dist += dS
			steady++
		}
		if steady < 100 || cycles < 2 {
			t.Fatalf("%s: only %d steady samples, %.2f cycles", c.species, steady, cycles)
		}
		if got := dist / cycles; math.Abs(got-c.gait) > 1e-9 {
			t.Fatalf("%s: the body moves %.4f tiles per cycle, want its gait %.4f", c.species, got, c.gait)
		}
	}
	r := &Renderer{art: gaitArt("deer", 0.816)}
	if g := r.animalGait(game.AnimalView{Species: "deer", Scale: 0.6}); math.Abs(g-0.816*0.6) > 1e-12 {
		t.Fatalf("a juvenile's gait %v, want 0.6 x 0.816", g)
	}
	if g := r.animalGait(game.AnimalView{Species: "nosuch"}); g != gaitFallback {
		t.Fatalf("a set without gait walks %v, want the %v fallback", g, gaitFallback)
	}
}

// Smooth starts and stops (B.2): a walk begins on walk03 and comes to rest
// only from walk03 or walk07, at every speed and walk length, also when
// it is blocked; the phase never runs backwards or skips more than the
// 6x cap.
func TestAnimalWalkStartsAndStopsOnNeutralFrames(t *testing.T) {
	t.Parallel()
	for _, gait := range []float64{0.234, 0.816, 1.136} {
		for _, tpf := range []int{1, 3} {
			for n := 1; n <= 5; n++ {
				r, sim := gaitRenderer("deer", gait, line(5, 5, 1, 0, n+2))
				if n == 5 {
					sim.movers[0].blockAt = 4 // blocked mid-path
				}
				s := driveGait(r, tpf, (10+15*(n+3)+60)/tpf, []float64{0, 0.5})
				starts, stops := 0, 0
				for i := 1; i < len(s); i++ {
					a, b := s[i-1], s[i]
					switch {
					case !a.show.walk && b.show.walk:
						starts++
						if b.frame != "_walk03" {
							t.Fatalf("gait %v %dx n=%d: the walk starts on %s", gait, tpf, n, b.frame)
						}
					case a.show.walk && !b.show.walk:
						stops++
						if !neutralFrame(a.show.ph) {
							t.Fatalf("gait %v %dx n=%d: stopped from mid-stride %s (phase %.3f)", gait, tpf, n, a.frame, a.show.ph)
						}
					case a.show.walk && b.show.walk:
						if d := b.show.ph - a.show.ph; d < 0 || d > maxPhasePerDraw+1e-9 {
							t.Fatalf("gait %v %dx n=%d: the phase jumps %.3f", gait, tpf, n, d)
						}
					}
				}
				if starts != 1 || stops != 1 {
					t.Fatalf("gait %v %dx n=%d: %d starts, %d stops", gait, tpf, n, starts, stops)
				}
			}
		}
	}
}

// A walk whose end is known steers its phase so it arrives on a neutral
// frame as the body stops: the settle after the stop is short.
func TestAnimalStopLandsWithTheBody(t *testing.T) {
	t.Parallel()
	r, _ := gaitRenderer("deer", 0.816, line(5, 5, 1, 0, 4))
	s := driveGait(r, 1, 140, []float64{0})
	stopped := -1.0
	for i := 1; i < len(s); i++ {
		if s[i-1].m.Blend >= minBlend && s[i].m.Blend < minBlend {
			stopped = s[i].t
		}
		if stopped >= 0 && s[i-1].show.walk && !s[i].show.walk {
			if d := s[i].t - stopped; d > 2 {
				t.Fatalf("the legs finished %.0f ticks after the body stopped", d)
			}
			return
		}
	}
	t.Fatal("the walk never stopped")
}

// 6x speed (B.6): the phase advances at most maxPhasePerDraw per draw, so
// the frames never alias; at 1x the same walk is not capped.
func TestAnimalWalkCappedAtSixTimes(t *testing.T) {
	t.Parallel()
	r, _ := gaitRenderer("rat", 0.234, line(5, 5, 1, 0, 40))
	s := driveGait(r, 6, 80, []float64{0})
	capped := false
	for i := 1; i < len(s); i++ {
		if !s[i-1].show.walk || !s[i].show.walk {
			continue
		}
		d := s[i].show.ph - s[i-1].show.ph
		if d > maxPhasePerDraw+1e-9 {
			t.Fatalf("6x: the walk advanced %.3f cycles in one draw", d)
		}
		capped = capped || d > maxPhasePerDraw-1e-9
	}
	if !capped {
		t.Fatal("6x: a rat's walk never reached the cap; the test is moot")
	}
	if maxPhasePerDraw*trotFrames > 3 {
		t.Fatalf("the cap lets %v frames go by in one draw", maxPhasePerDraw*trotFrames)
	}
}

// One bob (B.3): a walking animal's pose has no renderer bob (the baked
// frames rise and nod); a run leans into its facing a little.
func TestAnimalWalkHasNoRendererBob(t *testing.T) {
	t.Parallel()
	r, sim := gaitRenderer("deer", 0.816, line(5, 5, 1, 0, 20))
	sim.gait = game.GaitRun
	leaned := false
	for i := 0; i < 150; i++ {
		r.advance()
		an := r.game.World().Animals[0]
		m := r.animalMotion(an.ID)
		ps := r.animalPose(an)
		if m.Blend >= minBlend && ps.Dy != 0 {
			t.Fatalf("a walking animal bobs %v tiles", ps.Dy)
		}
		if m.Blend >= minBlend && m.Facing == faceEast && ps.Rot > 0 && ps.Rot <= runLean {
			leaned = true
		}
	}
	if !leaned {
		t.Fatal("a run east never leans forward")
	}
}

// Breathing (B.4) moves the top of every body by at least breatheBase
// tiles (about a pixel at 64 px tiles), a rat's as well as an elephant's.
func TestBreathingShowsOnEverySize(t *testing.T) {
	t.Parallel()
	for _, h := range []float64{0.45, 1.0, 2.2, 4.0} {
		lo, hi := math.Inf(1), math.Inf(-1)
		for p := uint64(0); p < 400; p++ {
			k := breathe(h, p, 3)
			lo, hi = math.Min(lo, k), math.Max(hi, k)
		}
		if amp := (hi - lo) / 2 * h; amp < breatheBase-1e-9 || amp > 0.1 {
			t.Fatalf("a body %.2f tiles tall breathes %.3f tiles", h, amp)
		}
	}
}

// standThen is a deer standing still at (5, 5) whose Action and Sleeping
// follow the given functions of the tick.
func standThen(action func(uint64) string, sleep func(uint64) bool) (*Renderer, *gaitSim) {
	r, sim := gaitRenderer("deer", 0.816, []cell{{5, 5}})
	sim.action, sim.sleep = action, sleep
	return r, sim
}

// frameRun is one frame shown, from animT from, held ticks.
type frameRun struct {
	frame string
	from  float64
	held  float64
}

func frameRuns(s []gaitSample, frameOf func(gaitSample) string) []frameRun {
	var out []frameRun
	for i, x := range s {
		f := frameOf(x)
		if len(out) == 0 || out[len(out)-1].frame != f {
			if len(out) > 0 {
				out[len(out)-1].held = x.t - out[len(out)-1].from
			}
			out = append(out, frameRun{frame: f, from: x.t})
		}
		if i == len(s)-1 {
			out[len(out)-1].held = x.t - out[len(out)-1].from
		}
	}
	return out
}

func bodyFrame(x gaitSample) string {
	if x.show.rest {
		return "rest"
	}
	return x.frame
}

// Graze bouts (B.4, C5): an animal that stops on food stands a neutral
// hold, lowers its head through graze1, keeps it down (graze2) for 3–8 s,
// comes up through graze1 and looks about (standing) for 1–2 s, then
// grazes again; bout lengths differ per animal and per bout.
func TestGrazeBoutsLastSeconds(t *testing.T) {
	t.Parallel()
	r, sim := gaitRenderer("deer", 0.816, line(5, 5, 1, 0, 2))
	sim.action = func(tick uint64) string {
		if tick >= 30 { // reaches food on its last step
			return game.ActionGraze
		}
		return ""
	}
	all := frameRuns(driveGait(r, 1, 3000, []float64{0}), bodyFrame)
	first := -1
	for i, run := range all {
		if run.frame == "_graze1" {
			first = i
			break
		}
	}
	if first < 1 || all[first-1].frame != "" || all[first-1].held < bodyNeutralHold {
		t.Fatalf("the walk stops, then stands %d ticks before grazing: %+v", bodyNeutralHold, all)
	}
	runs := all[first-1:]
	if len(runs) < 8 || runs[2].frame != "_graze2" {
		t.Fatalf("stand, graze1, graze2...: %+v", runs)
	}
	downs := map[float64]bool{}
	for i, run := range runs[1 : len(runs)-1] {
		switch run.frame {
		case "_graze1":
			if run.held != bodyReachTicks {
				t.Fatalf("run %d: graze1 held %.0f ticks, want %d", i+1, run.held, bodyReachTicks)
			}
		case "_graze2":
			if run.held < grazeDownMin || run.held > grazeDownMin+grazeDownSpan {
				t.Fatalf("run %d: head down %.0f ticks, want 3-8 s", i+1, run.held)
			}
			downs[run.held] = true
		case "":
			if run.held < grazeLookMin || run.held > grazeLookMin+grazeLookSpan {
				t.Fatalf("run %d: looked up %.0f ticks, want 1-2 s", i+1, run.held)
			}
		default:
			t.Fatalf("run %d: %s while grazing", i+1, run.frame)
		}
	}
	if len(downs) < 2 {
		t.Fatalf("every bout is as long: %v", downs)
	}
}

// No flap (B.4): a view that flips between grazing and standing, or
// sleeping and awake, every few ticks never changes the frame faster than
// bodyMinHold, and lying down waits out the neutral hold.
func TestAnimalPosesDoNotFlap(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		action func(uint64) string
		sleep  func(uint64) bool
	}{
		{"graze", func(tick uint64) string {
			if tick/3%2 == 0 {
				return game.ActionGraze
			}
			return ""
		}, nil},
		{"sleep", nil, func(tick uint64) bool { return tick/5%2 == 0 }},
		{"both", func(tick uint64) string {
			if tick/4%3 == 0 {
				return game.ActionGraze
			}
			return ""
		}, func(tick uint64) bool { return tick/7%2 == 0 }},
	} {
		r, _ := standThen(c.action, c.sleep)
		runs := frameRuns(driveGait(r, 1, 600, []float64{0, 0.5}), bodyFrame)
		if len(runs) < 3 {
			t.Fatalf("%s: the pose never changed: %+v", c.name, runs)
		}
		for i, run := range runs[1 : len(runs)-1] {
			if run.held < bodyMinHold {
				t.Fatalf("%s: run %d (%s) held %.1f ticks, under the %d-tick hold", c.name, i+1, run.frame, run.held, bodyMinHold)
			}
			if run.frame == "rest" && runs[i].frame != "" {
				t.Fatalf("%s: lay down from %q, not from standing", c.name, runs[i].frame)
			}
		}
	}
}

// Lying down and getting up pass through standing (B.2): a grazer that
// falls asleep lifts its head (graze1), stands bodyNeutralHold, then lies
// down; woken to walk, it stands a moment before its walk.
func TestRestPassesThroughStanding(t *testing.T) {
	t.Parallel()
	mv := &scriptMover{id: 7, path: line(5, 5, 1, 0, 3), start: 300, animal: true}
	sim := &gaitSim{motionSim: newMotionSim(mv), species: "deer",
		action: func(tick uint64) string {
			if tick < 60 {
				return game.ActionGraze
			}
			return ""
		},
		sleep: func(tick uint64) bool { return tick >= 60 && tick < 290 }}
	r := New(sim)
	r.art = gaitArt("deer", 0.816)
	runs := frameRuns(driveGait(r, 1, 400, []float64{0}), bodyFrame)
	var seq []string
	for _, run := range runs {
		seq = append(seq, run.frame)
	}
	if len(runs) < 6 || runs[0].frame != "_graze2" || runs[1].frame != "_graze1" || runs[2].frame != "" ||
		runs[3].frame != "rest" || runs[4].frame != "" || !runs[5].isWalk() {
		t.Fatalf("graze2, graze1, stand, rest, stand, walk: %v", seq)
	}
	if runs[2].held < bodyNeutralHold {
		t.Fatalf("stood %.0f ticks before lying down, want a %d-tick hold", runs[2].held, bodyNeutralHold)
	}
	if runs[4].held < bodyReachTicks {
		t.Fatalf("stood %.0f ticks between lying and walking", runs[4].held)
	}
}

// isWalk reports a run of a walk frame.
func (f frameRun) isWalk() bool { return len(f.frame) == 7 && f.frame[:5] == "_walk" }

// Side facings on diagonals (motion.go sideBias): an animal setting off
// diagonally from facing south turns to its side; a pawn on the same path
// keeps its axis (ART §27.1 hysteresis).
func TestAnimalDiagonalFacesSideways(t *testing.T) {
	t.Parallel()
	path := []cell{{5, 5}}
	for i := 0; i < 8; i++ {
		c := path[len(path)-1]
		path = append(path, cell{c.x + 1, c.y + 1})
	}
	for _, animal := range []bool{true, false} {
		mv := &scriptMover{id: 1, path: path, start: 10, animal: animal}
		r := New(newMotionSim(mv))
		var mid facing
		for _, x := range drive(r, 1, 1, 70, []float64{0}) {
			if x.t > 40 && x.t < 60 {
				mid = x.m.Facing
			}
		}
		if animal && mid != faceEast {
			t.Fatalf("an animal on a diagonal faces %v, want its side (east)", mid)
		}
		if !animal && mid != faceSouth {
			t.Fatalf("a pawn on a diagonal faces %v, want its axis kept (south)", mid)
		}
	}
}

// The body player belongs to the motion track: a capture's settle starts
// it afresh, straight into the view's pose (a grazer shows a graze frame
// on its first draw, no hold).
func TestAnimalPoseStartsInItsClass(t *testing.T) {
	t.Parallel()
	r, _ := standThen(func(uint64) string { return game.ActionGraze }, nil)
	r.advance()
	an := r.game.World().Animals[0]
	if f := r.animalFrame(an); f != "_graze2" {
		t.Fatalf("a grazer first drawn shows %q, want its head down", f)
	}
	r.settleVisuals()
	if k := r.mo.Tracks[entity.EntityID(7)]; k == nil || k.Body.ok {
		t.Fatal("settle keeps the body player")
	}
	if f := r.animalFrame(an); f != "_graze2" {
		t.Fatalf("after a settle: %q", f)
	}
}
