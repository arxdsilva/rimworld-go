package sense

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeJoy is the game's side of TopOff and Relax: one drink on offer.
type fakeJoy struct {
	colonist, seat bool
	joy            int
	jobs           map[entity.EntityID]job.Record
	drink          item.Kind
	stack          entity.EntityID
	has            bool
	doseJoy        int
	held           entity.EntityID
	swallowed      int
	noTake         bool
	noSeat         bool
}

func newFakeJoy() *fakeJoy {
	return &fakeJoy{colonist: true, jobs: map[entity.EntityID]job.Record{}, doseJoy: 2000, drink: item.Beer, has: true, stack: 50}
}

func (f *fakeJoy) Colonist(entity.EntityID) bool           { return f.colonist }
func (f *fakeJoy) SeatWanted(entity.EntityID) bool         { return f.seat }
func (f *fakeJoy) Joy(entity.EntityID) int                 { return f.joy }
func (f *fakeJoy) SetJob(id entity.EntityID, j job.Record) { f.jobs[id] = j }
func (f *fakeJoy) StartJob(id entity.EntityID, r job.Record) bool {
	r.State = job.Goto
	f.jobs[id] = r
	return true
}
func (f *fakeJoy) Seat(_ entity.EntityID, x, y int) (int, int, bool) { return x + 1, y, !f.noSeat }
func (f *fakeJoy) Drink(entity.EntityID) (item.Kind, entity.EntityID, int, int, bool) {
	return f.drink, f.stack, 4, 4, f.has
}
func (f *fakeJoy) DoseJoy(item.Kind) int { return f.doseJoy }
func (f *fakeJoy) Unpack(entity.EntityID, item.Kind) (entity.EntityID, bool) {
	f.held = 60
	return 60, true
}
func (f *fakeJoy) TakeOne(entity.EntityID, entity.EntityID) (entity.EntityID, bool) {
	if f.noTake {
		return 0, false
	}
	f.held = 60
	return 60, true
}
func (f *fakeJoy) HeldJoy(_, stack entity.EntityID) (int, bool) { return f.doseJoy, stack == f.held }
func (f *fakeJoy) Swallow(_, _ entity.EntityID) bool {
	f.held = 0
	f.swallowed++
	return false
}

func joyGame() (*Sense, *fakeJoy) {
	f := newFakeJoy()
	return New(Roles{Joy: f}), f
}

func TestTopOffHysteresis(t *testing.T) {
	t.Parallel()
	s, _ := joyGame()
	steps := []struct {
		joy  int
		want bool
	}{
		{0, true},
		{topOffLow, true},
		{topOffHigh, true}, // at the mark: not yet
		{topOffHigh + 1, false},
		{topOffLow + 1, false}, // down the band, still topped off
		{topOffLow, false},     // at the mark: still
		{topOffLow - 1, true},
		{topOffHigh, true}, // up the band: still wants
	}
	for i, st := range steps {
		if got := s.TopOff.Wants(1, st.joy); got != st.want {
			t.Fatalf("step %d, joy %d: wants %v", i, st.joy, got)
		}
	}
}

func TestTopOffOnlyForColonistsWithTheRuleOn(t *testing.T) {
	t.Parallel()
	s, f := joyGame()
	s.Rules = s.Rules.With(gameplay.JoyTopOff, false)
	if !s.TopOff.Wants(1, recreation.Full) || len(s.TopOffOn) != 0 {
		t.Fatal("rule off: wants recreation and records nothing")
	}
	s.Rules = s.Rules.With(gameplay.JoyTopOff, true)
	f.colonist = false
	if !s.TopOff.Wants(1, recreation.Full) || len(s.TopOffOn) != 0 {
		t.Fatal("not a colonist: wants recreation and records nothing")
	}
}

// TestTopOffKeepsItsSeat: low comfort still takes a free hour first.
func TestTopOffKeepsItsSeat(t *testing.T) {
	t.Parallel()
	s, f := joyGame()
	f.seat = true
	if !s.TopOff.Wants(1, recreation.Full) {
		t.Fatal("a seat to find comes before work")
	}
	f.seat = false
	if s.TopOff.Wants(1, recreation.Full) {
		t.Fatal("no seat wanted: to work")
	}
}

// TestTopOffDoesNotFlap walks joy up and down inside the band: the answer turns
// once for each crossing of the far mark, and the map is written only then.
func TestTopOffDoesNotFlap(t *testing.T) {
	t.Parallel()
	s, _ := joyGame()
	turns := 0
	was := s.TopOff.Wants(1, recreation.Full) // topped off
	walk := func(from, to, step int) {
		for j := from; j != to; j += step {
			if w := s.TopOff.Wants(1, j); w != was {
				turns, was = turns+1, w
			}
		}
	}
	for i := 0; i < 20; i++ {
		walk(topOffHigh, topOffLow, -50) // drains, then rests in the band
		walk(topOffLow, topOffHigh, 50)
	}
	if turns != 0 {
		t.Fatalf("%d turns inside the band", turns)
	}
	walk(topOffLow, topOffLow-100, -50) // under: wants recreation
	for i := 0; i < 20; i++ {
		walk(topOffLow, topOffHigh, 50)
		walk(topOffHigh, topOffLow, -50)
	}
	if turns != 1 {
		t.Fatalf("%d turns, want 1", turns)
	}
}

func TestTopOffWantsAllocatesNothing(t *testing.T) {
	s, _ := joyGame()
	s.TopOff.Wants(1, recreation.Full)
	j := 0
	if n := testing.AllocsPerRun(1000, func() {
		j++
		s.TopOff.Wants(1, topOffLow+j%500)
	}); n != 0 {
		t.Fatalf("%v allocations per call", n)
	}
}

func TestRelaxAssign(t *testing.T) {
	t.Parallel()
	seat := job.Record{Kind: job.SocialRelax, TX: 9, TY: 9, DestX: 8, DestY: 9, WorkLeft: 600}
	s, f := joyGame()
	if !s.Relax.Assign(1, seat) {
		t.Fatal("a stack of drink on offer: starts with it")
	}
	if j := f.jobs[1]; j.Target != 50 || j.TX != 4 || j.TY != 4 || j.Count != 600-relaxSit || j.WorkLeft != 600 {
		t.Fatalf("job %+v: to the stack first", j)
	}
	s, f = joyGame()
	f.stack = 0
	if !s.Relax.Assign(1, seat) {
		t.Fatal("a drink in the pack: starts with it")
	}
	if j := f.jobs[1]; j.Carry != 60 || j.Target != 0 || j.TX != 9 {
		t.Fatalf("job %+v: straight to the seat, drink in hand", j)
	}
	for name, edit := range map[string]func(s *Sense, f *fakeJoy){
		"rule off":      func(s *Sense, _ *fakeJoy) { s.Rules = s.Rules.With(gameplay.RelaxDrink, false) },
		"nothing there": func(_ *Sense, f *fakeJoy) { f.has = false },
		"nearly full":   func(_ *Sense, f *fakeJoy) { f.joy = finishMark(f.doseJoy) },
	} {
		s, f := joyGame()
		edit(s, f)
		if s.Relax.Assign(1, seat) || len(f.jobs) != 0 {
			t.Errorf("%s: a bare relax", name)
		}
	}
}

// TestRelaxDo: a bare relax is untouched; the walk to the stack ends in a seat;
// the drink goes down at the mark, not before the sit is over, or at the end.
func TestRelaxDo(t *testing.T) {
	t.Parallel()
	s, f := joyGame()
	bare := job.Record{Kind: job.SocialRelax, WorkLeft: 100}
	if j, lingers := s.Relax.Do(1, bare); !lingers || j.WorkLeft != 100 {
		t.Fatal("a bare relax lingers")
	}
	s.Relax.Do(1, job.Record{Kind: job.SocialRelax, Target: 50, DestX: 8, DestY: 9, WorkLeft: 600, Count: 360})
	if j := f.jobs[1]; j.Carry != 60 || j.Target != 0 || j.TX != 9 || j.WorkLeft != 600 || j.Count != 360 {
		t.Fatalf("after the fetch: %+v", j)
	}
	j := f.jobs[1]
	j.WorkLeft = 500 // not sat long enough
	f.joy = recreation.Full
	if _, lingers := s.Relax.Do(1, j); !lingers || f.swallowed != 0 {
		t.Fatal("the sit comes first")
	}
	j.WorkLeft = 300
	f.joy = finishMark(f.doseJoy) - 1
	if _, lingers := s.Relax.Do(1, j); !lingers || f.swallowed != 0 {
		t.Fatal("under the mark: keeps it")
	}
	f.joy = finishMark(f.doseJoy)
	done, lingers := s.Relax.Do(1, j)
	if !lingers || f.swallowed != 1 || done.Carry != 0 || done.WorkLeft != 1 {
		t.Fatalf("at the mark: job %+v, swallowed %d", done, f.swallowed)
	}
	// Under the mark at the end of the job, the drink still goes down.
	s, f = joyGame()
	f.held = 60
	if done, _ := s.Relax.Do(1, job.Record{Kind: job.SocialRelax, Carry: 60, WorkLeft: 1, Count: 100}); f.swallowed != 1 || done.Carry != 0 {
		t.Fatalf("at the end: %+v, swallowed %d", done, f.swallowed)
	}
}

// TestRelaxDoLosesItsDrink: a stale record (restored after a failed work giver
// put the drink down) doesn't drink what it no longer holds.
func TestRelaxDoLosesItsDrink(t *testing.T) {
	t.Parallel()
	s, f := joyGame()
	j, lingers := s.Relax.Do(1, job.Record{Kind: job.SocialRelax, Carry: 60, WorkLeft: 1, Count: 100})
	if !lingers || j.Carry != 0 || f.swallowed != 0 {
		t.Fatalf("job %+v, swallowed %d", j, f.swallowed)
	}
}

func TestRelaxFetchFailures(t *testing.T) {
	t.Parallel()
	s, f := joyGame()
	f.noTake = true
	s.Relax.Do(1, job.Record{Kind: job.SocialRelax, Target: 50, WorkLeft: 600})
	if j := f.jobs[1]; j.Carry != 0 || j.Target != 0 || j.WorkLeft != 600 {
		t.Fatalf("stack gone: a bare relax, got %+v", j)
	}
	s, f = joyGame()
	f.noSeat = true
	s.Relax.Do(1, job.Record{Kind: job.SocialRelax, Target: 50, WorkLeft: 600})
	if j := f.jobs[1]; j.State != job.Complete || j.Carry != 60 {
		t.Fatalf("no seat: ends with the drink on the job, got %+v", j)
	}
}
