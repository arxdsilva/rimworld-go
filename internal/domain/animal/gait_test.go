package animal

import (
	"testing"

	"rimworld-go/internal/domain/pawn/body"
)

// PLAY-022 C4: an animal moves at one of three gaits. Amble is wandering,
// grazing and milling; walk is an errand; run is fleeing, hunting,
// charging and manhunting.

func TestGaitOfMode(t *testing.T) {
	t.Parallel()
	for m, want := range map[Mode]Gait{
		Wander: GaitAmble, Confused: GaitAmble,
		Feed: GaitWalk, Rest: GaitWalk, Leave: GaitWalk,
		Flee: GaitRun, Attack: GaitRun, Hunt: GaitRun,
	} {
		if got := (State{Species: Deer, Mode: m}).GaitAt(0); got != want {
			t.Errorf("mode %v: gait %v, want %v", m, got, want)
		}
	}
}

func TestGaitAtErrandsAndManhunters(t *testing.T) {
	t.Parallel()
	s := State{Species: Deer}
	if s.GaitAt(10) != GaitAmble {
		t.Fatal("a wandering deer ambles")
	}
	s.Errand = true
	if s.GaitAt(10) != GaitWalk {
		t.Fatal("being led, following its master or going to a mate walks")
	}
	s = State{Species: Deer, Leaving: true}
	if s.GaitAt(10) != GaitWalk {
		t.Fatal("an animal walking off the map walks, whatever its mode")
	}
	s = State{Species: Deer, Manhunter: 100}
	if s.GaitAt(99) != GaitRun || s.GaitAt(100) != GaitAmble {
		t.Fatal("a manhunter runs until its rage ends")
	}
	// a flee or a fight on the way off the map runs
	s = State{Species: Deer, Leaving: true, Mode: Flee}
	if s.GaitAt(0) != GaitRun {
		t.Fatal("a fleeing animal runs even while leaving")
	}
}

func TestGaitNames(t *testing.T) {
	t.Parallel()
	for g, want := range map[Gait]string{GaitNone: "", GaitAmble: "amble", GaitWalk: "walk", GaitRun: "run", Gait(99): ""} {
		if g.String() != want {
			t.Errorf("gait %d = %q, want %q", g, g.String(), want)
		}
	}
}

// Amble < walk < run in speed for every species: more ticks per tile
// ambling than walking, and fewer running away than walking.
func TestAmbleSlowerThanWalkSlowerThanRun(t *testing.T) {
	t.Parallel()
	for _, s := range All() {
		amble := State{Species: s}.StepAt(GaitAmble)
		walk := State{Species: s}.StepAt(GaitWalk)
		run := State{Species: s, Mode: Flee}.StepAt(GaitRun)
		if amble <= walk || walk <= run {
			t.Errorf("%s: amble %d, walk %d, run %d ticks a tile; want amble > walk > run", s, amble, walk, run)
		}
		if walk != DefOf(s).StepTicks || run != DefOf(s).FleeStep {
			t.Errorf("%s: walk %d run %d, want today's step %d and flee step %d", s, walk, run, DefOf(s).StepTicks, DefOf(s).FleeStep)
		}
		if ratio := float64(amble) / float64(walk); ratio < 1.8 || ratio > 2.2 {
			t.Errorf("%s: amble is %.2fx the walk step, want about 2x", s, ratio)
		}
	}
}

// The balance guard: hunting, charging and attacking keep today's step
// (they only report run); fleeing alone uses the flee step.
func TestOnlyFleeingUsesTheFleeStep(t *testing.T) {
	t.Parallel()
	for _, s := range All() {
		d := DefOf(s)
		for _, m := range []Mode{Attack, Hunt, Wander, Feed, Rest, Leave, Confused} {
			if got := (State{Species: s, Mode: m}).StepAt(GaitRun); got != d.StepTicks {
				t.Errorf("%s in mode %v runs at %d ticks, want today's step %d", s, m, got, d.StepTicks)
			}
		}
		if got := (State{Species: s, Mode: Flee}).StepAt(GaitRun); got != d.FleeStep {
			t.Errorf("%s flees at %d ticks, want the flee step %d", s, got, d.FleeStep)
		}
		// the mode-only pace is what every earlier caller read
		for _, m := range []Mode{Wander, Attack, Flee} {
			st := State{Species: s, Mode: m}
			want := d.StepTicks
			if m == Flee {
				want = d.FleeStep
			}
			if st.Step() != want {
				t.Errorf("%s mode %v: Step %d, want %d", s, m, st.Step(), want)
			}
		}
	}
}

// A slow species (the tortoise) is clearly slower than a deer at every
// gait; a fast one (the gazelle) keeps its pace.
func TestTortoiseSlowerThanDeerAtEveryGait(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		gait Gait
		mode Mode
	}{{GaitAmble, Wander}, {GaitWalk, Feed}, {GaitRun, Flee}} {
		tort := State{Species: Tortoise, Mode: c.mode}.StepAt(c.gait)
		deer := State{Species: Deer, Mode: c.mode}.StepAt(c.gait)
		if float64(tort) < 1.5*float64(deer) {
			t.Errorf("%v: tortoise %d ticks a tile, deer %d; want the tortoise at least 1.5x slower", c.gait, tort, deer)
		}
	}
	if g, d := DefOf(Gazelle).StepTicks, DefOf(Deer).StepTicks; g >= d {
		t.Errorf("a gazelle (%d) is quicker than a deer (%d)", g, d)
	}
}

// The slow class is one notch of its own for every size: slower than the
// same size at a normal pace by slowStep ticks.
func TestSlowClassAddsSlowStep(t *testing.T) {
	t.Parallel()
	if slowStep < 12 {
		t.Fatalf("slowStep %d: the slow class must be clearly slow (was 6)", slowStep)
	}
	if fastStep != -4 {
		t.Fatalf("fastStep %d: the fast class keeps its pace", fastStep)
	}
	z := bySize[RosterOf(Tortoise).Size]
	if got := DefOf(Tortoise).StepTicks; got != z.step+slowStep {
		t.Fatalf("tortoise step %d, want %d + %d", got, z.step, slowStep)
	}
}

func TestStepCostAtScalesDiagonalsAndInjury(t *testing.T) {
	t.Parallel()
	s := State{Species: Deer}
	if got, want := s.StepCostAt(GaitAmble, false), 2*DefOf(Deer).StepTicks; got != want {
		t.Errorf("amble straight %d, want %d", got, want)
	}
	if got, want := s.StepCostAt(GaitAmble, true), (2*DefOf(Deer).StepTicks*1414+500)/1000; got != want {
		t.Errorf("amble diagonal %d, want %d", got, want)
	}
	if got := s.StepCostAt(GaitWalk, false); got != s.StepCost(false) {
		t.Errorf("walk straight %d, want today's %d", got, s.StepCost(false))
	}
	// a lame animal is slower at every gait
	lame := State{Species: Deer, Body: legsGone(body.QLegFL)}
	for _, g := range []Gait{GaitAmble, GaitWalk, GaitRun} {
		if lame.StepAt(g) <= s.StepAt(g) {
			t.Errorf("%v: a lame deer (%d) must be slower than a sound one (%d)", g, lame.StepAt(g), s.StepAt(g))
		}
	}
	// no gait: today's mode-only step
	if s.StepAt(GaitNone) != s.Step() {
		t.Errorf("no gait: %d, want the mode step %d", s.StepAt(GaitNone), s.Step())
	}
}
