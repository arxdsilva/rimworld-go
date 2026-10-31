package bodytemp

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is AnimalHost over one air temperature.
type fakeHost struct {
	air                  int
	settled, died, rolls int
	downed, stands       bool
}

func (h *fakeHost) AirAt(entity.EntityID) int { return h.air }

// step is one Step with the game's follow-up: downing and standing on a
// change, the death counted.
func step(c Animals, h *fakeHost, a *animal.State, now uint64) bool {
	switch c.Step(1, a, now) {
	case Died:
		h.died++
		return false
	case Changed:
		h.settled++
		switch {
		case a.DownsNow():
			a.Downed, h.downed = true, true
		case a.StandsUp():
			a.Downed, h.stands = false, true
		}
	}
	return true
}
func (h *fakeHost) Roll(n int) int {
	h.rolls++
	return h.rolls * 7919 % n
}

// run steps an animal of species s in air t for hours, on its 60-tick
// cadence; it reports the hours it lived (hours when it survived).
func run(h *fakeHost, l Ledger, a *animal.State, hours int) int {
	c := Animals{H: h, L: l}
	for tick := 0; tick < hours*2500; tick += animal.DecideEvery {
		if !step(c, h, a, uint64(tick)) {
			return tick / 2500
		}
	}
	return hours
}

// TestAnimalFreezesByTheWiki (TEMP-3): a cow (wiki −10 °C) 10 °C past its
// danger line dies in about 32 h, going down on the way through
// consciousness; it takes nothing in bearable air.
func TestAnimalFreezesByTheWiki(t *testing.T) {
	h, l := &fakeHost{air: -300}, Ledger{}
	a := &animal.State{Species: animal.Cow}
	lived := run(h, l, a, 40)
	if lived < 31 || lived > 33 || h.died != 1 || !h.downed || l[1] != (Sev{}) {
		t.Fatalf("lived %d h, died %d, downed %v, ledger %+v", lived, h.died, h.downed, l[1])
	}
	h2, l2 := &fakeHost{air: 0}, Ledger{}
	b := &animal.State{Species: animal.Cow}
	if run(h2, l2, b, 10); b.Body != nil || len(l2) != 0 || h2.settled != 0 {
		t.Error("0 C is within a cow's margin: nothing happens")
	}
}

// TestInsectSlowsAndMachineFeelsNothing: a hive insect takes hypothermic
// slowdown up to full severity and lives; a machine takes nothing.
func TestInsectSlowsAndMachineFeelsNothing(t *testing.T) {
	h, l := &fakeHost{air: -600}, Ledger{}
	bug := &animal.State{Species: animal.HiveBeetle}
	if lived := run(h, l, bug, 60); lived != 60 || h.died != 0 {
		t.Fatalf("an insect never dies of the cold (lived %d, died %d)", lived, h.died)
	}
	if body.ExposureSev(bug.Body, body.CondHypothermicSlowdown) != 1000 || body.ExposureSev(bug.Body, body.CondHypothermia) != 0 {
		t.Error("the insect's cold is the slowdown")
	}
	if bug.Step() <= animal.DefOf(animal.HiveBeetle).StepTicks {
		t.Error("slowed: its step takes longer")
	}
	mech := &animal.State{Species: animal.Harrow}
	if run(h, l, mech, 5); mech.Body != nil || len(l) != 1 {
		t.Error("a machine feels no air")
	}
}

// TestAnimalRecoversAndStands: back in bearable air a downed animal's
// exposure recovers and it stands once consciousness is past the band
// (no flapping on the way: one down, one stand).
func TestAnimalRecoversAndStands(t *testing.T) {
	h, l := &fakeHost{air: 100}, Ledger{1: {Hypo: exposure.Extreme + exposure.Full/100}}
	a := &animal.State{Species: animal.Cow, Body: animal.NewBody(animal.Cow)}
	body.SetExposure(a.Body, body.CondHypothermia, exposure.Permille(l[1].Hypo))
	a.Downed = a.DownsNow()
	if !a.Downed {
		t.Fatal("extreme hypothermia downs")
	}
	downs := 0
	c := Animals{H: h, L: l}
	for tick := 0; tick < 6*2500; tick += animal.DecideEvery {
		was := a.Downed
		step(c, h, a, uint64(tick))
		if a.Downed != was {
			downs++
		}
	}
	if downs != 1 || a.Downed || len(l) != 0 || a.Body.Conds != nil {
		t.Fatalf("changes %d, downed %v, ledger %v, conds %v", downs, a.Downed, l, a.Body.Conds)
	}
}

func TestMigratePermilleKeepsMeaning(t *testing.T) {
	l := Ledger{1: {Hypo: 600, Heat: 300}, 2: {Hypo: 1000, Heat: 0}, 3: {Hypo: 800}}
	l.MigratePermille()
	if l[1].Hypo != exposure.Extreme || l[1].Heat != exposure.FromPermille(310) || l[2].Hypo != exposure.Full ||
		l[3].Hypo != exposure.FromPermille(810) {
		t.Fatalf("%+v", l)
	}
	if _, _, hypo, heat := l.AnimalView(1, animal.Cow); hypo != 62 || heat != 31 {
		t.Errorf("view %d %d", hypo, heat)
	}
	if lo, hi, _, _ := l.AnimalView(9, animal.Alpaca); lo != -200 || hi != 450 {
		t.Errorf("alpaca range %d..%d", lo, hi)
	}
}

func TestSpotNearestReachable(t *testing.T) {
	// a 5x1 strip: rooms 0 (cells 0,1), wall, room 1 (cells 3,4)
	rooms := []int{0, 0, -2, 1, 1}
	warm := func(r int) bool { return r == 1 }
	all := func(int, int) bool { return true }
	if x, _, ok := Spot(rooms, 5, 0, 0, warm, all, all); !ok || x != 3 {
		t.Errorf("nearest warm cell %d %v", x, ok)
	}
	if x, _, ok := Spot(rooms, 5, 0, 0, warm, all, func(x, _ int) bool { return x == 4 }); !ok || x != 4 {
		t.Errorf("skips the unreachable: %d %v", x, ok)
	}
	if _, _, ok := Spot(rooms, 5, 0, 0, func(int) bool { return false }, all, all); ok {
		t.Error("none bearable")
	}
}

// TestAnimalFrostbite: a cow at 90 % hypothermia in −30 °C air takes
// frostbite on its extremities in the end (paws, ears, nose, tail), as
// hit points, with an infection roll; above 0 °C, or as an insect, never.
func TestAnimalFrostbite(t *testing.T) {
	try := func(s animal.Species, air int) *animal.State {
		h, l := &fakeHost{air: air}, Ledger{}
		a := &animal.State{Species: s}
		c := Animals{H: h, L: l}
		for i := 0; i < 3000 && (a.Body == nil || body.FrostbiteParts(a.Body) == 0); i++ {
			l[1] = Sev{Hypo: exposure.Full / 10 * 9}
			step(c, h, a, uint64(i*animal.DecideEvery))
		}
		return a
	}
	cow := try(animal.Cow, -300)
	if body.FrostbiteParts(cow.Body) == 0 || cow.Injury == 0 {
		t.Fatalf("no frostbite: %+v", cow.Body)
	}
	for _, w := range cow.Body.Wounds {
		if w.Kind == body.Frostbite && (w.InfectAt == 0 || w.Part == cow.Body.Root()) {
			t.Errorf("frostbite on %s, infect %d", cow.Body.PartLabel(w.Part), w.InfectAt)
		}
	}
	if warm := try(animal.Cow, 50); body.FrostbiteParts(warm.Body) != 0 {
		t.Error("frostbite above 0 C")
	}
	if bug := try(animal.HiveBeetle, -300); body.FrostbiteParts(bug.Body) != 0 {
		t.Error("insects never frostbite")
	}
}
