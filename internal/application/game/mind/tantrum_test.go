package mind

import (
	"testing"

	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeSmash is the things on the fake line (y 0) a tantrum may smash:
// id by x, with hit points; GoBeside moves the pawn next to x at once.
type fakeSmash struct {
	things map[int]entity.EntityID
	hp     map[entity.EntityID]int
	moveTo map[entity.EntityID]int // GoBeside: the pawn's new x
	blows  []entity.EntityID
}

func (f *fakeSmash) Breakable(x, y int) (entity.EntityID, bool) {
	id, ok := f.things[x]
	return id, ok && y == 0 && f.hp[id] > 0
}
func (f *fakeSmash) GoBeside(id entity.EntityID, x, _ int) bool {
	if f.moveTo == nil {
		f.moveTo = map[entity.EntityID]int{}
	}
	f.moveTo[id] = x - 1
	return true
}
func (f *fakeSmash) Smash(_, t entity.EntityID) bool {
	f.blows = append(f.blows, t)
	if f.hp[t]--; f.hp[t] <= 0 {
		for x, id := range f.things {
			if id == t {
				delete(f.things, x)
			}
		}
	}
	return true
}

// tantrumWorld is one colonist at x 0 in a tantrum, with a chair at 3
// (2 HP) and a table at 5 (3 HP).
func tantrumWorld() (*fakeHost, *Mind) {
	f := newFake(1)
	f.things = map[int]entity.EntityID{3: 100, 5: 200}
	f.hp = map[entity.EntityID]int{100: 2, 200: 3}
	m := New(f)
	m.Force(1, mentalbreak.Tantrum)
	return f, m
}

// step runs one tick; GoBeside's walk lands at once.
func tantrumStep(f *fakeHost, m *Mind) {
	m.Tick(1, f.jobs[1])
	if x, ok := f.moveTo[1]; ok {
		f.x[1] = x
		delete(f.moveTo, 1)
	}
	f.now++
}

// TestTantrumSmashesThings: a tantrum is a major break and breaks the
// things around the colonist, one after the other.
func TestTantrumSmashesThings(t *testing.T) {
	if mentalbreak.TierOf(mentalbreak.Tantrum) != mentalbreak.Major || mentalbreak.Tantrum.String() != "tantrum" {
		t.Fatal("the tantrum is the major break named tantrum")
	}
	f, m := tantrumWorld()
	if k, _, ok := f.Break(1); !ok || k != mentalbreak.Tantrum {
		t.Fatal("forced tantrum")
	}
	for i := 0; i < 400 && len(f.things) > 0; i++ {
		tantrumStep(f, m)
	}
	if len(f.things) != 0 || len(f.blows) != 5 {
		t.Fatalf("both things broken by their hit points: left %v, blows %v", f.things, f.blows)
	}
}

// TestTantrumDoesNotFlap: the colonist keeps punching one thing until it
// breaks (never switching while it stands); with nothing left it looks
// again only every RetargetTicks.
func TestTantrumDoesNotFlap(t *testing.T) {
	f, m := tantrumWorld()
	f.rolls = []uint32{0, 1, 0, 1, 1, 0} // alternate picks if it ever looked again
	switches := 0
	for i := 0; i < 400 && len(f.things) > 0; i++ {
		tantrumStep(f, m)
	}
	for i := 1; i < len(f.blows); i++ {
		if f.blows[i] != f.blows[i-1] {
			switches++
		}
	}
	if switches != 1 {
		t.Fatalf("the tantrum switched marks %d times: %v", switches, f.blows)
	}
	wanders := f.wandered
	for i := 0; i < int(RetargetTicks)*2; i++ {
		tantrumStep(f, m)
	}
	if f.wandered <= wanders || m.track[1].Target != 0 {
		t.Fatal("nothing left: it wanders")
	}
	m.Tick(1, f.jobs[1])
	if mt := m.track[1]; mt.Next <= f.now {
		t.Fatalf("an empty look waits RetargetTicks: next %d now %d", mt.Next, f.now)
	}
}

// TestTantrumNeedsViolence: a pacifist never draws a tantrum.
func TestTantrumNeedsViolence(t *testing.T) {
	f := newFake(1)
	m := New(f)
	f.peaceful[1] = true
	if m.allowed(1, mentalbreak.Tantrum) {
		t.Fatal("a pacifist throws no tantrum")
	}
	f.peaceful[1] = false
	if !m.allowed(1, mentalbreak.Tantrum) {
		t.Fatal("a tantrum is allowed")
	}
}
